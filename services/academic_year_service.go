package services

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"

	"e-absensi-backend/dto"
	"e-absensi-backend/models"
	"e-absensi-backend/repositories"
	"e-absensi-backend/utils"
)

const dateFormatYYYYMMDD = "2006-01-02"

// AcademicYearService menangani logika bisnis Tahun Ajaran (Epic 3 F3.5).
type AcademicYearService struct {
	db    *gorm.DB
	years *repositories.AcademicYearRepository
	audit AuditLogger
	now   func() time.Time
}

// NewAcademicYearService membuat AcademicYearService baru.
func NewAcademicYearService(db *gorm.DB, audit AuditLogger) *AcademicYearService {
	if audit == nil {
		audit = NewNoopAuditLogger()
	}
	return &AcademicYearService{
		db:    db,
		years: repositories.NewAcademicYearRepository(db),
		audit: audit,
		now:   time.Now,
	}
}

// List mengambil daftar tahun ajaran dengan pencarian dan filter.
func (s *AcademicYearService) List(ctx context.Context, filter dto.AcademicYearListFilter) (*dto.AcademicYearListResponse, error) {
	page, limit, err := normalizePageLimit(filter.Page, filter.Limit)
	if err != nil {
		return nil, err
	}

	semester := strings.ToLower(strings.TrimSpace(filter.ActiveSemester))
	if semester != "" && semester != models.Semester1 && semester != models.Semester2 {
		return nil, validationError("active_semester", utils.FieldInvalidSemester, "active_semester harus semester_1 atau semester_2")
	}

	items, total, err := s.years.List(repositories.AcademicYearListParams{
		Page:           page,
		Limit:          limit,
		Q:              strings.TrimSpace(filter.Q),
		IsActive:       filter.IsActive,
		ActiveSemester: semester,
	})
	if err != nil {
		return nil, utils.ErrInternal
	}

	respItems := make([]dto.AcademicYearResponse, 0, len(items))
	for i := range items {
		respItems = append(respItems, toAcademicYearResponse(&items[i]))
	}

	return &dto.AcademicYearListResponse{
		Items: respItems,
		Meta:  buildPaginationMeta(page, limit, total),
	}, nil
}

// Get mengambil detail satu tahun ajaran.
func (s *AcademicYearService) Get(ctx context.Context, id int64) (*dto.AcademicYearResponse, error) {
	year, err := s.years.FindByID(id)
	if err != nil {
		if errors.Is(err, repositories.ErrNotFound) {
			return nil, utils.ErrAcademicYearNotFound
		}
		return nil, utils.ErrInternal
	}
	resp := toAcademicYearResponse(year)
	return &resp, nil
}

// Create membuat tahun ajaran baru (khusus Admin). Jika is_active=true, tahun
// ajaran aktif sebelumnya dinonaktifkan dalam satu transaksi.
func (s *AcademicYearService) Create(ctx context.Context, actorID int64, req dto.CreateAcademicYearRequest) (*dto.AcademicYearResponse, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, validationError("name", utils.FieldRequired, "name wajib diisi")
	}
	if len([]rune(name)) > 20 {
		return nil, validationError("name", utils.FieldTooLong, "name maksimal 20 karakter")
	}

	semester := strings.ToLower(strings.TrimSpace(req.ActiveSemester))
	if semester != models.Semester1 && semester != models.Semester2 {
		return nil, validationError("active_semester", utils.FieldInvalidSemester, "active_semester harus semester_1 atau semester_2")
	}

	startDate, err := parseDateField("start_date", req.StartDate)
	if err != nil {
		return nil, err
	}

	var endDate *time.Time
	if req.EndDate != nil && strings.TrimSpace(*req.EndDate) != "" {
		parsedEnd, err := parseDateField("end_date", *req.EndDate)
		if err != nil {
			return nil, err
		}
		if parsedEnd.Before(startDate) {
			return nil, validationError("end_date", utils.FieldInvalidDateRange, "end_date tidak boleh sebelum start_date")
		}
		endDate = &parsedEnd
	}

	exists, err := s.years.ExistsByName(name, 0)
	if err != nil {
		return nil, utils.ErrInternal
	}
	if exists {
		return nil, utils.ErrAcademicYearAlreadyExists.WithField("name", "ACADEMIC_YEAR_TAKEN", "Nama tahun ajaran sudah terdaftar")
	}

	isActive := false
	if req.IsActive != nil {
		isActive = *req.IsActive
	}

	year := &models.AcademicYear{
		Name:           name,
		ActiveSemester: semester,
		StartDate:      startDate,
		EndDate:        endDate,
		IsActive:       isActive,
	}

	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		txYears := s.years.WithTx(tx)
		if isActive {
			if err := txYears.DeactivateAll(0); err != nil {
				return err
			}
		}
		if err := txYears.Create(year); err != nil {
			return err
		}
		snapshot, err := marshalAuditSnapshot(toAcademicYearResponse(year))
		if err != nil {
			return utils.ErrInternal
		}
		return s.audit.LogTx(ctx, tx, AuditEvent{
			ActorID:    actorID,
			TargetType: TargetTypeAcademicYear,
			TargetID:   year.ID,
			Action:     ActionAcademicYearCreated,
			Snapshot:   snapshot,
		})
	})
	if err != nil {
		if appErr := asAppError(err); appErr != nil {
			return nil, appErr
		}
		if mapped := mapAcademicYearWriteError(err); mapped != nil {
			return nil, mapped
		}
		return nil, utils.ErrInternal
	}

	return s.Get(ctx, year.ID)
}

// Update memperbarui tahun ajaran dengan optimistic locking (updated_at).
// Jika is_active diubah menjadi true, tahun ajaran lain dinonaktifkan dalam
// transaksi yang sama.
func (s *AcademicYearService) Update(ctx context.Context, actorID, id int64, req dto.UpdateAcademicYearRequest) (*dto.AcademicYearResponse, error) {
	if req.UpdatedAt == nil {
		return nil, validationError("updated_at", utils.FieldRequired, "updated_at wajib diisi")
	}

	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		txYears := s.years.WithTx(tx)

		year, err := txYears.FindByIDForUpdate(id)
		if err != nil {
			if errors.Is(err, repositories.ErrNotFound) {
				return utils.ErrAcademicYearNotFound
			}
			return utils.ErrInternal
		}

		if !sameTimestamp(*req.UpdatedAt, year.UpdatedAt) {
			return utils.ErrStaleData
		}

		before := toAcademicYearResponse(year)
		updates := make(map[string]interface{})

		if req.Name != nil {
			name := strings.TrimSpace(*req.Name)
			if name == "" {
				return validationError("name", utils.FieldRequired, "name wajib diisi")
			}
			if len([]rune(name)) > 20 {
				return validationError("name", utils.FieldTooLong, "name maksimal 20 karakter")
			}
			if name != year.Name {
				exists, err := txYears.ExistsByName(name, id)
				if err != nil {
					return utils.ErrInternal
				}
				if exists {
					return utils.ErrAcademicYearAlreadyExists.WithField("name", "ACADEMIC_YEAR_TAKEN", "Nama tahun ajaran sudah terdaftar")
				}
				updates["name"] = name
			}
		}

		if req.ActiveSemester != nil {
			semester := strings.ToLower(strings.TrimSpace(*req.ActiveSemester))
			if semester != models.Semester1 && semester != models.Semester2 {
				return validationError("active_semester", utils.FieldInvalidSemester, "active_semester harus semester_1 atau semester_2")
			}
			if semester != year.ActiveSemester {
				updates["active_semester"] = semester
			}
		}

		effectiveStart := year.StartDate
		if req.StartDate != nil {
			parsedStart, err := parseDateField("start_date", *req.StartDate)
			if err != nil {
				return err
			}
			effectiveStart = parsedStart
			if !sameDate(parsedStart, year.StartDate) {
				updates["start_date"] = parsedStart
			}
		}

		effectiveEnd := year.EndDate
		if req.ClearEndDate {
			effectiveEnd = nil
			if year.EndDate != nil {
				updates["end_date"] = nil
			}
		} else if req.EndDate != nil {
			trimmed := strings.TrimSpace(*req.EndDate)
			if trimmed == "" {
				effectiveEnd = nil
				if year.EndDate != nil {
					updates["end_date"] = nil
				}
			} else {
				parsedEnd, err := parseDateField("end_date", trimmed)
				if err != nil {
					return err
				}
				effectiveEnd = &parsedEnd
				if year.EndDate == nil || !sameDate(parsedEnd, *year.EndDate) {
					updates["end_date"] = parsedEnd
				}
			}
		}

		if effectiveEnd != nil && effectiveEnd.Before(effectiveStart) {
			return validationError("end_date", utils.FieldInvalidDateRange, "end_date tidak boleh sebelum start_date")
		}

		if req.IsActive != nil && *req.IsActive != year.IsActive {
			if *req.IsActive {
				if err := txYears.DeactivateAll(id); err != nil {
					return err
				}
			}
			updates["is_active"] = *req.IsActive
		}

		if len(updates) == 0 {
			return nil
		}
		updates["updated_at"] = s.now()

		rows, err := txYears.Update(id, year.UpdatedAt, updates)
		if err != nil {
			return err
		}
		if rows == 0 {
			return utils.ErrStaleData
		}

		updated, err := txYears.FindByID(id)
		if err != nil {
			return utils.ErrInternal
		}
		after := toAcademicYearResponse(updated)
		snapshot, err := marshalAuditSnapshot(map[string]interface{}{
			"before": before,
			"after":  after,
		})
		if err != nil {
			return utils.ErrInternal
		}
		return s.audit.LogTx(ctx, tx, AuditEvent{
			ActorID:    actorID,
			TargetType: TargetTypeAcademicYear,
			TargetID:   id,
			Action:     ActionAcademicYearUpdated,
			Snapshot:   snapshot,
		})
	})
	if err != nil {
		if appErr := asAppError(err); appErr != nil {
			return nil, appErr
		}
		if mapped := mapAcademicYearWriteError(err); mapped != nil {
			return nil, mapped
		}
		return nil, utils.ErrInternal
	}

	return s.Get(ctx, id)
}

// Delete menghapus tahun ajaran (khusus Admin). Jika sudah dipakai kelas,
// dikembalikan 409 ACADEMIC_YEAR_IN_USE.
func (s *AcademicYearService) Delete(ctx context.Context, actorID, id int64) error {
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		txYears := s.years.WithTx(tx)

		year, err := txYears.FindByIDForUpdate(id)
		if err != nil {
			if errors.Is(err, repositories.ErrNotFound) {
				return utils.ErrAcademicYearNotFound
			}
			return utils.ErrInternal
		}

		snapshot, err := marshalAuditSnapshot(toAcademicYearResponse(year))
		if err != nil {
			return utils.ErrInternal
		}
		if err := s.audit.LogTx(ctx, tx, AuditEvent{
			ActorID:    actorID,
			TargetType: TargetTypeAcademicYear,
			TargetID:   id,
			Action:     ActionAcademicYearDeleted,
			Snapshot:   snapshot,
		}); err != nil {
			return err
		}

		rows, err := txYears.Delete(id)
		if err != nil {
			return err
		}
		if rows == 0 {
			return utils.ErrAcademicYearNotFound
		}
		return nil
	})
	if err != nil {
		if appErr := asAppError(err); appErr != nil {
			return appErr
		}
		if isForeignKeyViolation(err) {
			return utils.ErrAcademicYearInUse
		}
		return utils.ErrInternal
	}
	return nil
}

func toAcademicYearResponse(year *models.AcademicYear) dto.AcademicYearResponse {
	var endStr *string
	if year.EndDate != nil {
		formatted := year.EndDate.Format(dateFormatYYYYMMDD)
		endStr = &formatted
	}
	return dto.AcademicYearResponse{
		ID:             year.ID,
		Name:           year.Name,
		ActiveSemester: year.ActiveSemester,
		StartDate:      year.StartDate.Format(dateFormatYYYYMMDD),
		EndDate:        endStr,
		IsActive:       year.IsActive,
		UpdatedAt:      year.UpdatedAt,
	}
}

func parseDateField(field, raw string) (time.Time, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return time.Time{}, validationError(field, utils.FieldRequired, field+" wajib diisi")
	}
	parsed, err := time.Parse(dateFormatYYYYMMDD, trimmed)
	if err != nil {
		return time.Time{}, validationError(field, utils.FieldInvalidDate, field+" harus berformat YYYY-MM-DD")
	}
	return parsed, nil
}

func sameDate(a, b time.Time) bool {
	return a.Format(dateFormatYYYYMMDD) == b.Format(dateFormatYYYYMMDD)
}

func normalizePageLimit(page, limit int) (int, int, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 20
	}
	if limit > 100 {
		return 0, 0, validationError("limit", utils.FieldInvalidLimit, "limit maksimal 100")
	}
	return page, limit, nil
}

func buildPaginationMeta(page, limit int, total int64) dto.PaginationResponse {
	totalPages := 0
	if total > 0 {
		totalPages = int((total + int64(limit) - 1) / int64(limit))
	}
	return dto.PaginationResponse{
		Page:       page,
		Limit:      limit,
		Total:      total,
		TotalPages: totalPages,
	}
}

func mapAcademicYearWriteError(err error) *utils.AppError {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "23505" {
		return nil
	}
	switch pgErr.ConstraintName {
	case "uq_academic_years_name":
		return utils.ErrAcademicYearAlreadyExists.WithField("name", "ACADEMIC_YEAR_TAKEN", "Nama tahun ajaran sudah terdaftar")
	case "uq_academic_years_active":
		return utils.ErrConflict.WithMessage("Hanya satu tahun ajaran yang boleh aktif pada satu waktu")
	default:
		return utils.ErrConflict
	}
}
