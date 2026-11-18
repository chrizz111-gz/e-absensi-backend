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

// ClassService menangani logika bisnis Kelas (Epic 3 F3.3).
type ClassService struct {
	db       *gorm.DB
	classes  *repositories.ClassRepository
	years    *repositories.AcademicYearRepository
	students *repositories.StudentRepository
	audit    AuditLogger
	now      func() time.Time
}

// NewClassService membuat ClassService baru.
func NewClassService(db *gorm.DB, audit AuditLogger) *ClassService {
	if audit == nil {
		audit = NewNoopAuditLogger()
	}
	return &ClassService{
		db:       db,
		classes:  repositories.NewClassRepository(db),
		years:    repositories.NewAcademicYearRepository(db),
		students: repositories.NewStudentRepository(db),
		audit:    audit,
		now:      time.Now,
	}
}

// List mengambil daftar kelas dengan pencarian dan filter.
func (s *ClassService) List(ctx context.Context, filter dto.ClassListFilter) (*dto.ClassListResponse, error) {
	page, limit, err := normalizePageLimit(filter.Page, filter.Limit)
	if err != nil {
		return nil, err
	}

	if filter.GradeLevel != nil && !isValidGradeLevel(*filter.GradeLevel) {
		return nil, validationError("grade_level", utils.FieldInvalidGradeLevel, "grade_level harus 10, 11, atau 12")
	}

	items, total, err := s.classes.List(repositories.ClassListParams{
		Page:           page,
		Limit:          limit,
		Q:              strings.TrimSpace(filter.Q),
		AcademicYearID: filter.AcademicYearID,
		GradeLevel:     filter.GradeLevel,
		IsActive:       filter.IsActive,
	})
	if err != nil {
		return nil, utils.ErrInternal
	}

	classIDs := make([]int64, 0, len(items))
	for i := range items {
		classIDs = append(classIDs, items[i].ID)
	}
	counts, err := s.classes.CountStudentsByClassIDs(classIDs)
	if err != nil {
		return nil, utils.ErrInternal
	}

	respItems := make([]dto.ClassResponse, 0, len(items))
	for i := range items {
		respItems = append(respItems, toClassResponse(&items[i], counts[items[i].ID], nil))
	}

	return &dto.ClassListResponse{
		Items: respItems,
		Meta:  buildPaginationMeta(page, limit, total),
	}, nil
}

// Get mengambil detail satu kelas. Jika diakses oleh Siswa/Siswa Inti untuk
// kelas selain kelasnya sendiri, daftar anggota siswa disembunyikan untuk
// mencegah kebocoran data (F3.3).
func (s *ClassService) Get(ctx context.Context, actorID int64, actorRoles []string, id int64) (*dto.ClassResponse, error) {
	class, err := s.classes.FindByID(id)
	if err != nil {
		if errors.Is(err, repositories.ErrNotFound) {
			return nil, utils.ErrClassNotFound
		}
		return nil, utils.ErrInternal
	}

	counts, err := s.classes.CountStudentsByClassIDs([]int64{class.ID})
	if err != nil {
		return nil, utils.ErrInternal
	}

	canSeeMembers := s.canViewClassRoster(actorID, actorRoles, class.ID)
	var members []dto.ClassStudentSummary
	if canSeeMembers {
		studentRows, err := s.students.ListByClassID(class.ID)
		if err != nil {
			return nil, utils.ErrInternal
		}
		members = make([]dto.ClassStudentSummary, 0, len(studentRows))
		for _, st := range studentRows {
			members = append(members, dto.ClassStudentSummary{
				ID:       st.ID,
				UserID:   st.UserID,
				Name:     st.User.Name,
				Email:    st.User.Email,
				NISN:     st.NISN,
				IsActive: st.User.IsActive,
			})
		}
	}

	resp := toClassResponse(class, counts[class.ID], members)
	return &resp, nil
}

// Create membuat kelas baru (Admin & Waka Kurikulum).
func (s *ClassService) Create(ctx context.Context, actorID int64, req dto.CreateClassRequest) (*dto.ClassResponse, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, validationError("name", utils.FieldRequired, "name wajib diisi")
	}
	if len([]rune(name)) > 50 {
		return nil, validationError("name", utils.FieldTooLong, "name maksimal 50 karakter")
	}

	if !isValidGradeLevel(req.GradeLevel) {
		return nil, validationError("grade_level", utils.FieldInvalidGradeLevel, "grade_level harus 10, 11, atau 12")
	}

	if _, err := s.years.FindByID(req.AcademicYearID); err != nil {
		if errors.Is(err, repositories.ErrNotFound) {
			return nil, utils.ErrAcademicYearNotFound
		}
		return nil, utils.ErrInternal
	}

	exists, err := s.classes.ExistsByNameAndYear(name, req.AcademicYearID, 0)
	if err != nil {
		return nil, utils.ErrInternal
	}
	if exists {
		return nil, utils.ErrClassAlreadyExists.WithField("name", "CLASS_NAME_TAKEN", "Nama kelas sudah terdaftar pada tahun ajaran ini")
	}

	isActive := true
	if req.IsActive != nil {
		isActive = *req.IsActive
	}

	class := &models.Class{
		Name:           name,
		GradeLevel:     req.GradeLevel,
		AcademicYearID: req.AcademicYearID,
		IsActive:       isActive,
	}

	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		txClasses := s.classes.WithTx(tx)
		if err := txClasses.Create(class); err != nil {
			return err
		}
		snapshot, err := marshalAuditSnapshot(toClassResponse(class, 0, nil))
		if err != nil {
			return utils.ErrInternal
		}
		return s.audit.LogTx(ctx, tx, AuditEvent{
			ActorID:    actorID,
			TargetType: TargetTypeClass,
			TargetID:   class.ID,
			Action:     ActionClassCreated,
			Snapshot:   snapshot,
		})
	})
	if err != nil {
		if appErr := asAppError(err); appErr != nil {
			return nil, appErr
		}
		if mapped := mapClassWriteError(err); mapped != nil {
			return nil, mapped
		}
		return nil, utils.ErrInternal
	}

	return s.Get(ctx, actorID, []string{models.RoleAdmin}, class.ID)
}

// Update memperbarui kelas dengan optimistic locking (updated_at).
func (s *ClassService) Update(ctx context.Context, actorID, id int64, req dto.UpdateClassRequest) (*dto.ClassResponse, error) {
	if req.UpdatedAt == nil {
		return nil, validationError("updated_at", utils.FieldRequired, "updated_at wajib diisi")
	}

	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		txClasses := s.classes.WithTx(tx)

		class, err := txClasses.FindByIDForUpdate(id)
		if err != nil {
			if errors.Is(err, repositories.ErrNotFound) {
				return utils.ErrClassNotFound
			}
			return utils.ErrInternal
		}

		if !sameTimestamp(*req.UpdatedAt, class.UpdatedAt) {
			return utils.ErrStaleData
		}

		before := toClassResponse(class, 0, nil)
		updates := make(map[string]interface{})

		effectiveName := class.Name
		if req.Name != nil {
			name := strings.TrimSpace(*req.Name)
			if name == "" {
				return validationError("name", utils.FieldRequired, "name wajib diisi")
			}
			if len([]rune(name)) > 50 {
				return validationError("name", utils.FieldTooLong, "name maksimal 50 karakter")
			}
			effectiveName = name
			if name != class.Name {
				updates["name"] = name
			}
		}

		if req.GradeLevel != nil {
			if !isValidGradeLevel(*req.GradeLevel) {
				return validationError("grade_level", utils.FieldInvalidGradeLevel, "grade_level harus 10, 11, atau 12")
			}
			if *req.GradeLevel != class.GradeLevel {
				updates["grade_level"] = *req.GradeLevel
			}
		}

		effectiveYearID := class.AcademicYearID
		if req.AcademicYearID != nil {
			if _, err := s.years.WithTx(tx).FindByID(*req.AcademicYearID); err != nil {
				if errors.Is(err, repositories.ErrNotFound) {
					return utils.ErrAcademicYearNotFound
				}
				return utils.ErrInternal
			}
			effectiveYearID = *req.AcademicYearID
			if *req.AcademicYearID != class.AcademicYearID {
				updates["academic_year_id"] = *req.AcademicYearID
			}
		}

		if effectiveName != class.Name || effectiveYearID != class.AcademicYearID {
			exists, err := txClasses.ExistsByNameAndYear(effectiveName, effectiveYearID, id)
			if err != nil {
				return utils.ErrInternal
			}
			if exists {
				return utils.ErrClassAlreadyExists.WithField("name", "CLASS_NAME_TAKEN", "Nama kelas sudah terdaftar pada tahun ajaran ini")
			}
		}

		if req.IsActive != nil && *req.IsActive != class.IsActive {
			updates["is_active"] = *req.IsActive
		}

		if len(updates) == 0 {
			return nil
		}
		updates["updated_at"] = s.now()

		rows, err := txClasses.Update(id, class.UpdatedAt, updates)
		if err != nil {
			return err
		}
		if rows == 0 {
			return utils.ErrStaleData
		}

		updated, err := txClasses.FindByID(id)
		if err != nil {
			return utils.ErrInternal
		}
		after := toClassResponse(updated, 0, nil)
		snapshot, err := marshalAuditSnapshot(map[string]interface{}{
			"before": before,
			"after":  after,
		})
		if err != nil {
			return utils.ErrInternal
		}
		return s.audit.LogTx(ctx, tx, AuditEvent{
			ActorID:    actorID,
			TargetType: TargetTypeClass,
			TargetID:   id,
			Action:     ActionClassUpdated,
			Snapshot:   snapshot,
		})
	})
	if err != nil {
		if appErr := asAppError(err); appErr != nil {
			return nil, appErr
		}
		if mapped := mapClassWriteError(err); mapped != nil {
			return nil, mapped
		}
		return nil, utils.ErrInternal
	}

	return s.Get(ctx, actorID, []string{models.RoleAdmin}, id)
}

// Delete menghapus kelas. Jika masih punya siswa/jadwal (ON DELETE RESTRICT),
// mengembalikan 409 CLASS_IN_USE dengan arahan menonaktifkan.
func (s *ClassService) Delete(ctx context.Context, actorID, id int64) error {
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		txClasses := s.classes.WithTx(tx)

		class, err := txClasses.FindByIDForUpdate(id)
		if err != nil {
			if errors.Is(err, repositories.ErrNotFound) {
				return utils.ErrClassNotFound
			}
			return utils.ErrInternal
		}

		snapshot, err := marshalAuditSnapshot(toClassResponse(class, 0, nil))
		if err != nil {
			return utils.ErrInternal
		}
		if err := s.audit.LogTx(ctx, tx, AuditEvent{
			ActorID:    actorID,
			TargetType: TargetTypeClass,
			TargetID:   id,
			Action:     ActionClassDeleted,
			Snapshot:   snapshot,
		}); err != nil {
			return err
		}

		rows, err := txClasses.Delete(id)
		if err != nil {
			return err
		}
		if rows == 0 {
			return utils.ErrClassNotFound
		}
		return nil
	})
	if err != nil {
		if appErr := asAppError(err); appErr != nil {
			return appErr
		}
		if isForeignKeyViolation(err) {
			return utils.ErrClassInUse
		}
		return utils.ErrInternal
	}
	return nil
}

// canViewClassRoster menentukan apakah pemanggil boleh melihat daftar anggota
// siswa pada kelas tertentu. Staf boleh melihat semua; Siswa/Siswa Inti hanya
// boleh melihat daftar anggota kelasnya sendiri.
func (s *ClassService) canViewClassRoster(actorID int64, actorRoles []string, classID int64) bool {
	for _, r := range actorRoles {
		switch r {
		case models.RoleAdmin, models.RoleWakaKurikulum, models.RoleWakaKesiswaan, models.RoleGuruMapel, models.RoleWaliKelas:
			return true
		}
	}
	student, err := s.students.FindByUserID(actorID)
	if err != nil || student.ClassID == nil {
		return false
	}
	return *student.ClassID == classID
}

func isValidGradeLevel(level int16) bool {
	return level == 10 || level == 11 || level == 12
}

func toClassResponse(class *models.Class, studentCount int64, students []dto.ClassStudentSummary) dto.ClassResponse {
	var yearSummary *dto.ClassAcademicYearSummary
	if class.AcademicYear != nil && class.AcademicYear.ID > 0 {
		yearSummary = &dto.ClassAcademicYearSummary{
			ID:             class.AcademicYear.ID,
			Name:           class.AcademicYear.Name,
			ActiveSemester: class.AcademicYear.ActiveSemester,
			IsActive:       class.AcademicYear.IsActive,
		}
	}
	var waliSummary *dto.ClassWaliKelasSummary
	if class.WaliKelas != nil && class.WaliKelas.ID > 0 {
		waliSummary = &dto.ClassWaliKelasSummary{
			ID:    class.WaliKelas.ID,
			Name:  class.WaliKelas.Name,
			Email: class.WaliKelas.Email,
		}
	}
	return dto.ClassResponse{
		ID:              class.ID,
		Name:            class.Name,
		GradeLevel:      class.GradeLevel,
		AcademicYearID:  class.AcademicYearID,
		AcademicYear:    yearSummary,
		WaliKelasUserID: class.WaliKelasUserID,
		WaliKelas:       waliSummary,
		IsActive:        class.IsActive,
		StudentCount:    studentCount,
		Students:        students,
		UpdatedAt:       class.UpdatedAt,
	}
}

func mapClassWriteError(err error) *utils.AppError {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "23505" {
		return nil
	}
	if pgErr.ConstraintName == "uq_classes_name_year" {
		return utils.ErrClassAlreadyExists.WithField("name", "CLASS_NAME_TAKEN", "Nama kelas sudah terdaftar pada tahun ajaran ini")
	}
	return utils.ErrConflict
}
