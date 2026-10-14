package utils

import (
	"testing"

	"github.com/go-playground/validator/v10"
	"github.com/stretchr/testify/assert"
)

func newTestValidator() *validator.Validate {
	v := validator.New()
	_ = v.RegisterValidation("nisn", validateNISN)
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

func TestFormatValidationErrors(t *testing.T) {
	v := newTestValidator()

	type sample struct {
		Email string `validate:"required,email"`
		NISN  string `validate:"nisn"`
	}

	err := v.Struct(sample{Email: "bukan-email", NISN: "12"})
	assert.Error(t, err)

	msgs := FormatValidationErrors(err)
	assert.Len(t, msgs, 2)
}
