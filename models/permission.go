package models

// Permission adalah satu izin sistem pada tabel permissions. Kode berformat
// module.action dan dipakai middleware RequirePermission untuk otorisasi.
type Permission struct {
	ID          int64   `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	Code        string  `gorm:"column:code;size:100;not null" json:"code"`
	Module      string  `gorm:"column:module;size:50;not null" json:"module"`
	Description *string `gorm:"column:description;size:255" json:"description,omitempty"`
}

// TableName memetakan Permission ke tabel "permissions".
func (Permission) TableName() string {
	return "permissions"
}

// RolePermission adalah pivot pemberian permission ke role. Matriks ini
// bersifat tetap dan diubah lewat migrasi/seeder, bukan endpoint CRUD.
type RolePermission struct {
	RoleID       int16 `gorm:"column:role_id;primaryKey" json:"role_id"`
	PermissionID int64 `gorm:"column:permission_id;primaryKey" json:"permission_id"`
}

// TableName memetakan RolePermission ke tabel "role_permissions".
func (RolePermission) TableName() string {
	return "role_permissions"
}

// Kode permission Epic 2. Nilai ini harus sama dengan seed pada migrasi 004.
const (
	PermissionAccountView      = "account.view"
	PermissionAccountCreate    = "account.create"
	PermissionAccountUpdate    = "account.update"
	PermissionAccountSetStatus = "account.set_status"
	PermissionAccountDelete    = "account.delete"
	PermissionRoleAssign       = "role.assign"
	PermissionPermissionView   = "permission.view"
)
