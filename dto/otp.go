package dto

// ChangePasswordRequest adalah payload verifikasi OTP + ganti password (F1.4).
type ChangePasswordRequest struct {
	Code        string `json:"code" binding:"required,len=8"`
	NewPassword string `json:"new_password" binding:"required,min=8,max=72"`
}

// ForgotPasswordRequest adalah payload permintaan/resend OTP reset password (F1.5/F1.6).
type ForgotPasswordRequest struct {
	Email string `json:"email" binding:"required,email"`
}

// ResetPasswordRequest adalah payload verifikasi OTP + reset password (F1.5).
type ResetPasswordRequest struct {
	Email       string `json:"email" binding:"required,email"`
	Code        string `json:"code" binding:"required,len=8"`
	NewPassword string `json:"new_password" binding:"required,min=8,max=72"`
}

// GoogleLoginRequest adalah payload login/daftar via Google Sign-In (F1.3).
type GoogleLoginRequest struct {
	IDToken    string `json:"id_token" binding:"required"`
	DeviceInfo string `json:"device_info" binding:"omitempty,max=255"`
}
