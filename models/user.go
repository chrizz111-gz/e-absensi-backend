package models

import "time"

// User merepresentasikan satu akun login. Role tidak disimpan di sini melainkan
// di pivot user_roles (multi-role / union, tanpa role aktif per sesi).
//
// Catatan kolom:
//   - Email disimpan lowercase (ditegakkan trigger fn_users_normalize) dan unik
//     case-insensitive melalui index uq_users_email_lower.
//   - PasswordHash & GoogleID dapat dikosongkan, tetapi constraint
//     ck_users_password_or_google mengharuskan minimal salah satu terisi.
//   - ID memakai GENERATED ALWAYS AS IDENTITY sehingga tidak boleh diisi manual.
type User struct {
	ID                  int64      `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	Name                string     `gorm:"column:name;size:150;not null" json:"name"`
	Email               string     `gorm:"column:email;size:255;not null" json:"email"`
	Username            *string    `gorm:"column:username;size:50" json:"username,omitempty"`
	PasswordHash        *string    `gorm:"column:password_hash;size:255" json:"-"`
	NISN                *string    `gorm:"column:nisn;size:10" json:"nisn,omitempty"`
	GoogleID            *string    `gorm:"column:google_id;size:255" json:"-"`
	AvatarURL           *string    `gorm:"column:avatar_url;size:500" json:"avatar_url,omitempty"`
	IsActive            bool       `gorm:"column:is_active;not null;default:true" json:"is_active"`
	FailedLoginAttempts int16      `gorm:"column:failed_login_attempts;not null;default:0" json:"-"`
	FailedLoginAt       *time.Time `gorm:"column:failed_login_at" json:"-"`
	LockedUntil         *time.Time `gorm:"column:locked_until" json:"locked_until,omitempty"`
	PasswordChangedAt   *time.Time `gorm:"column:password_changed_at" json:"-"`
	CreatedAt           time.Time  `gorm:"column:created_at;not null;autoCreateTime" json:"created_at"`
	UpdatedAt           time.Time  `gorm:"column:updated_at;not null;autoUpdateTime" json:"updated_at"`

	// Roles adalah union seluruh role akun (diambil dari tabel pivot user_roles).
	Roles []Role `gorm:"many2many:user_roles;foreignKey:ID;joinForeignKey:user_id;References:ID;joinReferences:role_id" json:"roles,omitempty"`
}

// TableName memetakan User ke tabel "users".
func (User) TableName() string {
	return "users"
}

// IsLocked mengembalikan true bila akun sedang terkunci sementara.
func (u *User) IsLocked() bool {
	return u.LockedUntil != nil && u.LockedUntil.After(time.Now())
}

// RoleNames mengembalikan daftar nama role milik user.
func (u *User) RoleNames() []string {
	names := make([]string, 0, len(u.Roles))
	for _, r := range u.Roles {
		names = append(names, r.Name)
	}
	return names
}
