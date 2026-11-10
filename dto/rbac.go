package dto

// RoleResponse adalah representasi aman master role untuk endpoint /roles.
type RoleResponse struct {
	ID           int16   `json:"id"`
	Name         string  `json:"name"`
	Label        string  `json:"label"`
	Category     string  `json:"category"`
	IsAssignable bool    `json:"assignable"`
	Description  *string `json:"description,omitempty"`
}

// RoleListResponse adalah payload endpoint GET /roles.
type RoleListResponse struct {
	Roles []RoleResponse `json:"roles"`
}

// PermissionResponse adalah satu baris kamus permission.
type PermissionResponse struct {
	ID          int64   `json:"id"`
	Code        string  `json:"code"`
	Module      string  `json:"module"`
	Description *string `json:"description,omitempty"`
}

// PermissionGrantResponse adalah satu sel matriks role-permission.
type PermissionGrantResponse struct {
	RoleID       int16 `json:"role_id"`
	PermissionID int64 `json:"permission_id"`
}

// PermissionMatrixResponse adalah data read-only untuk halaman Matriks Hak Akses.
// Frontend membentuk tabel dari roles x permissions memakai grants sebagai sel.
type PermissionMatrixResponse struct {
	Roles       []RoleResponse            `json:"roles"`
	Permissions []PermissionResponse      `json:"permissions"`
	Grants      []PermissionGrantResponse `json:"grants"`
}
