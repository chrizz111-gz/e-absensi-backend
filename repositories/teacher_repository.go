package repositories

import (
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"e-absensi-backend/models"
	"e-absensi-backend/utils"
)

// TeacherListParams adalah parameter query daftar guru.
type TeacherListParams struct {
	Page      int
	Limit     int
	Q         string
	SubjectID *int64
	IsActive  *bool
}

// TeacherRepository menangani query tabel teachers dan teacher_subjects (Epic 3 F3.2).
type TeacherRepository struct {
	db *gorm.DB
}

// NewTeacherRepository membuat TeacherRepository baru.
func NewTeacherRepository(db *gorm.DB) *TeacherRepository {
	return &TeacherRepository{db: db}
}

// WithTx mengembalikan repository yang terikat pada transaksi tertentu.
func (r *TeacherRepository) WithTx(tx *gorm.DB) *TeacherRepository {
	return &TeacherRepository{db: tx}
}

// FindByID mencari profil guru berdasarkan primary key beserta akun dan mapelnya.
func (r *TeacherRepository) FindByID(id int64) (*models.Teacher, error) {
	var teacher models.Teacher
	err := r.db.
		Preload("User").
		Preload("Subjects", func(db *gorm.DB) *gorm.DB {
			return db.Order("subjects.name ASC, subjects.id ASC")
		}).
		First(&teacher, "id = ?", id).Error
	if err != nil {
		return nil, translate(err)
	}
	return &teacher, nil
}

// FindByIDForUpdate mengunci baris guru sampai transaksi selesai.
func (r *TeacherRepository) FindByIDForUpdate(id int64) (*models.Teacher, error) {
	var teacher models.Teacher
	err := r.db.Clauses(clause.Locking{Strength: "UPDATE"}).
		First(&teacher, "id = ?", id).Error
	if err != nil {
		return nil, translate(err)
	}
	return &teacher, nil
}

// FindByUserID mencari profil guru berdasarkan user_id.
func (r *TeacherRepository) FindByUserID(userID int64) (*models.Teacher, error) {
	var teacher models.Teacher
	err := r.db.
		Preload("User").
		Preload("Subjects").
		Where("user_id = ?", userID).
		First(&teacher).Error
	if err != nil {
		return nil, translate(err)
	}
	return &teacher, nil
}

// FindByUserEmail mencari profil guru berdasarkan email akunnya (case-insensitive).
func (r *TeacherRepository) FindByUserEmail(email string) (*models.Teacher, error) {
	var teacher models.Teacher
	err := r.db.Model(&models.Teacher{}).
		Select("teachers.*").
		Joins("JOIN users u ON u.id = teachers.user_id").
		Preload("User").
		Preload("Subjects").
		Where("lower(u.email) = lower(?)", strings.TrimSpace(email)).
		First(&teacher).Error
	if err != nil {
		return nil, translate(err)
	}
	return &teacher, nil
}

// ExistsByUserID memeriksa apakah user sudah memiliki profil guru.
func (r *TeacherRepository) ExistsByUserID(userID int64) (bool, error) {
	var count int64
	err := r.db.Model(&models.Teacher{}).Where("user_id = ?", userID).Count(&count).Error
	return count > 0, err
}

// ListCandidates mengambil daftar akun ber-role guru_mapel yang belum punya
// profil guru (untuk dropdown tambah guru F3.2).
func (r *TeacherRepository) ListCandidates(q string) ([]models.User, error) {
	query := r.db.Model(&models.User{}).
		Where("users.archived_at IS NULL").
		Where(`EXISTS (
			SELECT 1
			FROM user_roles ur
			JOIN roles ro ON ro.id = ur.role_id
			WHERE ur.user_id = users.id AND ro.name = ?
		)`, models.RoleGuruMapel).
		Where(`NOT EXISTS (
			SELECT 1
			FROM teachers t
			WHERE t.user_id = users.id
		)`)

	if trimmed := strings.TrimSpace(q); trimmed != "" {
		like := "%" + utils.EscapeLike(trimmed) + "%"
		query = query.Where(`(users.name ILIKE ? ESCAPE '\' OR users.email ILIKE ? ESCAPE '\')`, like, like)
	}

	var users []models.User
	if err := query.Order("users.name ASC, users.id ASC").Find(&users).Error; err != nil {
		return nil, err
	}
	return users, nil
}

// List mengambil satu halaman guru beserta total baris.
func (r *TeacherRepository) List(params TeacherListParams) ([]models.Teacher, int64, error) {
	applyFilters := func(db *gorm.DB) *gorm.DB {
		query := db.Model(&models.Teacher{}).
			Joins("JOIN users u ON u.id = teachers.user_id").
			Where("u.archived_at IS NULL")

		if q := strings.TrimSpace(params.Q); q != "" {
			like := "%" + utils.EscapeLike(q) + "%"
			query = query.Where(
				`(u.name ILIKE ? ESCAPE '\' OR u.email ILIKE ? ESCAPE '\' OR teachers.nip ILIKE ? ESCAPE '\')`,
				like, like, like,
			)
		}
		if params.SubjectID != nil {
			query = query.Where(`EXISTS (
				SELECT 1 FROM teacher_subjects ts
				WHERE ts.teacher_id = teachers.id AND ts.subject_id = ?
			)`, *params.SubjectID)
		}
		if params.IsActive != nil {
			query = query.Where("u.is_active = ?", *params.IsActive)
		}
		return query
	}

	var total int64
	if err := applyFilters(r.db).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var items []models.Teacher
	offset := (params.Page - 1) * params.Limit
	err := applyFilters(r.db).
		Select("teachers.*").
		Preload("User").
		Preload("Subjects", func(db *gorm.DB) *gorm.DB {
			return db.Order("subjects.name ASC, subjects.id ASC")
		}).
		Order("u.name ASC, teachers.id ASC").
		Offset(offset).
		Limit(params.Limit).
		Find(&items).Error
	if err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

// Create menyimpan profil guru baru.
func (r *TeacherRepository) Create(teacher *models.Teacher) error {
	return r.db.Omit(clause.Associations).Create(teacher).Error
}

// Update menerapkan perubahan dengan optimistic locking pada updated_at.
func (r *TeacherRepository) Update(id int64, expectedUpdatedAt time.Time, updates map[string]interface{}) (int64, error) {
	res := r.db.Model(&models.Teacher{}).
		Where("id = ? AND updated_at = ?", id, expectedUpdatedAt).
		Updates(updates)
	return res.RowsAffected, res.Error
}

// ReplaceSubjects mengganti seluruh mata pelajaran yang diampu seorang guru.
func (r *TeacherRepository) ReplaceSubjects(teacherID int64, subjectIDs []int64) error {
	if err := r.db.Where("teacher_id = ?", teacherID).Delete(&models.TeacherSubject{}).Error; err != nil {
		return err
	}
	if len(subjectIDs) == 0 {
		return nil
	}
	rows := make([]models.TeacherSubject, 0, len(subjectIDs))
	for _, subjectID := range subjectIDs {
		rows = append(rows, models.TeacherSubject{
			TeacherID: teacherID,
			SubjectID: subjectID,
		})
	}
	return r.db.Clauses(clause.OnConflict{DoNothing: true}).Create(&rows).Error
}
