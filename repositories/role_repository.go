package repositories

import (
	"gorm.io/gorm"

	"e-absensi-backend/models"
)

// RoleRepository menangani query master role.
type RoleRepository struct {
	db *gorm.DB
}

// NewRoleRepository membuat RoleRepository baru.
func NewRoleRepository(db *gorm.DB) *RoleRepository {
	return &RoleRepository{db: db}
}

// WithTx mengembalikan repository yang terikat pada transaksi tertentu.
func (r *RoleRepository) WithTx(tx *gorm.DB) *RoleRepository {
	return &RoleRepository{db: tx}
}

// FindByName mencari satu role berdasarkan nama.
func (r *RoleRepository) FindByName(name string) (*models.Role, error) {
	var role models.Role
	if err := r.db.Where("name = ?", name).First(&role).Error; err != nil {
		return nil, translate(err)
	}
	return &role, nil
}

// FindByNames mencari beberapa role sekaligus berdasarkan nama.
func (r *RoleRepository) FindByNames(names []string) ([]models.Role, error) {
	var roles []models.Role
	if err := r.db.Where("name IN ?", names).Find(&roles).Error; err != nil {
		return nil, err
	}
	return roles, nil
}
