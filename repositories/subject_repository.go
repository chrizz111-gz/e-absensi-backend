package repositories

import (
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"e-absensi-backend/models"
	"e-absensi-backend/utils"
)

// SubjectListParams adalah parameter query daftar mata pelajaran.
type SubjectListParams struct {
	Page     int
	Limit    int
	Q        string
	IsActive *bool
}

// SubjectRepository menangani query tabel subjects (Epic 3 F3.4).
type SubjectRepository struct {
	db *gorm.DB
}

// NewSubjectRepository membuat SubjectRepository baru.
func NewSubjectRepository(db *gorm.DB) *SubjectRepository {
	return &SubjectRepository{db: db}
}

// WithTx mengembalikan repository yang terikat pada transaksi tertentu.
func (r *SubjectRepository) WithTx(tx *gorm.DB) *SubjectRepository {
	return &SubjectRepository{db: tx}
}

// FindByID mencari mata pelajaran berdasarkan primary key.
func (r *SubjectRepository) FindByID(id int64) (*models.Subject, error) {
	var subject models.Subject
	if err := r.db.First(&subject, "id = ?", id).Error; err != nil {
		return nil, translate(err)
	}
	return &subject, nil
}

// FindByIDForUpdate mengunci baris mata pelajaran sampai transaksi selesai.
func (r *SubjectRepository) FindByIDForUpdate(id int64) (*models.Subject, error) {
	var subject models.Subject
	err := r.db.Clauses(clause.Locking{Strength: "UPDATE"}).
		First(&subject, "id = ?", id).Error
	if err != nil {
		return nil, translate(err)
	}
	return &subject, nil
}

// FindByCode mencari mata pelajaran berdasarkan kode (case-insensitive).
func (r *SubjectRepository) FindByCode(code string) (*models.Subject, error) {
	var subject models.Subject
	err := r.db.Where("lower(code) = lower(?)", strings.TrimSpace(code)).First(&subject).Error
	if err != nil {
		return nil, translate(err)
	}
	return &subject, nil
}

// FindByIDs mengambil beberapa mata pelajaran sekaligus berdasarkan daftar ID.
func (r *SubjectRepository) FindByIDs(ids []int64) ([]models.Subject, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	var subjects []models.Subject
	if err := r.db.Where("id IN ?", ids).Order("id").Find(&subjects).Error; err != nil {
		return nil, err
	}
	return subjects, nil
}

// ExistsByCode memeriksa keunikan kode mata pelajaran (case-insensitive).
func (r *SubjectRepository) ExistsByCode(code string, excludeID int64) (bool, error) {
	var count int64
	query := r.db.Model(&models.Subject{}).
		Where("lower(code) = lower(?)", strings.TrimSpace(code))
	if excludeID > 0 {
		query = query.Where("id <> ?", excludeID)
	}
	err := query.Count(&count).Error
	return count > 0, err
}

// List mengambil satu halaman mata pelajaran beserta total baris.
func (r *SubjectRepository) List(params SubjectListParams) ([]models.Subject, int64, error) {
	applyFilters := func(db *gorm.DB) *gorm.DB {
		query := db.Model(&models.Subject{})
		if q := strings.TrimSpace(params.Q); q != "" {
			like := "%" + utils.EscapeLike(q) + "%"
			query = query.Where(`(code ILIKE ? ESCAPE '\' OR name ILIKE ? ESCAPE '\')`, like, like)
		}
		if params.IsActive != nil {
			query = query.Where("is_active = ?", *params.IsActive)
		}
		return query
	}

	var total int64
	if err := applyFilters(r.db).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var items []models.Subject
	offset := (params.Page - 1) * params.Limit
	err := applyFilters(r.db).
		Order("name ASC, id ASC").
		Offset(offset).
		Limit(params.Limit).
		Find(&items).Error
	if err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

// Create menyimpan mata pelajaran baru.
func (r *SubjectRepository) Create(subject *models.Subject) error {
	return r.db.Create(subject).Error
}

// Update menerapkan perubahan dengan optimistic locking pada updated_at.
func (r *SubjectRepository) Update(id int64, expectedUpdatedAt time.Time, updates map[string]interface{}) (int64, error) {
	res := r.db.Model(&models.Subject{}).
		Where("id = ? AND updated_at = ?", id, expectedUpdatedAt).
		Updates(updates)
	return res.RowsAffected, res.Error
}

// Delete menghapus mata pelajaran berdasarkan ID.
func (r *SubjectRepository) Delete(id int64) (int64, error) {
	res := r.db.Delete(&models.Subject{}, id)
	return res.RowsAffected, res.Error
}
