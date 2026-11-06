package utils

import (
	"errors"
	"fmt"
	"reflect"
	"regexp"
	"strings"

	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/validator/v10"

	"e-absensi-backend/config"
)

var (
	nisnRegex     = regexp.MustCompile(`^[0-9]{10}$`)
	usernameRegex = regexp.MustCompile(`^[a-z0-9._]{3,30}$`)
)

// RegisterValidators mendaftarkan rule validasi kustom ke validator Gin.
// Wajib dipanggil sekali saat startup.
func RegisterValidators() {
	if v, ok := binding.Validator.Engine().(*validator.Validate); ok {
		v.RegisterTagNameFunc(jsonFieldName)
		_ = v.RegisterValidation("nisn", validateNISN)
		_ = v.RegisterValidation("otp", validateOTP)
		_ = v.RegisterValidation("strongpassword", validateStrongPassword)
		_ = v.RegisterValidation("username", validateUsername)
	}
}

// jsonFieldName memakai nama field pada tag JSON agar pesan error konsisten
// dengan kontrak API (misalnya "new_password", bukan "NewPassword").
func jsonFieldName(fld reflect.StructField) string {
	name := strings.SplitN(fld.Tag.Get("json"), ",", 2)[0]
	if name == "-" {
		return ""
	}
	if name == "" {
		return fld.Name
	}
	return name
}

// validateNISN memeriksa format NISN: tepat 10 digit angka.
func validateNISN(fl validator.FieldLevel) bool {
	return nisnRegex.MatchString(fl.Field().String())
}

// validateOTP memeriksa kode OTP: tepat OTPLength karakter alfanumerik.
// Input lowercase/spasi tepi tetap diterima lalu dinormalisasi oleh service.
func validateOTP(fl validator.FieldLevel) bool {
	return IsOTPCode(fl.Field().String(), configuredOTPLength())
}

// validateStrongPassword memeriksa seluruh kriteria password project.
func validateStrongPassword(fl validator.FieldLevel) bool {
	return IsStrongPassword(fl.Field().String())
}

// validateUsername memeriksa format username. Huruf besar diterima lalu
// dinormalisasi menjadi huruf kecil oleh service, sehingga user tidak perlu
// mengetik ulang.
func validateUsername(fl validator.FieldLevel) bool {
	return IsValidUsername(fl.Field().String())
}

// IsValidUsername memeriksa format username setelah trim dan lowercase.
func IsValidUsername(value string) bool {
	return usernameRegex.MatchString(strings.ToLower(strings.TrimSpace(value)))
}

// IsValidEmail memeriksa format email memakai validator yang sama dengan binding.
func IsValidEmail(value string) bool {
	return validator.New().Var(strings.TrimSpace(value), "email") == nil
}

// configuredOTPLength mengembalikan panjang OTP aktif. Fallback 6 dipakai saat
// config belum diinisialisasi (misalnya pada unit test).
func configuredOTPLength() int {
	if config.OTPLength > 0 {
		return config.OTPLength
	}
	return 6
}

// FormatValidationErrors mengubah error validator menjadi daftar FieldError
// yang menyebut nama field, kode mesin, dan pesan ramah dibaca.
func FormatValidationErrors(err error) []FieldError {
	var ve validator.ValidationErrors
	if errors.As(err, &ve) {
		out := make([]FieldError, 0, len(ve))
		for _, fe := range ve {
			out = append(out, toFieldError(fe))
		}
		return out
	}
	return []FieldError{{
		Field:   "request",
		Code:    "INVALID_REQUEST",
		Message: err.Error(),
	}}
}

// toFieldError memetakan satu error validator ke FieldError.
func toFieldError(fe validator.FieldError) FieldError {
	code := validationCode(fe)
	return FieldError{
		Field:   fe.Field(),
		Code:    code,
		Message: validationMessage(fe, code),
	}
}

// validationCode memetakan tag validator ke kode error yang stabil.
func validationCode(fe validator.FieldError) string {
	switch fe.Tag() {
	case "required":
		return FieldRequired
	case "email":
		return FieldInvalidEmail
	case "min":
		return FieldTooShort
	case "max":
		return FieldTooLong
	case "nisn":
		return FieldInvalidNISN
	case "username":
		return FieldInvalidUsername
	case "otp":
		return FieldInvalidOTP
	case "eqfield":
		if fe.Field() == "confirm_password" {
			return FieldPasswordMatch
		}
		return "FIELDS_MISMATCH"
	case "strongpassword":
		if password, ok := fe.Value().(string); ok {
			if code := PasswordPolicyViolation(password); code != PasswordPolicyOK {
				return code
			}
		}
		return "PASSWORD_TOO_WEAK"
	default:
		return strings.ToUpper(fe.Tag())
	}
}

// validationMessage memetakan tag validator ke pesan berbahasa Indonesia.
func validationMessage(fe validator.FieldError, code string) string {
	switch fe.Tag() {
	case "required":
		return fmt.Sprintf("%s wajib diisi", fe.Field())
	case "email":
		return fmt.Sprintf("%s harus berupa email yang valid", fe.Field())
	case "min":
		return fmt.Sprintf("%s minimal %s karakter", fe.Field(), fe.Param())
	case "max":
		return fmt.Sprintf("%s maksimal %s karakter", fe.Field(), fe.Param())
	case "nisn":
		return fmt.Sprintf("%s harus 10 digit angka", fe.Field())
	case "eqfield":
		if fe.Field() == "confirm_password" {
			return "Konfirmasi kata sandi tidak sesuai"
		}
		return fmt.Sprintf("%s tidak cocok", fe.Field())
	case "otp":
		return fmt.Sprintf("Kode OTP harus %d karakter (A-Z, 2-9)", configuredOTPLength())
	case "strongpassword":
		return PasswordPolicyMessage(code)
	case "username":
		return "Username 3-30 karakter, hanya huruf kecil, angka, titik, dan underscore"
	default:
		return fmt.Sprintf("%s tidak valid (%s)", fe.Field(), fe.Tag())
	}
}
