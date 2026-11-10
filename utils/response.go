package utils

import "github.com/gin-gonic/gin"

// Kode standar response envelope (dipakai React & Flutter untuk branching).
const (
	CodeSuccess      = "SUCCESS"
	CodeCreated      = "CREATED"
	CodeUserList     = "USER_LIST"
	CodeValidation   = "VALIDATION_ERROR"
	CodeUnauthorized = "UNAUTHORIZED"
	CodeForbidden    = "FORBIDDEN"
	CodeConflict     = "CONFLICT"
	CodeNotFound     = "NOT_FOUND"
	CodeInternal     = "INTERNAL_ERROR"

	CodeStaleData              = "STALE_DATA"
	CodeLastAdminProtected     = "LAST_ADMIN_PROTECTED"
	CodeSelfActionForbidden    = "SELF_ACTION_FORBIDDEN"
	CodeAccountHasHistory      = "ACCOUNT_HAS_HISTORY"
	CodeAccountNotFound        = "ACCOUNT_NOT_FOUND"
	CodeEmailAlreadyExists     = "EMAIL_ALREADY_EXISTS"
	CodeAccountAlreadyInactive = "ACCOUNT_ALREADY_INACTIVE"
	CodeAccountAlreadyActive   = "ACCOUNT_ALREADY_ACTIVE"
	CodeRoleNotAssignable      = "ROLE_NOT_ASSIGNABLE"
	CodeInvalidRoleCombination = "INVALID_ROLE_COMBINATION"
	CodeAccountTypeNotEditable = "ACCOUNT_TYPE_NOT_EDITABLE"

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
	CodeOTPVerified      = "OTP_VERIFIED"
	CodePasswordChanged  = "AUTH_PASSWORD_CHANGED"
	CodePasswordSame     = "AUTH_PASSWORD_SAME"
	CodeOTPInvalid       = "OTP_INVALID"
	CodeOTPExpired       = "OTP_EXPIRED"
	CodeOTPTooMany       = "OTP_TOO_MANY_ATTEMPTS"
	CodeOTPResendTooSoon = "OTP_RESEND_TOO_SOON"
	CodeOTPResendLimit   = "OTP_RESEND_LIMIT"

	CodeResetTokenInvalid = "AUTH_RESET_TOKEN_INVALID"
	CodeResetTokenExpired = "AUTH_RESET_TOKEN_EXPIRED"

	CodeRateLimited           = "RATE_LIMITED"
	CodeGoogleLoginSuccess    = "AUTH_GOOGLE_SUCCESS"
	CodeGoogleDisabled        = "AUTH_GOOGLE_DISABLED"
	CodeGoogleTokenInvalid    = "AUTH_GOOGLE_TOKEN_INVALID"
	CodeGoogleEmailUnverified = "AUTH_GOOGLE_EMAIL_UNVERIFIED"
	CodeAuthAccountNotFound   = "AUTH_ACCOUNT_NOT_FOUND"
)

// Kode detail field (dipakai pada errors[].code).
const (
	FieldRequired               = "FIELD_REQUIRED"
	FieldInvalidEmail           = "INVALID_EMAIL"
	FieldTooShort               = "TOO_SHORT"
	FieldTooLong                = "TOO_LONG"
	FieldInvalidNISN            = "INVALID_NISN"
	FieldInvalidUsername        = "INVALID_USERNAME"
	FieldInvalidOTP             = "INVALID_OTP_FORMAT"
	FieldPasswordMatch          = "PASSWORD_MISMATCH"
	FieldEmailTaken             = "EMAIL_TAKEN"
	FieldEmailAlreadyExists     = "EMAIL_ALREADY_EXISTS"
	FieldNISNTaken              = "NISN_TAKEN"
	FieldUsernameTaken          = "USERNAME_TAKEN"
	FieldInvalidID              = "INVALID_ID"
	FieldInvalidStatus          = "INVALID_STATUS"
	FieldInvalidPage            = "INVALID_PAGE"
	FieldInvalidLimit           = "INVALID_LIMIT"
	FieldInvalidReason          = "INVALID_INACTIVE_REASON"
	FieldRoleNotFound           = "ROLE_NOT_FOUND"
	FieldRoleNotAssignable      = "ROLE_NOT_ASSIGNABLE"
	FieldInvalidRoleCombination = "INVALID_ROLE_COMBINATION"
	FieldAccountTypeNotEditable = "ACCOUNT_TYPE_NOT_EDITABLE"
	FieldInvalidBoolean         = "INVALID_BOOLEAN"
)

// APIResponse adalah envelope standar seluruh endpoint.
type APIResponse struct {
	Success bool         `json:"success"`
	Code    string       `json:"code"`
	Message string       `json:"message"`
	Data    interface{}  `json:"data,omitempty"`
	Errors  []FieldError `json:"errors,omitempty"`
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

// ErrorResponseWithDetails mengirim response gagal beserta detail field dan
// data tambahan (misalnya sisa percobaan OTP).
func ErrorResponseWithDetails(c *gin.Context, status int, code, message string, errs []FieldError, data interface{}) {
	c.JSON(status, APIResponse{
		Success: false,
		Code:    code,
		Message: message,
		Data:    data,
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
