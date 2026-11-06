package dto

import "time"

// UserListFilter adalah parameter tervalidasi untuk GET /users.
type UserListFilter struct {
	Page   int
	Limit  int
	Q      string
	Roles  []string
	Status string
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
	IsAssignable bool    `json:"is_assignable"`
	Description  *string `json:"description,omitempty"`
}

// UserAdminResponse adalah representasi user untuk endpoint Admin. Tidak pernah
// memuat password_hash maupun google_id.
type UserAdminResponse struct {
	ID             int64             `json:"id"`
	Name           string            `json:"name"`
	Email          string            `json:"email"`
	Username       *string           `json:"username,omitempty"`
	NISN           *string           `json:"nisn,omitempty"`
	AvatarURL      *string           `json:"avatar_url,omitempty"`
	IsActive       bool              `json:"is_active"`
	InactiveReason *string           `json:"inactive_reason,omitempty"`
	DeactivatedAt  *time.Time        `json:"deactivated_at,omitempty"`
	CreatedAt      time.Time         `json:"created_at"`
	UpdatedAt      time.Time         `json:"updated_at"`
	Roles          []UserRoleSummary `json:"roles"`
}

// UserListResponse adalah payload GET /users.
type UserListResponse struct {
	Users      []UserAdminResponse `json:"users"`
	Pagination PaginationResponse  `json:"pagination"`
}

// CreateUserRequest adalah payload POST /users. Akun staf dibuat tanpa
// password/Google; kredensial diatur lewat alur akun masing-masing.
type CreateUserRequest struct {
	Name     string   `json:"name" binding:"required,min=2,max=150"`
	Email    string   `json:"email" binding:"required,email,max=255"`
	Username string   `json:"username" binding:"omitempty,username"`
	Roles    []string `json:"roles" binding:"required,min=1,dive,required"`
}

// UpdateUserStatusRequest adalah payload PATCH /users/{id}/status. IsActive
// memakai pointer agar false tetap terdeteksi sebagai nilai yang dikirim.
type UpdateUserStatusRequest struct {
	IsActive       *bool   `json:"is_active" binding:"required"`
	InactiveReason *string `json:"inactive_reason"`
}

// UpdateUserRolesRequest adalah payload PUT /users/{id}/roles. Array kosong
// sah dan berarti mencabut seluruh role yang assignable; role terkunci tetap
// dipertahankan oleh service.
type UpdateUserRolesRequest struct {
	Roles []string `json:"roles"`
}

// UpdateUserRequest adalah payload PATCH /users/{id}. Field yang tidak dikirim
// tidak diubah. UpdatedAt wajib berisi nilai terakhir dari GET detail untuk
// optimistic locking.
type UpdateUserRequest struct {
	Name      *string    `json:"name"`
	Email     *string    `json:"email"`
	Username  *string    `json:"username"`
	UpdatedAt *time.Time `json:"updated_at" binding:"required"`
}
