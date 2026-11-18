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

// SubjectService menangani logika bisnis Mata Pelajaran (Epic 3 F3.4).
type SubjectService struct {
	db       *gorm.DB
	subjects *repositories.SubjectRepository
	audit    AuditLogger
	now      func() time.Time
}

// NewSubjectService membuat SubjectService baru.
func NewSubjectService(db *gorm.DB, audit AuditLogger) *SubjectService {
	if audit == nil {
		audit = NewNoopAuditLogger()
	}
	return &SubjectService{
		db:       db,
		subjects: repositories.NewSubjectRepository(db),
		audit:    audit,
		now:      time.Now,
	}
}

// List mengambil daftar mata pelajaran dengan pencarian dan filter.
func (s *SubjectService) List(ctx context.Context, filter dto.SubjectListFilter) (*dto.SubjectListResponse, error) {
	page, limit, err := normalizePageLimit(filter.Page, filter.Limit)
	if err != nil {
		return nil, err
	}

	items, total, err := s.subjects.List(repositories.SubjectListParams{
		Page:     page,
		Limit:    limit,
		Q:        strings.TrimSpace(filter.Q),
		IsActive: filter.IsActive,
	})
	if err != nil {
		return nil, utils.ErrInternal
	}

	respItems := make([]dto.SubjectResponse, 0, len(items))
	for i := range items {
		respItems = append(respItems, toSubjectResponse(&items[i]))
	}

	return &dto.SubjectListResponse{
		Items: respItems,
		Meta:  buildPaginationMeta(page, limit, total),
	}, nil
}

// Get mengambil detail satu mata pelajaran.
func (s *SubjectService) Get(ctx context.Context, id int64) (*dto.SubjectResponse, error) {
	subject, err := s.subjects.FindByID(id)
	if err != nil {
		if errors.Is(err, repositories.ErrNotFound) {
			return nil, utils.ErrSubjectNotFound
		}
		return nil, utils.ErrInternal
	}
	resp := toSubjectResponse(subject)
	return &resp, nil
}

// Create membuat mata pelajaran baru.
func (s *SubjectService) Create(ctx context.Context, actorID int64, req dto.CreateSubjectRequest) (*dto.SubjectResponse, error) {
	code := strings.ToUpper(strings.TrimSpace(req.Code))
	if code == "" {
		return nil, validationError("code", utils.FieldRequired, "code wajib diisi")
	}
	if len([]rune(code)) > 20 {
		return nil, validationError("code", utils.FieldTooLong, "code maksimal 20 karakter")
	}

	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, validationError("name", utils.FieldRequired, "name wajib diisi")
	}
	if len([]rune(name)) > 100 {
		return nil, validationError("name", utils.FieldTooLong, "name maksimal 100 karakter")
	}

	exists, err := s.subjects.ExistsByCode(code, 0)
	if err != nil {
		return nil, utils.ErrInternal
	}
	if exists {
		return nil, utils.ErrSubjectAlreadyExists.WithField("code", "SUBJECT_CODE_TAKEN", "Kode mata pelajaran sudah terdaftar")
	}

	isActive := true
	if req.IsActive != nil {
		isActive = *req.IsActive
	}

	subject := &models.Subject{
		Code:     code,
		Name:     name,
		IsActive: isActive,
	}

	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		txSubjects := s.subjects.WithTx(tx)
		if err := txSubjects.Create(subject); err != nil {
			return err
		}
		snapshot, err := marshalAuditSnapshot(toSubjectResponse(subject))
		if err != nil {
			return utils.ErrInternal
		}
		return s.audit.LogTx(ctx, tx, AuditEvent{
			ActorID:    actorID,
			TargetType: TargetTypeSubject,
			TargetID:   subject.ID,
			Action:     ActionSubjectCreated,
			Snapshot:   snapshot,
		})
	})
	if err != nil {
		if appErr := asAppError(err); appErr != nil {
			return nil, appErr
		}
		if mapped := mapSubjectWriteError(err); mapped != nil {
			return nil, mapped
		}
		return nil, utils.ErrInternal
	}

	return s.Get(ctx, subject.ID)
}

// Update memperbarui mata pelajaran dengan optimistic locking (updated_at).
func (s *SubjectService) Update(ctx context.Context, actorID, id int64, req dto.UpdateSubjectRequest) (*dto.SubjectResponse, error) {
	if req.UpdatedAt == nil {
		return nil, validationError("updated_at", utils.FieldRequired, "updated_at wajib diisi")
	}

	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		txSubjects := s.subjects.WithTx(tx)

		subject, err := txSubjects.FindByIDForUpdate(id)
		if err != nil {
			if errors.Is(err, repositories.ErrNotFound) {
				return utils.ErrSubjectNotFound
			}
			return utils.ErrInternal
		}

		if !sameTimestamp(*req.UpdatedAt, subject.UpdatedAt) {
			return utils.ErrStaleData
		}

		before := toSubjectResponse(subject)
		updates := make(map[string]interface{})

		if req.Code != nil {
			code := strings.ToUpper(strings.TrimSpace(*req.Code))
			if code == "" {
				return validationError("code", utils.FieldRequired, "code wajib diisi")
			}
			if len([]rune(code)) > 20 {
				return validationError("code", utils.FieldTooLong, "code maksimal 20 karakter")
			}
			if code != subject.Code {
				exists, err := txSubjects.ExistsByCode(code, id)
				if err != nil {
					return utils.ErrInternal
				}
				if exists {
					return utils.ErrSubjectAlreadyExists.WithField("code", "SUBJECT_CODE_TAKEN", "Kode mata pelajaran sudah terdaftar")
				}
				updates["code"] = code
			}
		}

		if req.Name != nil {
			name := strings.TrimSpace(*req.Name)
			if name == "" {
				return validationError("name", utils.FieldRequired, "name wajib diisi")
			}
			if len([]rune(name)) > 100 {
				return validationError("name", utils.FieldTooLong, "name maksimal 100 karakter")
			}
			if name != subject.Name {
				updates["name"] = name
			}
		}

		if req.IsActive != nil && *req.IsActive != subject.IsActive {
			updates["is_active"] = *req.IsActive
		}

		if len(updates) == 0 {
			return nil
		}
		updates["updated_at"] = s.now()

		rows, err := txSubjects.Update(id, subject.UpdatedAt, updates)
		if err != nil {
			return err
		}
		if rows == 0 {
			return utils.ErrStaleData
		}

		updated, err := txSubjects.FindByID(id)
		if err != nil {
			return utils.ErrInternal
		}
		after := toSubjectResponse(updated)
		snapshot, err := marshalAuditSnapshot(map[string]interface{}{
			"before": before,
			"after":  after,
		})
		if err != nil {
			return utils.ErrInternal
		}
		return s.audit.LogTx(ctx, tx, AuditEvent{
			ActorID:    actorID,
			TargetType: TargetTypeSubject,
			TargetID:   id,
			Action:     ActionSubjectUpdated,
			Snapshot:   snapshot,
		})
	})
	if err != nil {
		if appErr := asAppError(err); appErr != nil {
			return nil, appErr
		}
		if mapped := mapSubjectWriteError(err); mapped != nil {
			return nil, mapped
		}
		return nil, utils.ErrInternal
	}

	return s.Get(ctx, id)
}

// Delete menghapus mata pelajaran. Jika masih dipakai guru/jadwal (ON DELETE
// RESTRICT), mengembalikan 409 SUBJECT_IN_USE dengan arahan menonaktifkan.
func (s *SubjectService) Delete(ctx context.Context, actorID, id int64) error {
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		txSubjects := s.subjects.WithTx(tx)

		subject, err := txSubjects.FindByIDForUpdate(id)
		if err != nil {
			if errors.Is(err, repositories.ErrNotFound) {
				return utils.ErrSubjectNotFound
			}
			return utils.ErrInternal
		}

		snapshot, err := marshalAuditSnapshot(toSubjectResponse(subject))
		if err != nil {
			return utils.ErrInternal
		}
		if err := s.audit.LogTx(ctx, tx, AuditEvent{
			ActorID:    actorID,
			TargetType: TargetTypeSubject,
			TargetID:   id,
			Action:     ActionSubjectDeleted,
			Snapshot:   snapshot,
		}); err != nil {
			return err
		}

		rows, err := txSubjects.Delete(id)
		if err != nil {
			return err
		}
		if rows == 0 {
			return utils.ErrSubjectNotFound
		}
		return nil
	})
	if err != nil {
		if appErr := asAppError(err); appErr != nil {
			return appErr
		}
		if isForeignKeyViolation(err) {
			return utils.ErrSubjectInUse
		}
		return utils.ErrInternal
	}
	return nil
}

func toSubjectResponse(subject *models.Subject) dto.SubjectResponse {
	return dto.SubjectResponse{
		ID:        subject.ID,
		Code:      subject.Code,
		Name:      subject.Name,
		IsActive:  subject.IsActive,
		UpdatedAt: subject.UpdatedAt,
	}
}

func mapSubjectWriteError(err error) *utils.AppError {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "23505" {
		return nil
	}
	if pgErr.ConstraintName == "uq_subjects_code" {
		return utils.ErrSubjectAlreadyExists.WithField("code", "SUBJECT_CODE_TAKEN", "Kode mata pelajaran sudah terdaftar")
	}
	return utils.ErrConflict
}
