package utils

import (
	"errors"
	"fmt"
	"regexp"

	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/validator/v10"
)

var nisnRegex = regexp.MustCompile(`^[0-9]{10}$`)

// RegisterValidators mendaftarkan rule validasi kustom ke validator Gin.
// Wajib dipanggil sekali saat startup.
func RegisterValidators() {
	if v, ok := binding.Validator.Engine().(*validator.Validate); ok {
		_ = v.RegisterValidation("nisn", validateNISN)
	}
}

// validateNISN memeriksa format NISN: tepat 10 digit angka.
func validateNISN(fl validator.FieldLevel) bool {
	return nisnRegex.MatchString(fl.Field().String())
}

// FormatValidationErrors mengubah error validator menjadi daftar pesan yang
// ramah dibaca untuk bagian "errors" pada envelope.
func FormatValidationErrors(err error) []string {
	var ve validator.ValidationErrors
	if errors.As(err, &ve) {
		out := make([]string, 0, len(ve))
		for _, fe := range ve {
			out = append(out, validationMessage(fe))
		}
		return out
	}
	return []string{err.Error()}
}

// validationMessage memetakan tag validator ke pesan berbahasa Indonesia.
func validationMessage(fe validator.FieldError) string {
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
		return fmt.Sprintf("%s tidak cocok", fe.Field())
	default:
		return fmt.Sprintf("%s tidak valid (%s)", fe.Field(), fe.Tag())
	}
}
