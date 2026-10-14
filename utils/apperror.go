package utils

import "net/http"

// AppError adalah error domain yang membawa status HTTP dan kode response.
// Service mengembalikan *AppError; middleware ErrorHandler menerjemahkannya
// menjadi envelope standar.
type AppError struct {
	HTTPStatus int
	Code       string
	Message    string
	Errors     []string
}

// Error mengimplementasikan interface error.
func (e *AppError) Error() string {
	return e.Message
}

// NewAppError membuat AppError baru.
func NewAppError(status int, code, message string) *AppError {
	return &AppError{HTTPStatus: status, Code: code, Message: message}
}

// WithMessage mengembalikan salinan AppError dengan pesan berbeda, sehingga
// error sentinel di bawah tetap aman dipakai berulang (tidak termutasi).
func (e *AppError) WithMessage(message string) *AppError {
	clone := *e
	clone.Message = message
	return &clone
}

// NewValidationError membuat AppError validasi (422) beserta detail field.
func NewValidationError(errs []string) *AppError {
	return &AppError{
		HTTPStatus: http.StatusUnprocessableEntity,
		Code:       CodeValidation,
		Message:    "Validasi data gagal",
		Errors:     errs,
	}
}

// Error sentinel domain (dipakai berulang via WithMessage).
var (
	ErrInvalidCredentials = NewAppError(http.StatusUnauthorized, CodeInvalidCredentials, "Email/username atau password salah")
	ErrAccountInactive    = NewAppError(http.StatusForbidden, CodeAccountInactive, "Akun tidak aktif, hubungi admin")
	ErrAccountLocked      = NewAppError(http.StatusTooManyRequests, CodeAccountLocked, "Akun terkunci sementara karena percobaan login berlebih")
	ErrUnauthorized       = NewAppError(http.StatusUnauthorized, CodeUnauthorized, "Autentikasi diperlukan atau token tidak valid")
	ErrTokenExpired       = NewAppError(http.StatusUnauthorized, CodeTokenExpired, "Token telah kedaluwarsa")
	ErrSessionRevoked     = NewAppError(http.StatusUnauthorized, CodeSessionRevoked, "Sesi tidak valid atau telah berakhir")
	ErrForbidden          = NewAppError(http.StatusForbidden, CodeForbidden, "Anda tidak memiliki hak akses")
	ErrNotFound           = NewAppError(http.StatusNotFound, CodeNotFound, "Data tidak ditemukan")
	ErrConflict           = NewAppError(http.StatusConflict, CodeConflict, "Data sudah terdaftar")
	ErrInternal           = NewAppError(http.StatusInternalServerError, CodeInternal, "Terjadi kesalahan pada server")

	ErrOTPInvalid       = NewAppError(http.StatusBadRequest, CodeOTPInvalid, "Kode OTP salah")
	ErrOTPExpired       = NewAppError(http.StatusBadRequest, CodeOTPExpired, "Kode OTP sudah kedaluwarsa, silakan minta kode baru")
	ErrOTPTooMany       = NewAppError(http.StatusTooManyRequests, CodeOTPTooMany, "Terlalu banyak percobaan OTP, silakan minta kode baru")
	ErrOTPResendTooSoon = NewAppError(http.StatusTooManyRequests, CodeOTPResendTooSoon, "Mohon tunggu sebelum meminta kode baru")
	ErrOTPResendLimit   = NewAppError(http.StatusTooManyRequests, CodeOTPResendLimit, "Batas permintaan OTP per jam tercapai")

	ErrRateLimited           = NewAppError(http.StatusTooManyRequests, CodeRateLimited, "Terlalu banyak permintaan, silakan coba lagi nanti")
	ErrGoogleDisabled        = NewAppError(http.StatusServiceUnavailable, CodeGoogleDisabled, "Login Google belum dikonfigurasi")
	ErrGoogleTokenInvalid    = NewAppError(http.StatusUnauthorized, CodeGoogleTokenInvalid, "Token Google tidak valid")
	ErrGoogleEmailUnverified = NewAppError(http.StatusUnauthorized, CodeGoogleEmailUnverified, "Email Google belum terverifikasi")
	ErrAccountNotFound       = NewAppError(http.StatusNotFound, CodeAccountNotFound, "Akun belum terdaftar, silakan registrasi terlebih dahulu")
)
