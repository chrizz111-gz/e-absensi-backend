package models

import "time"

// Session merepresentasikan satu sesi login per device. Logout = revoke baris
// ini; ganti/reset password = revoke seluruh baris milik user (revoked_at).
type Session struct {
	ID               int64      `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	UserID           int64      `gorm:"column:user_id;not null" json:"user_id"`
	RefreshTokenHash string     `gorm:"column:refresh_token_hash;size:255;not null" json:"-"`
	DeviceInfo       *string    `gorm:"column:device_info;size:255" json:"device_info,omitempty"`
	IPAddress        *string    `gorm:"column:ip_address;type:inet" json:"ip_address,omitempty"`
	UserAgent        *string    `gorm:"column:user_agent;size:500" json:"user_agent,omitempty"`
	ExpiredAt        time.Time  `gorm:"column:expired_at;not null" json:"expired_at"`
	RevokedAt        *time.Time `gorm:"column:revoked_at" json:"revoked_at,omitempty"`
	LastUsedAt       *time.Time `gorm:"column:last_used_at" json:"last_used_at,omitempty"`
	CreatedAt        time.Time  `gorm:"column:created_at;not null;autoCreateTime" json:"created_at"`
}

// TableName memetakan Session ke tabel "sessions".
func (Session) TableName() string {
	return "sessions"
}

// IsActive mengembalikan true bila sesi belum direvoke dan belum kedaluwarsa.
func (s *Session) IsActive() bool {
	return s.RevokedAt == nil && s.ExpiredAt.After(time.Now())
}
