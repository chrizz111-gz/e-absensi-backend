package dto

import "time"

// VerifyOTPRequest adalah payload cek OTP untuk alur ganti password
// (user sudah login; identitas diambil dari access token).
type VerifyOTPRequest struct {
	Code string `json:"code" binding:"required,otp"`
}

// VerifyResetPasswordOTPRequest adalah payload cek OTP untuk alur lupa
// password. Email dipakai untuk mencari akun.
type VerifyResetPasswordOTPRequest struct {
	Email string `json:"email" binding:"required,email"`
	Code  string `json:"code" binding:"required,otp"`
}

// SetPasswordRequest adalah payload penyimpanan password baru setelah OTP
// diverifikasi. ResetToken diterbitkan oleh endpoint verify-otp.
type SetPasswordRequest struct {
	ResetToken      string `json:"reset_token" binding:"required"`
	NewPassword     string `json:"new_password" binding:"required,strongpassword"`
	ConfirmPassword string `json:"confirm_password" binding:"required,eqfield=NewPassword"`
}

// ForgotPasswordRequest adalah payload permintaan/resend OTP reset password (F1.5/F1.6).
type ForgotPasswordRequest struct {
	Email string `json:"email" binding:"required,email"`
}

// GoogleLoginRequest adalah payload login/daftar via Google Sign-In (F1.3).
type GoogleLoginRequest struct {
	IDToken    string `json:"id_token" binding:"required"`
	DeviceInfo string `json:"device_info" binding:"omitempty,max=255"`
}

// OTPRequestedResponse dikembalikan saat OTP berhasil dibuat/dikirim ulang.
type OTPRequestedResponse struct {
	AttemptsRemaining  int       `json:"attempts_remaining"`
	ExpiresAt          time.Time `json:"expires_at"`
	ResendAfterSeconds int       `json:"resend_after_seconds"`
}

// OTPVerifiedResponse dikembalikan setelah kode OTP benar. ResetToken dipakai
// pada endpoint simpan password baru.
type OTPVerifiedResponse struct {
	ResetToken        string    `json:"reset_token"`
	ExpiresAt         time.Time `json:"expires_at"`
	AttemptsRemaining int       `json:"attempts_remaining"`
}

// OTPAttemptErrorData adalah data tambahan pada error OTP agar frontend bisa
// menampilkan sisa percobaan dan waktu kedaluwarsa.
type OTPAttemptErrorData struct {
	AttemptsRemaining int        `json:"attempts_remaining"`
	ExpiresAt         *time.Time `json:"expires_at,omitempty"`
}

// RetryAfterData adalah data tambahan pada error cooldown/resend.
type RetryAfterData struct {
	RetryAfterSeconds int `json:"retry_after_seconds"`
}
