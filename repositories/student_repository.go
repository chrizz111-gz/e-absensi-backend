package repositories

import (
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"e-absensi-backend/models"
	"e-absensi-backend/utils"
)

// StudentListParams adalah parameter query daftar siswa.
type StudentListParams struct {
	Page            int
	Limit           int
	Q               string
	ClassID         *int64
	AllowedClassIDs []int64
	RestrictByClass bool
	Unassigned      bool
	IsActive        *bool
	IncludeArchived bool
}

// StudentRepository menangani query tabel students (Epic 3 F3.1).
type StudentRepository struct {
	db *gorm.DB
}

// NewStudentRepository membuat StudentRepository baru.
func NewStudentRepository(db *gorm.DB) *StudentRepository {
	return &StudentRepository{db: db}
}

// WithTx mengembalikan repository yang terikat pada transaksi tertentu.
func (r *StudentRepository) WithTx(tx *gorm.DB) *StudentRepository {
	return &StudentRepository{db: tx}
}

// FindByID mencari profil siswa berdasarkan primary key beserta akun dan kelasnya.
func (r *StudentRepository) FindByID(id int64) (*models.Student, error) {
	var student models.Student
	err := r.db.
		Preload("User").
		Preload("Class").
		First(&student, "id = ?", id).Error
	if err != nil {
		return nil, translate(err)
	}
	return &student, nil
}

// FindByIDForUpdate mengunci baris siswa sampai transaksi selesai.
func (r *StudentRepository) FindByIDForUpdate(id int64) (*models.Student, error) {
	var student models.Student
	err := r.db.Clauses(clause.Locking{Strength: "UPDATE"}).
		First(&student, "id = ?", id).Error
	if err != nil {
		return nil, translate(err)
	}
	return &student, nil
}

// FindByUserID mencari profil siswa berdasarkan user_id.
func (r *StudentRepository) FindByUserID(userID int64) (*models.Student, error) {
	var student models.Student
	err := r.db.
		Preload("User").
		Preload("Class").
		Where("user_id = ?", userID).
		First(&student).Error
	if err != nil {
		return nil, translate(err)
	}
	return &student, nil
}

// ExistsByNISN memeriksa apakah NISN sudah dipakai oleh siswa/user lain.
func (r *StudentRepository) ExistsByNISN(nisn string, excludeStudentID, excludeUserID int64) (bool, error) {
	var count int64
	q := strings.TrimSpace(nisn)
	err := r.db.Model(&models.Student{}).
		Where("nisn = ? AND id <> ?", q, excludeStudentID).
		Count(&count).Error
	if err != nil || count > 0 {
		return count > 0, err
	}

	err = r.db.Model(&models.User{}).
		Where("nisn = ? AND id <> ?", q, excludeUserID).
		Count(&count).Error
	return count > 0, err
}

// ListByClassID mengambil seluruh anggota siswa pada satu kelas (untuk detail kelas).
func (r *StudentRepository) ListByClassID(classID int64) ([]models.Student, error) {
	var students []models.Student
	err := r.db.Model(&models.Student{}).
		Select("students.*").
		Joins("JOIN users u ON u.id = students.user_id").
		Preload("User").
		Where("students.class_id = ? AND u.archived_at IS NULL", classID).
		Order("u.name ASC, students.id ASC").
		Find(&students).Error
	return students, err
}

// List mengambil satu halaman siswa beserta total baris.
func (r *StudentRepository) List(params StudentListParams) ([]models.Student, int64, error) {
	if params.RestrictByClass && len(params.AllowedClassIDs) == 0 {
		return []models.Student{}, 0, nil
	}

	applyFilters := func(db *gorm.DB) *gorm.DB {
		query := db.Model(&models.Student{}).
			Joins("JOIN users u ON u.id = students.user_id")

		if !params.IncludeArchived {
			query = query.Where("u.archived_at IS NULL")
		}
		if params.RestrictByClass {
			query = query.Where("students.class_id IN ?", params.AllowedClassIDs)
		}
		if params.Unassigned {
			query = query.Where("students.class_id IS NULL")
		} else if params.ClassID != nil {
			query = query.Where("students.class_id = ?", *params.ClassID)
		}
		if params.IsActive != nil {
			query = query.Where("u.is_active = ?", *params.IsActive)
		}
		if q := strings.TrimSpace(params.Q); q != "" {
			like := "%" + utils.EscapeLike(q) + "%"
			query = query.Where(
				`(u.name ILIKE ? ESCAPE '\' OR u.email ILIKE ? ESCAPE '\' OR students.nisn ILIKE ? ESCAPE '\')`,
				like, like, like,
			)
		}
		return query
	}

	var total int64
	if err := applyFilters(r.db).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var items []models.Student
	offset := (params.Page - 1) * params.Limit
	err := applyFilters(r.db).
		Select("students.*").
		Preload("User").
		Preload("Class").
		Order("u.name ASC, students.id ASC").
		Offset(offset).
		Limit(params.Limit).
		Find(&items).Error
	if err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

// Update menerapkan perubahan profil siswa dengan optimistic locking pada updated_at.
func (r *StudentRepository) Update(id int64, expectedUpdatedAt time.Time, updates map[string]interface{}) (int64, error) {
	res := r.db.Model(&models.Student{}).
		Where("id = ? AND updated_at = ?", id, expectedUpdatedAt).
		Updates(updates)
	return res.RowsAffected, res.Error
}
