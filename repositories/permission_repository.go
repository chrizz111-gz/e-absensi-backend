package repositories

import (
	"gorm.io/gorm"

	"e-absensi-backend/models"
)

// PermissionRepository menangani kamus permission dan matriks role_permissions.
type PermissionRepository struct {
	db *gorm.DB
}

// NewPermissionRepository membuat PermissionRepository baru.
func NewPermissionRepository(db *gorm.DB) *PermissionRepository {
	return &PermissionRepository{db: db}
}

// WithTx mengembalikan repository yang terikat pada transaksi tertentu.
func (r *PermissionRepository) WithTx(tx *gorm.DB) *PermissionRepository {
	return &PermissionRepository{db: tx}
}

// UserHasPermission memeriksa apakah minimal satu role user memiliki permission
// dengan kode tertentu. Query langsung ke pivot agar tidak mempercayai klaim
// role/permission dari token atau frontend.
func (r *PermissionRepository) UserHasPermission(userID int64, permissionCode string) (bool, error) {
	var allowed bool
	err := r.db.Raw(`
		SELECT EXISTS (
			SELECT 1
			FROM user_roles ur
			JOIN role_permissions rp ON rp.role_id = ur.role_id
			JOIN permissions p ON p.id = rp.permission_id
			WHERE ur.user_id = ? AND p.code = ?
		)`, userID, permissionCode).Scan(&allowed).Error
	if err != nil {
		return false, err
	}
	return allowed, nil
}

// List mengembalikan seluruh kamus permission, diurutkan per modul lalu kode.
func (r *PermissionRepository) List() ([]models.Permission, error) {
	var permissions []models.Permission
	if err := r.db.Order("module, code").Find(&permissions).Error; err != nil {
		return nil, err
	}
	return permissions, nil
}

// ListGrants mengembalikan seluruh pasangan role-permission pada matriks.
func (r *PermissionRepository) ListGrants() ([]models.RolePermission, error) {
	var grants []models.RolePermission
	if err := r.db.Order("role_id, permission_id").Find(&grants).Error; err != nil {
		return nil, err
	}
	return grants, nil
}
