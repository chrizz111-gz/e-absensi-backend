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

// TeacherService menangani logika bisnis Data Guru (Epic 3 F3.2).
// Sesuai PRD, tidak ada fitur DELETE untuk profil guru.
type TeacherService struct {
	db       *gorm.DB
	teachers *repositories.TeacherRepository
	users    *repositories.UserRepository
	subjects *repositories.SubjectRepository
	audit    AuditLogger
	now      func() time.Time
}

// NewTeacherService membuat TeacherService baru.
func NewTeacherService(db *gorm.DB, audit AuditLogger) *TeacherService {
	if audit == nil {
		audit = NewNoopAuditLogger()
	}
	return &TeacherService{
		db:       db,
		teachers: repositories.NewTeacherRepository(db),
		users:    repositories.NewUserRepository(db),
		subjects: repositories.NewSubjectRepository(db),
		audit:    audit,
		now:      time.Now,
	}
}

// ListCandidates mengembalikan daftar akun ber-role guru_mapel yang belum punya
// profil guru (helper dropdown F3.2).
func (s *TeacherService) ListCandidates(ctx context.Context, q string) ([]dto.TeacherCandidateResponse, error) {
	users, err := s.teachers.ListCandidates(q)
	if err != nil {
		return nil, utils.ErrInternal
	}
	items := make([]dto.TeacherCandidateResponse, 0, len(users))
	for _, u := range users {
		items = append(items, dto.TeacherCandidateResponse{
			UserID:    u.ID,
			Name:      u.Name,
			Email:     u.Email,
			AvatarURL: u.AvatarURL,
			IsActive:  u.IsActive,
		})
	}
	return items, nil
}

// List mengambil daftar guru dengan pencarian dan filter.
func (s *TeacherService) List(ctx context.Context, filter dto.TeacherListFilter) (*dto.TeacherListResponse, error) {
	page, limit, err := normalizePageLimit(filter.Page, filter.Limit)
	if err != nil {
		return nil, err
	}

	items, total, err := s.teachers.List(repositories.TeacherListParams{
		Page:      page,
		Limit:     limit,
		Q:         strings.TrimSpace(filter.Q),
		SubjectID: filter.SubjectID,
		IsActive:  filter.IsActive,
	})
	if err != nil {
		return nil, utils.ErrInternal
	}

	respItems := make([]dto.TeacherResponse, 0, len(items))
	for i := range items {
		respItems = append(respItems, toTeacherResponse(&items[i]))
	}

	return &dto.TeacherListResponse{
		Items: respItems,
		Meta:  buildPaginationMeta(page, limit, total),
	}, nil
}

// Get mengambil detail satu guru beserta akun dan mata pelajaran yang diampu.
func (s *TeacherService) Get(ctx context.Context, id int64) (*dto.TeacherResponse, error) {
	teacher, err := s.teachers.FindByID(id)
	if err != nil {
		if errors.Is(err, repositories.ErrNotFound) {
			return nil, utils.ErrTeacherNotFound
		}
		return nil, utils.ErrInternal
	}
	resp := toTeacherResponse(teacher)
	return &resp, nil
}

// Create membuat profil guru baru pada akun guru_mapel yang belum punya profil.
func (s *TeacherService) Create(ctx context.Context, actorID int64, req dto.CreateTeacherRequest) (*dto.TeacherResponse, error) {
	if req.UserID <= 0 {
		return nil, validationError("user_id", utils.FieldInvalidID, "user_id tidak valid")
	}

	nip := strings.TrimSpace(req.NIP)
	if nip == "" {
		return nil, validationError("nip", utils.FieldRequired, "nip wajib diisi")
	}
	if len([]rune(nip)) > 30 {
		return nil, validationError("nip", utils.FieldTooLong, "nip maksimal 30 karakter")
	}

	subjectIDs, err := s.validateActiveSubjects(req.SubjectIDs)
	if err != nil {
		return nil, err
	}

	if _, err := s.users.FindByID(req.UserID); err != nil {
		if errors.Is(err, repositories.ErrNotFound) {
			return nil, utils.ErrUserNotFound
		}
		return nil, utils.ErrInternal
	}

	roles, err := s.users.GetRoles(req.UserID)
	if err != nil {
		return nil, utils.ErrInternal
	}
	if !hasRole(roles, models.RoleGuruMapel) {
		return nil, utils.ErrAccountNotTeacherRole.WithField("user_id", utils.CodeAccountNotTeacherRole, "Akun yang dipilih harus memiliki role guru_mapel")
	}

	exists, err := s.teachers.ExistsByUserID(req.UserID)
	if err != nil {
		return nil, utils.ErrInternal
	}
	if exists {
		return nil, utils.ErrTeacherAlreadyExists.WithField("user_id", utils.CodeTeacherAlreadyExists, "Akun ini sudah memiliki profil guru")
	}

	teacher := &models.Teacher{
		UserID: req.UserID,
		NIP:    nip,
	}

	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		txTeachers := s.teachers.WithTx(tx)
		if err := txTeachers.Create(teacher); err != nil {
			return err
		}
		if err := txTeachers.ReplaceSubjects(teacher.ID, subjectIDs); err != nil {
			return err
		}
		created, err := txTeachers.FindByID(teacher.ID)
		if err != nil {
			return utils.ErrInternal
		}
		snapshot, err := marshalAuditSnapshot(toTeacherResponse(created))
		if err != nil {
			return utils.ErrInternal
		}
		return s.audit.LogTx(ctx, tx, AuditEvent{
			ActorID:    actorID,
			TargetType: TargetTypeTeacher,
			TargetID:   teacher.ID,
			Action:     ActionTeacherCreated,
			Snapshot:   snapshot,
		})
	})
	if err != nil {
		if appErr := asAppError(err); appErr != nil {
			return nil, appErr
		}
		if mapped := mapTeacherWriteError(err); mapped != nil {
			return nil, mapped
		}
		return nil, utils.ErrInternal
	}

	return s.Get(ctx, teacher.ID)
}

// Update memperbarui NIP dan daftar mata pelajaran guru dengan optimistic
// locking (updated_at).
func (s *TeacherService) Update(ctx context.Context, actorID, id int64, req dto.UpdateTeacherRequest) (*dto.TeacherResponse, error) {
	if req.UpdatedAt == nil {
		return nil, validationError("updated_at", utils.FieldRequired, "updated_at wajib diisi")
	}

	var validatedSubjectIDs []int64
	if req.SubjectIDs != nil {
		ids, err := s.validateActiveSubjects(*req.SubjectIDs)
		if err != nil {
			return nil, err
		}
		validatedSubjectIDs = ids
	}

	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		txTeachers := s.teachers.WithTx(tx)

		locked, err := txTeachers.FindByIDForUpdate(id)
		if err != nil {
			if errors.Is(err, repositories.ErrNotFound) {
				return utils.ErrTeacherNotFound
			}
			return utils.ErrInternal
		}

		if !sameTimestamp(*req.UpdatedAt, locked.UpdatedAt) {
			return utils.ErrStaleData
		}

		currentFull, err := txTeachers.FindByID(id)
		if err != nil {
			return utils.ErrInternal
		}
		before := toTeacherResponse(currentFull)

		updates := make(map[string]interface{})
		if req.NIP != nil {
			nip := strings.TrimSpace(*req.NIP)
			if nip == "" {
				return validationError("nip", utils.FieldRequired, "nip wajib diisi")
			}
			if len([]rune(nip)) > 30 {
				return validationError("nip", utils.FieldTooLong, "nip maksimal 30 karakter")
			}
			if nip != locked.NIP {
				updates["nip"] = nip
			}
		}

		if len(updates) == 0 && req.SubjectIDs == nil {
			return nil
		}

		updates["updated_at"] = s.now()
		rows, err := txTeachers.Update(id, locked.UpdatedAt, updates)
		if err != nil {
			return err
		}
		if rows == 0 {
			return utils.ErrStaleData
		}

		if req.SubjectIDs != nil {
			if err := txTeachers.ReplaceSubjects(id, validatedSubjectIDs); err != nil {
				return err
			}
		}

		updatedFull, err := txTeachers.FindByID(id)
		if err != nil {
			return utils.ErrInternal
		}
		after := toTeacherResponse(updatedFull)
		snapshot, err := marshalAuditSnapshot(map[string]interface{}{
			"before": before,
			"after":  after,
		})
		if err != nil {
			return utils.ErrInternal
		}
		return s.audit.LogTx(ctx, tx, AuditEvent{
			ActorID:    actorID,
			TargetType: TargetTypeTeacher,
			TargetID:   id,
			Action:     ActionTeacherUpdated,
			Snapshot:   snapshot,
		})
	})
	if err != nil {
		if appErr := asAppError(err); appErr != nil {
			return nil, appErr
		}
		return nil, utils.ErrInternal
	}

	return s.Get(ctx, id)
}

func (s *TeacherService) validateActiveSubjects(rawIDs []int64) ([]int64, error) {
	seen := make(map[int64]bool, len(rawIDs))
	unique := make([]int64, 0, len(rawIDs))
	for _, id := range rawIDs {
		if id <= 0 || seen[id] {
			continue
		}
		seen[id] = true
		unique = append(unique, id)
	}
	if len(unique) == 0 {
		return nil, validationError("subject_ids", utils.FieldRequired, "Guru wajib mengampu minimal 1 mata pelajaran aktif")
	}

	found, err := s.subjects.FindByIDs(unique)
	if err != nil {
		return nil, utils.ErrInternal
	}
	if len(found) != len(unique) {
		return nil, validationError("subject_ids", "SUBJECT_NOT_FOUND", "Ada mata pelajaran yang tidak ditemukan")
	}
	for _, subj := range found {
		if !subj.IsActive {
			return nil, validationError("subject_ids", utils.FieldSubjectInactive, "Mata pelajaran "+subj.Name+" sedang nonaktif")
		}
	}
	return unique, nil
}

func toTeacherResponse(teacher *models.Teacher) dto.TeacherResponse {
	subjects := make([]dto.TeacherSubjectSummary, 0, len(teacher.Subjects))
	for _, subj := range teacher.Subjects {
		subjects = append(subjects, dto.TeacherSubjectSummary{
			ID:       subj.ID,
			Code:     subj.Code,
			Name:     subj.Name,
			IsActive: subj.IsActive,
		})
	}
	return dto.TeacherResponse{
		ID:        teacher.ID,
		UserID:    teacher.UserID,
		Name:      teacher.User.Name,
		Email:     teacher.User.Email,
		AvatarURL: teacher.User.AvatarURL,
		IsActive:  teacher.User.IsActive,
		NIP:       teacher.NIP,
		Subjects:  subjects,
		UpdatedAt: teacher.UpdatedAt,
	}
}

func mapTeacherWriteError(err error) *utils.AppError {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "23505" {
		return nil
	}
	if pgErr.ConstraintName == "uq_teachers_user" {
		return utils.ErrTeacherAlreadyExists.WithField("user_id", utils.CodeTeacherAlreadyExists, "Akun ini sudah memiliki profil guru")
	}
	return utils.ErrConflict
}
