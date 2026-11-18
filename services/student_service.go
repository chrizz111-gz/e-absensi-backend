package services

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"

	"e-absensi-backend/dto"
	"e-absensi-backend/models"
	"e-absensi-backend/repositories"
	"e-absensi-backend/utils"
)

var studentNISNPattern = regexp.MustCompile(`^[0-9]{10}$`)

// StudentService menangani logika bisnis Data Siswa (Epic 3 F3.1).
// Sesuai PRD, tidak ada endpoint POST maupun DELETE untuk siswa.
type StudentService struct {
	db       *gorm.DB
	students *repositories.StudentRepository
	users    *repositories.UserRepository
	classes  *repositories.ClassRepository
	audit    AuditLogger
	now      func() time.Time
}

// NewStudentService membuat StudentService baru.
func NewStudentService(db *gorm.DB, audit AuditLogger) *StudentService {
	if audit == nil {
		audit = NewNoopAuditLogger()
	}
	return &StudentService{
		db:       db,
		students: repositories.NewStudentRepository(db),
		users:    repositories.NewUserRepository(db),
		classes:  repositories.NewClassRepository(db),
		audit:    audit,
		now:      time.Now,
	}
}

// List mengambil daftar siswa. Jika pemanggil adalah Wali Kelas (tanpa role
// Admin / Waka Kurikulum / Waka Kesiswaan), cakupan data otomatis dibatasi
// server-side hanya pada kelas tanggung jawabnya.
func (s *StudentService) List(ctx context.Context, actorID int64, actorRoles []string, filter dto.StudentListFilter) (*dto.StudentListResponse, error) {
	page, limit, err := normalizePageLimit(filter.Page, filter.Limit)
	if err != nil {
		return nil, err
	}

	restrictByClass := false
	var allowedClassIDs []int64

	if isWaliKelasScopedOnly(actorRoles) {
		restrictByClass = true
		allowedClassIDs, err = s.classes.FindIDsByWaliKelas(actorID)
		if err != nil {
			return nil, utils.ErrInternal
		}
		if filter.Unassigned {
			return nil, utils.ErrForbidden
		}
		if filter.ClassID != nil && !containsInt64(allowedClassIDs, *filter.ClassID) {
			return nil, utils.ErrForbidden
		}
	}

	items, total, err := s.students.List(repositories.StudentListParams{
		Page:            page,
		Limit:           limit,
		Q:               strings.TrimSpace(filter.Q),
		ClassID:         filter.ClassID,
		AllowedClassIDs: allowedClassIDs,
		RestrictByClass: restrictByClass,
		Unassigned:      filter.Unassigned,
		IsActive:        filter.IsActive,
		IncludeArchived: filter.IncludeArchived,
	})
	if err != nil {
		return nil, utils.ErrInternal
	}

	respItems := make([]dto.StudentResponse, 0, len(items))
	for i := range items {
		respItems = append(respItems, toStudentResponse(&items[i]))
	}

	return &dto.StudentListResponse{
		Items: respItems,
		Meta:  buildPaginationMeta(page, limit, total),
	}, nil
}

// Get mengambil detail satu siswa. Jika diakses Wali Kelas untuk siswa di luar
// kelas tanggung jawabnya, ditolak 403 Forbidden (mencegah IDOR).
func (s *StudentService) Get(ctx context.Context, actorID int64, actorRoles []string, id int64) (*dto.StudentResponse, error) {
	student, err := s.students.FindByID(id)
	if err != nil {
		if errors.Is(err, repositories.ErrNotFound) {
			return nil, utils.ErrStudentNotFound
		}
		return nil, utils.ErrInternal
	}

	if isWaliKelasScopedOnly(actorRoles) {
		allowedClassIDs, err := s.classes.FindIDsByWaliKelas(actorID)
		if err != nil {
			return nil, utils.ErrInternal
		}
		if student.ClassID == nil || !containsInt64(allowedClassIDs, *student.ClassID) {
			return nil, utils.ErrForbidden
		}
	}

	resp := toStudentResponse(student)
	return &resp, nil
}

// Update memperbarui data siswa (whitelist: name, nisn, class_id) dengan
// optimistic locking (updated_at) dan menyinkronkan perubahan name/nisn ke
// tabel users dalam satu transaksi.
func (s *StudentService) Update(ctx context.Context, actorID, id int64, req dto.UpdateStudentRequest) (*dto.StudentResponse, error) {
	if req.UpdatedAt == nil {
		return nil, validationError("updated_at", utils.FieldRequired, "updated_at wajib diisi")
	}

	now := s.now()
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		txStudents := s.students.WithTx(tx)
		txUsers := s.users.WithTx(tx)
		txClasses := s.classes.WithTx(tx)

		locked, err := txStudents.FindByIDForUpdate(id)
		if err != nil {
			if errors.Is(err, repositories.ErrNotFound) {
				return utils.ErrStudentNotFound
			}
			return utils.ErrInternal
		}

		if !sameTimestamp(*req.UpdatedAt, locked.UpdatedAt) {
			return utils.ErrStaleData
		}

		user, err := txUsers.FindByIDForUpdate(locked.UserID)
		if err != nil {
			return utils.ErrInternal
		}

		currentFull, err := txStudents.FindByID(id)
		if err != nil {
			return utils.ErrInternal
		}
		before := toStudentResponse(currentFull)

		studentUpdates := make(map[string]interface{})
		userUpdates := make(map[string]interface{})

		if req.Name != nil {
			name := strings.TrimSpace(*req.Name)
			if err := validateName(name); err != nil {
				return err
			}
			if name != user.Name {
				userUpdates["name"] = name
			}
		}

		if req.NISN != nil {
			nisn := strings.TrimSpace(*req.NISN)
			if !studentNISNPattern.MatchString(nisn) {
				return validationError("nisn", utils.FieldInvalidNISN, "nisn harus 10 digit angka")
			}
			if nisn != locked.NISN {
				exists, err := txStudents.ExistsByNISN(nisn, locked.ID, locked.UserID)
				if err != nil {
					return utils.ErrInternal
				}
				if exists {
					return utils.ErrConflict.WithField("nisn", utils.FieldNISNTaken, "NISN sudah terdaftar")
				}
				studentUpdates["nisn"] = nisn
				userUpdates["nisn"] = nisn
			}
		}

		if req.UnassignClass {
			if locked.ClassID != nil {
				studentUpdates["class_id"] = nil
			}
		} else if req.ClassID != nil {
			if *req.ClassID <= 0 {
				if locked.ClassID != nil {
					studentUpdates["class_id"] = nil
				}
			} else {
				targetClass, err := txClasses.FindByID(*req.ClassID)
				if err != nil {
					if errors.Is(err, repositories.ErrNotFound) {
						return utils.ErrClassNotFound
					}
					return utils.ErrInternal
				}
				if !targetClass.IsActive {
					return validationError("class_id", utils.FieldClassInactive, "Kelas tujuan harus berstatus aktif")
				}
				if locked.ClassID == nil || *locked.ClassID != *req.ClassID {
					studentUpdates["class_id"] = *req.ClassID
				}
			}
		}

		if len(studentUpdates) == 0 && len(userUpdates) == 0 {
			return nil
		}

		studentUpdates["updated_at"] = now
		rows, err := txStudents.Update(id, locked.UpdatedAt, studentUpdates)
		if err != nil {
			return err
		}
		if rows == 0 {
			return utils.ErrStaleData
		}

		if len(userUpdates) > 0 {
			userUpdates["updated_at"] = now
			if err := tx.Model(&models.User{}).Where("id = ?", locked.UserID).Updates(userUpdates).Error; err != nil {
				return err
			}
		}

		updatedFull, err := txStudents.FindByID(id)
		if err != nil {
			return utils.ErrInternal
		}
		after := toStudentResponse(updatedFull)
		snapshot, err := marshalAuditSnapshot(map[string]interface{}{
			"before": before,
			"after":  after,
		})
		if err != nil {
			return utils.ErrInternal
		}
		return s.audit.LogTx(ctx, tx, AuditEvent{
			ActorID:    actorID,
			TargetType: TargetTypeStudent,
			TargetID:   id,
			Action:     ActionStudentUpdated,
			Snapshot:   snapshot,
		})
	})
	if err != nil {
		if appErr := asAppError(err); appErr != nil {
			return nil, appErr
		}
		if mapped := mapStudentWriteError(err); mapped != nil {
			return nil, mapped
		}
		return nil, utils.ErrInternal
	}

	return s.Get(ctx, actorID, []string{models.RoleAdmin}, id)
}

// isWaliKelasScopedOnly mengembalikan true bila user adalah Wali Kelas dan
// TIDAK memiliki role tingkat sekolah (Admin / Waka Kurikulum / Waka Kesiswaan).
func isWaliKelasScopedOnly(roles []string) bool {
	hasSchoolWide := false
	hasWaliKelas := false
	for _, r := range roles {
		switch r {
		case models.RoleAdmin, models.RoleWakaKurikulum, models.RoleWakaKesiswaan:
			hasSchoolWide = true
		case models.RoleWaliKelas:
			hasWaliKelas = true
		}
	}
	return hasWaliKelas && !hasSchoolWide
}

func containsInt64(list []int64, target int64) bool {
	for _, v := range list {
		if v == target {
			return true
		}
	}
	return false
}

func toStudentResponse(student *models.Student) dto.StudentResponse {
	var classSummary *dto.StudentClassSummary
	if student.Class != nil && student.Class.ID > 0 {
		classSummary = &dto.StudentClassSummary{
			ID:             student.Class.ID,
			Name:           student.Class.Name,
			GradeLevel:     student.Class.GradeLevel,
			AcademicYearID: student.Class.AcademicYearID,
			IsActive:       student.Class.IsActive,
		}
	}
	return dto.StudentResponse{
		ID:             student.ID,
		UserID:         student.UserID,
		Name:           student.User.Name,
		Email:          student.User.Email,
		AvatarURL:      student.User.AvatarURL,
		NISN:           student.NISN,
		ClassID:        student.ClassID,
		Class:          classSummary,
		IsActive:       student.User.IsActive,
		InactiveReason: student.User.InactiveReason,
		DeactivatedAt:  student.User.DeactivatedAt,
		ArchivedAt:     student.User.ArchivedAt,
		UpdatedAt:      student.UpdatedAt,
	}
}

func mapStudentWriteError(err error) *utils.AppError {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "23505" {
		return nil
	}
	switch pgErr.ConstraintName {
	case "uq_students_nisn", "uq_users_nisn":
		return utils.ErrConflict.WithField("nisn", utils.FieldNISNTaken, "NISN sudah terdaftar")
	default:
		return utils.ErrConflict
	}
}
