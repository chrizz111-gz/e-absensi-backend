package models

import "time"

// Purpose OTP yang diizinkan (selaras constraint ck_otp_purpose).
const (
	PurposeChangePassword = "change_password"
	PurposeResetPassword  = "reset_password"
)

// OTPCode menyimpan OTP (dalam bentuk hash) untuk Ganti/Reset Password.
// Isolasi per purpose: OTP change_password tidak valid untuk reset_password.
type OTPCode struct {
	ID           int64     `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	UserID       int64     `gorm:"column:user_id;not null" json:"user_id"`
	CodeHash     string    `gorm:"column:code_hash;size:255;not null" json:"-"`
	Purpose      string    `gorm:"column:purpose;size:30;not null" json:"purpose"`
	ExpiredAt    time.Time `gorm:"column:expired_at;not null" json:"expired_at"`
	LastSentAt   time.Time `gorm:"column:last_sent_at;not null;autoCreateTime" json:"last_sent_at"`
	IsUsed       bool      `gorm:"column:is_used;not null;default:false" json:"is_used"`
	AttemptCount int16     `gorm:"column:attempt_count;not null;default:0" json:"attempt_count"`
	CreatedAt    time.Time `gorm:"column:created_at;not null;autoCreateTime" json:"created_at"`
}

// TableName memetakan OTPCode ke tabel "otp_codes".
func (OTPCode) TableName() string {
	return "otp_codes"
}
