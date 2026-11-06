package utils

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateOTPIsAlphanumericWithRequestedLength(t *testing.T) {
	for _, length := range []int{4, 6, 8} {
		for i := 0; i < 20; i++ {
			code, err := GenerateOTP(length)
			require.NoError(t, err)
			assert.Len(t, code, length)
			assert.Truef(t, IsOTPCode(code, length), "kode %q harus memakai charset OTP", code)
			assert.Truef(t, hasOTPLetter(code), "kode %q harus memuat huruf", code)
			assert.Truef(t, hasOTPDigit(code), "kode %q harus memuat angka", code)
			assert.Falsef(t, strings.ContainsAny(code, "0O1IL"), "kode %q tidak boleh memuat karakter ambigu", code)
		}
	}
}

func TestGenerateOTPRejectsInvalidLength(t *testing.T) {
	_, err := GenerateOTP(0)
	assert.ErrorIs(t, err, ErrInvalidOTPLength)

	_, err = GenerateOTP(-1)
	assert.ErrorIs(t, err, ErrInvalidOTPLength)
}

func TestNormalizeOTP(t *testing.T) {
	assert.Equal(t, "A7K2Q9", NormalizeOTP(" a7k2q9 "))
	assert.Equal(t, "A7K2Q9", NormalizeOTP("A7K2Q9"))
}

func TestIsOTPCode(t *testing.T) {
	assert.True(t, IsOTPCode("A7K2Q9", 6))
	assert.True(t, IsOTPCode("a7k2q9", 6), "input lowercase dinormalisasi")
	assert.True(t, IsOTPCode(" A7K2Q9 ", 6), "spasi tepi dinormalisasi")

	assert.False(t, IsOTPCode("A7K2Q", 6))
	assert.False(t, IsOTPCode("A7K2Q99", 6))
	assert.False(t, IsOTPCode("A7K2Q0", 6), "angka 0 tidak dipakai")
	assert.False(t, IsOTPCode("A7K2QO", 6), "huruf O tidak dipakai")
	assert.False(t, IsOTPCode("A7K2Q1", 6), "angka 1 tidak dipakai")
	assert.False(t, IsOTPCode("A7K2QI", 6), "huruf I tidak dipakai")
	assert.False(t, IsOTPCode("A7K2QL", 6), "huruf L tidak dipakai")
	assert.False(t, IsOTPCode("A7K2Q!", 6))
	assert.False(t, IsOTPCode("", 6))
}
