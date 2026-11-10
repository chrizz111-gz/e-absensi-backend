package dto

import "time"

// UserListFilter adalah parameter tervalidasi untuk GET /users.
type UserListFilter struct {
	Page            int
	Limit           int
	Q               string
	Roles           []string
	Status          string
	IncludeArchived bool
}

// PaginationResponse adalah metadata halaman daftar user.
type PaginationResponse struct {
	Page       int   `json:"page"`
	Limit      int   `json:"limit"`
	Total      int64 `json:"total"`
	TotalPages int   `json:"total_pages"`
}

// UserRoleSummary adalah role ringkas yang aman ditampilkan di halaman Admin.
type UserRoleSummary struct {
	ID           int16   `json:"id"`
	Name         string  `json:"name"`
	Label        string  `json:"label"`
	Category     string  `json:"category"`
	IsAssignable bool    `json:"assignable"`
	Description  *string `json:"description,omitempty"`
}

// UserAdminResponse adalah representasi user untuk endpoint Admin. Tidak pernah
// memuat password_hash maupun google_id.
type UserAdminResponse struct {
	ID             int64             `json:"id"`
	Name           string            `json:"name"`
	Email          string            `json:"email"`
	NISN           *string           `json:"nisn,omitempty"`
	AvatarURL      *string           `json:"avatar_url,omitempty"`
	IsActive       bool              `json:"is_active"`
	InactiveReason *string           `json:"inactive_reason,omitempty"`
	DeactivatedAt  *time.Time        `json:"deactivated_at,omitempty"`
	ArchivedAt     *time.Time        `json:"archived_at,omitempty"`
	CreatedAt      time.Time         `json:"created_at"`
	UpdatedAt      time.Time         `json:"updated_at"`
	Roles          []UserRoleSummary `json:"roles"`
}

// UserListResponse adalah payload GET /users.
type UserListResponse struct {
	Items []UserAdminResponse `json:"items"`
	Meta  PaginationResponse  `json:"meta"`
}

// CreateUserRequest adalah payload POST /users. Akun staf dibuat tanpa
// password/Google; kredensial diatur lewat alur akun masing-masing. Hanya
// name, email, dan roles yang diterima (whitelist PRD).
type CreateUserRequest struct {
	Name  string   `json:"name" binding:"required,min=2,max=150"`
	Email string   `json:"email" binding:"required"`
	Roles []string `json:"roles"`
}

// UpdateUserStatusRequest adalah payload PATCH /users/{id}/status. IsActive
// memakai pointer agar false tetap terdeteksi sebagai nilai yang dikirim.
type UpdateUserStatusRequest struct {
	IsActive       *bool   `json:"is_active" binding:"required"`
	InactiveReason *string `json:"inactive_reason"`
}

// UpdateUserRolesRequest adalah payload PUT /users/{id}/roles. Array kosong
// berarti mencabut seluruh role assignable; role terkunci dipertahankan dan
// akun wajib tetap memiliki minimal satu role setelah penggantian.
type UpdateUserRolesRequest struct {
	Roles []string `json:"roles"`
}

// UpdateUserRequest adalah payload PATCH /users/{id}. Field yang tidak dikirim
// tidak diubah. UpdatedAt opsional; bila diisi, nilainya harus sama dengan
// updated_at terakhir dari GET detail untuk optimistic locking.
type UpdateUserRequest struct {
	Name      *string    `json:"name"`
	Email     *string    `json:"email"`
	UpdatedAt *time.Time `json:"updated_at"`
}
