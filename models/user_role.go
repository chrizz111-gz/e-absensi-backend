package models

import "time"

// UserRole adalah tabel pivot multi-role. UNIQUE(user_id, role_id) mencegah role
// ganda; FK role_id ON DELETE RESTRICT menjaga master role tetap utuh.
type UserRole struct {
	ID        int64     `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	UserID    int64     `gorm:"column:user_id;not null" json:"user_id"`
	RoleID    int16     `gorm:"column:role_id;not null" json:"role_id"`
	CreatedAt time.Time `gorm:"column:created_at;not null;autoCreateTime" json:"created_at"`

	Role Role `gorm:"foreignKey:RoleID;references:ID" json:"role,omitempty"`
	User User `gorm:"foreignKey:UserID;references:ID" json:"-"`
}

// TableName memetakan UserRole ke tabel "user_roles".
func (UserRole) TableName() string {
	return "user_roles"
}
