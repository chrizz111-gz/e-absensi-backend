package utils

import "github.com/gin-gonic/gin"

// Kode standar response envelope (dipakai React & Flutter untuk branching).
const (
	CodeSuccess      = "SUCCESS"
	CodeCreated      = "CREATED"
	CodeValidation   = "VALIDATION_ERROR"
	CodeUnauthorized = "AUTH_UNAUTHORIZED"
	CodeForbidden    = "FORBIDDEN"
	CodeConflict     = "CONFLICT"
	CodeNotFound     = "NOT_FOUND"
	CodeInternal     = "INTERNAL_ERROR"

	CodeRegisterSuccess = "AUTH_REGISTER_SUCCESS"
	CodeLoginSuccess    = "AUTH_LOGIN_SUCCESS"
	CodeLogoutSuccess   = "AUTH_LOGOUT_SUCCESS"
	CodeRefreshSuccess  = "AUTH_REFRESH_SUCCESS"

	CodeInvalidCredentials = "AUTH_INVALID_CREDENTIALS"
	CodeAccountLocked      = "AUTH_ACCOUNT_LOCKED"
	CodeAccountInactive    = "AUTH_ACCOUNT_INACTIVE"
	CodeTokenExpired       = "AUTH_TOKEN_EXPIRED"
	CodeSessionRevoked     = "AUTH_SESSION_REVOKED"

	CodeOTPRequested     = "OTP_REQUESTED"
	CodePasswordChanged  = "AUTH_PASSWORD_CHANGED"
	CodeOTPInvalid       = "OTP_INVALID"
	CodeOTPExpired       = "OTP_EXPIRED"
	CodeOTPTooMany       = "OTP_TOO_MANY_ATTEMPTS"
	CodeOTPResendTooSoon = "OTP_RESEND_TOO_SOON"
	CodeOTPResendLimit   = "OTP_RESEND_LIMIT"

	CodeRateLimited           = "RATE_LIMITED"
	CodeGoogleLoginSuccess    = "AUTH_GOOGLE_SUCCESS"
	CodeGoogleDisabled        = "AUTH_GOOGLE_DISABLED"
	CodeGoogleTokenInvalid    = "AUTH_GOOGLE_TOKEN_INVALID"
	CodeGoogleEmailUnverified = "AUTH_GOOGLE_EMAIL_UNVERIFIED"
	CodeAccountNotFound       = "AUTH_ACCOUNT_NOT_FOUND"
)

// APIResponse adalah envelope standar seluruh endpoint.
type APIResponse struct {
	Success bool        `json:"success"`
	Code    string      `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
	Errors  []string    `json:"errors,omitempty"`
}

// SuccessResponse mengirim response sukses dengan data opsional.
func SuccessResponse(c *gin.Context, status int, code, message string, data interface{}) {
	c.JSON(status, APIResponse{
		Success: true,
		Code:    code,
		Message: message,
		Data:    data,
	})
}

// ErrorResponse mengirim response gagal tanpa detail field.
func ErrorResponse(c *gin.Context, status int, code, message string) {
	c.JSON(status, APIResponse{
		Success: false,
		Code:    code,
		Message: message,
	})
}

// ErrorResponseWithDetails mengirim response gagal beserta daftar error
// (umumnya untuk kegagalan validasi).
func ErrorResponseWithDetails(c *gin.Context, status int, code, message string, errs []string) {
	c.JSON(status, APIResponse{
		Success: false,
		Code:    code,
		Message: message,
		Errors:  errs,
	})
}

// AbortWithError mengirim response gagal lalu menghentikan rantai handler
// (dipakai middleware).
func AbortWithError(c *gin.Context, status int, code, message string) {
	c.AbortWithStatusJSON(status, APIResponse{
		Success: false,
		Code:    code,
		Message: message,
	})
}
