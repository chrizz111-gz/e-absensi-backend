package repositories

import (
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"e-absensi-backend/models"
	"e-absensi-backend/utils"
)

// AcademicYearListParams adalah parameter query daftar tahun ajaran.
type AcademicYearListParams struct {
	Page           int
	Limit          int
	Q              string
	IsActive       *bool
	ActiveSemester string
}

// AcademicYearRepository menangani query tabel academic_years (Epic 3 F3.5).
type AcademicYearRepository struct {
	db *gorm.DB
}

// NewAcademicYearRepository membuat AcademicYearRepository baru.
func NewAcademicYearRepository(db *gorm.DB) *AcademicYearRepository {
	return &AcademicYearRepository{db: db}
}

// WithTx mengembalikan repository yang terikat pada transaksi tertentu.
func (r *AcademicYearRepository) WithTx(tx *gorm.DB) *AcademicYearRepository {
	return &AcademicYearRepository{db: tx}
}

// FindByID mencari tahun ajaran berdasarkan primary key.
func (r *AcademicYearRepository) FindByID(id int64) (*models.AcademicYear, error) {
	var year models.AcademicYear
	if err := r.db.First(&year, "id = ?", id).Error; err != nil {
		return nil, translate(err)
	}
	return &year, nil
}

// FindByIDForUpdate mengunci baris tahun ajaran sampai transaksi selesai.
func (r *AcademicYearRepository) FindByIDForUpdate(id int64) (*models.AcademicYear, error) {
	var year models.AcademicYear
	err := r.db.Clauses(clause.Locking{Strength: "UPDATE"}).
		First(&year, "id = ?", id).Error
	if err != nil {
		return nil, translate(err)
	}
	return &year, nil
}

// FindActive mengambil tahun ajaran yang sedang aktif (is_active = true).
func (r *AcademicYearRepository) FindActive() (*models.AcademicYear, error) {
	var year models.AcademicYear
	if err := r.db.Where("is_active = true").First(&year).Error; err != nil {
		return nil, translate(err)
	}
	return &year, nil
}

// ExistsByName memeriksa apakah nama tahun ajaran sudah dipakai (case-insensitive).
func (r *AcademicYearRepository) ExistsByName(name string, excludeID int64) (bool, error) {
	var count int64
	query := r.db.Model(&models.AcademicYear{}).
		Where("lower(name) = lower(?)", strings.TrimSpace(name))
	if excludeID > 0 {
		query = query.Where("id <> ?", excludeID)
	}
	err := query.Count(&count).Error
	return count > 0, err
}

// List mengambil satu halaman tahun ajaran beserta total baris.
func (r *AcademicYearRepository) List(params AcademicYearListParams) ([]models.AcademicYear, int64, error) {
	applyFilters := func(db *gorm.DB) *gorm.DB {
		query := db.Model(&models.AcademicYear{})
		if q := strings.TrimSpace(params.Q); q != "" {
			like := "%" + utils.EscapeLike(q) + "%"
			query = query.Where(`name ILIKE ? ESCAPE '\'`, like)
		}
		if params.IsActive != nil {
			query = query.Where("is_active = ?", *params.IsActive)
		}
		if sem := strings.TrimSpace(params.ActiveSemester); sem != "" {
			query = query.Where("active_semester = ?", sem)
		}
		return query
	}

	var total int64
	if err := applyFilters(r.db).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var items []models.AcademicYear
	offset := (params.Page - 1) * params.Limit
	err := applyFilters(r.db).
		Order("is_active DESC, start_date DESC, id DESC").
		Offset(offset).
		Limit(params.Limit).
		Find(&items).Error
	if err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

// DeactivateAll menonaktifkan seluruh tahun ajaran yang sedang aktif.
// Dipanggil dalam transaksi yang sama sebelum mengaktifkan tahun ajaran baru.
func (r *AcademicYearRepository) DeactivateAll(excludeID int64) error {
	query := r.db.Model(&models.AcademicYear{}).Where("is_active = true")
	if excludeID > 0 {
		query = query.Where("id <> ?", excludeID)
	}
	return query.Update("is_active", false).Error
}

// Create menyimpan tahun ajaran baru.
func (r *AcademicYearRepository) Create(year *models.AcademicYear) error {
	return r.db.Create(year).Error
}

// Update menerapkan perubahan dengan optimistic locking pada updated_at.
func (r *AcademicYearRepository) Update(id int64, expectedUpdatedAt time.Time, updates map[string]interface{}) (int64, error) {
	res := r.db.Model(&models.AcademicYear{}).
		Where("id = ? AND updated_at = ?", id, expectedUpdatedAt).
		Updates(updates)
	return res.RowsAffected, res.Error
}

// Delete menghapus tahun ajaran berdasarkan ID.
func (r *AcademicYearRepository) Delete(id int64) (int64, error) {
	res := r.db.Delete(&models.AcademicYear{}, id)
	return res.RowsAffected, res.Error
}
