package repositories

import (
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"e-absensi-backend/models"
	"e-absensi-backend/utils"
)

// ClassListParams adalah parameter query daftar kelas.
type ClassListParams struct {
	Page           int
	Limit          int
	Q              string
	AcademicYearID *int64
	GradeLevel     *int16
	IsActive       *bool
}

// ClassRepository menangani query tabel classes (Epic 3 F3.3).
type ClassRepository struct {
	db *gorm.DB
}

// NewClassRepository membuat ClassRepository baru.
func NewClassRepository(db *gorm.DB) *ClassRepository {
	return &ClassRepository{db: db}
}

// WithTx mengembalikan repository yang terikat pada transaksi tertentu.
func (r *ClassRepository) WithTx(tx *gorm.DB) *ClassRepository {
	return &ClassRepository{db: tx}
}

// FindByID mencari kelas berdasarkan primary key beserta relasi tahun ajaran
// dan wali kelas.
func (r *ClassRepository) FindByID(id int64) (*models.Class, error) {
	var class models.Class
	err := r.db.
		Preload("AcademicYear").
		Preload("WaliKelas").
		First(&class, "id = ?", id).Error
	if err != nil {
		return nil, translate(err)
	}
	return &class, nil
}

// FindByIDForUpdate mengunci baris kelas sampai transaksi selesai.
func (r *ClassRepository) FindByIDForUpdate(id int64) (*models.Class, error) {
	var class models.Class
	err := r.db.Clauses(clause.Locking{Strength: "UPDATE"}).
		First(&class, "id = ?", id).Error
	if err != nil {
		return nil, translate(err)
	}
	return &class, nil
}

// FindActiveByName mencari kelas aktif berdasarkan nama (case-insensitive),
// memprioritaskan kelas pada tahun ajaran aktif.
func (r *ClassRepository) FindActiveByName(name string) (*models.Class, error) {
	var class models.Class
	err := r.db.Model(&models.Class{}).
		Select("classes.*").
		Joins("JOIN academic_years ay ON ay.id = classes.academic_year_id").
		Where("lower(classes.name) = lower(?) AND classes.is_active = true", strings.TrimSpace(name)).
		Order("ay.is_active DESC, classes.id DESC").
		Limit(1).
		Take(&class).Error
	if err != nil {
		return nil, translate(err)
	}
	return &class, nil
}

// FindIDsByWaliKelas mengambil daftar class_id yang diampu oleh wali_kelas_user_id tertentu.
func (r *ClassRepository) FindIDsByWaliKelas(userID int64) ([]int64, error) {
	var ids []int64
	err := r.db.Model(&models.Class{}).
		Where("wali_kelas_user_id = ?", userID).
		Pluck("id", &ids).Error
	return ids, err
}

// ExistsByNameAndYear memeriksa keunikan nama kelas pada satu tahun ajaran.
func (r *ClassRepository) ExistsByNameAndYear(name string, academicYearID, excludeID int64) (bool, error) {
	var count int64
	query := r.db.Model(&models.Class{}).
		Where("lower(name) = lower(?) AND academic_year_id = ?", strings.TrimSpace(name), academicYearID)
	if excludeID > 0 {
		query = query.Where("id <> ?", excludeID)
	}
	err := query.Count(&count).Error
	return count > 0, err
}

// List mengambil satu halaman kelas beserta total baris.
func (r *ClassRepository) List(params ClassListParams) ([]models.Class, int64, error) {
	applyFilters := func(db *gorm.DB) *gorm.DB {
		query := db.Model(&models.Class{})
		if q := strings.TrimSpace(params.Q); q != "" {
			like := "%" + utils.EscapeLike(q) + "%"
			query = query.Where(`classes.name ILIKE ? ESCAPE '\'`, like)
		}
		if params.AcademicYearID != nil {
			query = query.Where("classes.academic_year_id = ?", *params.AcademicYearID)
		}
		if params.GradeLevel != nil {
			query = query.Where("classes.grade_level = ?", *params.GradeLevel)
		}
		if params.IsActive != nil {
			query = query.Where("classes.is_active = ?", *params.IsActive)
		}
		return query
	}

	var total int64
	if err := applyFilters(r.db).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var items []models.Class
	offset := (params.Page - 1) * params.Limit
	err := applyFilters(r.db).
		Preload("AcademicYear").
		Preload("WaliKelas").
		Order("classes.grade_level ASC, classes.name ASC, classes.id ASC").
		Offset(offset).
		Limit(params.Limit).
		Find(&items).Error
	if err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

// CountStudentsByClassIDs menghitung jumlah siswa per kelas tanpa N+1.
func (r *ClassRepository) CountStudentsByClassIDs(classIDs []int64) (map[int64]int64, error) {
	result := make(map[int64]int64, len(classIDs))
	if len(classIDs) == 0 {
		return result, nil
	}

	type countRow struct {
		ClassID int64 `gorm:"column:class_id"`
		Total   int64 `gorm:"column:total"`
	}
	var rows []countRow
	err := r.db.Table("students").
		Select("students.class_id, COUNT(*) AS total").
		Joins("JOIN users u ON u.id = students.user_id").
		Where("students.class_id IN ? AND u.archived_at IS NULL", classIDs).
		Group("students.class_id").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		result[row.ClassID] = row.Total
	}
	return result, nil
}

// Create menyimpan kelas baru.
func (r *ClassRepository) Create(class *models.Class) error {
	return r.db.Omit(clause.Associations).Create(class).Error
}

// Update menerapkan perubahan dengan optimistic locking pada updated_at.
func (r *ClassRepository) Update(id int64, expectedUpdatedAt time.Time, updates map[string]interface{}) (int64, error) {
	res := r.db.Model(&models.Class{}).
		Where("id = ? AND updated_at = ?", id, expectedUpdatedAt).
		Updates(updates)
	return res.RowsAffected, res.Error
}

// Delete menghapus kelas berdasarkan ID.
func (r *ClassRepository) Delete(id int64) (int64, error) {
	res := r.db.Delete(&models.Class{}, id)
	return res.RowsAffected, res.Error
}
