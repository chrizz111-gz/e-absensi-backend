package utils

import (
	"testing"

	"github.com/go-playground/validator/v10"
	"github.com/stretchr/testify/assert"
)

func newTestValidator() *validator.Validate {
	v := validator.New()
	v.RegisterTagNameFunc(jsonFieldName)
	_ = v.RegisterValidation("nisn", validateNISN)
	_ = v.RegisterValidation("otp", validateOTP)
	_ = v.RegisterValidation("strongpassword", validateStrongPassword)
	_ = v.RegisterValidation("username", validateUsername)
	return v
}

func TestValidateNISN(t *testing.T) {
	v := newTestValidator()

	type sample struct {
		NISN string `validate:"nisn"`
	}

	assert.NoError(t, v.Struct(sample{NISN: "1234567890"}), "10 digit angka harus valid")
	assert.Error(t, v.Struct(sample{NISN: "123"}), "kurang dari 10 digit harus gagal")
	assert.Error(t, v.Struct(sample{NISN: "12345678901"}), "lebih dari 10 digit harus gagal")
	assert.Error(t, v.Struct(sample{NISN: "12345abcde"}), "non-angka harus gagal")
	assert.Error(t, v.Struct(sample{NISN: ""}), "kosong harus gagal")
}

func TestValidateOTP(t *testing.T) {
	v := newTestValidator()

	type sample struct {
		Code string `validate:"otp"`
	}

	assert.NoError(t, v.Struct(sample{Code: "A7K2Q9"}), "6 karakter alfanumerik harus valid")
	assert.NoError(t, v.Struct(sample{Code: "a7k2q9"}), "lowercase dinormalisasi")
	assert.Error(t, v.Struct(sample{Code: "A7K2Q"}), "kurang dari 6 karakter harus gagal")
	assert.Error(t, v.Struct(sample{Code: "A7K2Q99"}), "lebih dari 6 karakter harus gagal")
	assert.Error(t, v.Struct(sample{Code: "A7K2Q!"}), "simbol harus gagal")
	assert.Error(t, v.Struct(sample{Code: "A7K2Q0"}), "angka 0 tidak dipakai")
	assert.Error(t, v.Struct(sample{Code: "A7K2QO"}), "huruf O tidak dipakai")
}

func TestValidateUsername(t *testing.T) {
	v := newTestValidator()

	type sample struct {
		Username string `validate:"username"`
	}

	assert.NoError(t, v.Struct(sample{Username: "budi.santoso_01"}))
	assert.NoError(t, v.Struct(sample{Username: "Budi"}), "huruf besar dinormalisasi oleh service")
	assert.Error(t, v.Struct(sample{Username: "bu"}), "kurang dari 3 karakter harus gagal")
	assert.Error(t, v.Struct(sample{Username: "budi santoso"}), "spasi harus gagal")
	assert.Error(t, v.Struct(sample{Username: "budi@example"}), "simbol selain . dan _ harus gagal")
}

func TestValidateStrongPassword(t *testing.T) {
	v := newTestValidator()

	type sample struct {
		Password string `validate:"strongpassword"`
	}

	assert.NoError(t, v.Struct(sample{Password: "Rahasia123!"}))
	assert.Error(t, v.Struct(sample{Password: "aaaaaaaa"}))
}

func TestValidateConfirmPassword(t *testing.T) {
	v := newTestValidator()

	type sample struct {
		Password        string `json:"password" validate:"required,strongpassword"`
		ConfirmPassword string `json:"confirm_password" validate:"required,eqfield=Password"`
	}

	assert.NoError(t, v.Struct(sample{
		Password:        "Rahasia123!",
		ConfirmPassword: "Rahasia123!",
	}))

	err := v.Struct(sample{
		Password:        "Rahasia123!",
		ConfirmPassword: "Rahasia123?",
	})
	assert.Error(t, err)

	fields := FormatValidationErrors(err)
	assert.Len(t, fields, 1)
	assert.Equal(t, "confirm_password", fields[0].Field)
	assert.Equal(t, FieldPasswordMatch, fields[0].Code)
}

func TestFormatValidationErrors(t *testing.T) {
	v := newTestValidator()

	type sample struct {
		Email    string `json:"email" validate:"required,email"`
		NISN     string `json:"nisn" validate:"nisn"`
		Password string `json:"password" validate:"strongpassword"`
	}

	err := v.Struct(sample{Email: "bukan-email", NISN: "12", Password: "aaaaaaaa"})
	assert.Error(t, err)

	fields := FormatValidationErrors(err)
	assert.Len(t, fields, 3)

	byField := make(map[string]FieldError, len(fields))
	for _, field := range fields {
		byField[field.Field] = field
	}

	assert.Equal(t, FieldInvalidEmail, byField["email"].Code)
	assert.Equal(t, FieldInvalidNISN, byField["nisn"].Code)
	assert.Equal(t, PasswordNeedsUpper, byField["password"].Code)
	assert.NotEmpty(t, byField["password"].Message)
}
