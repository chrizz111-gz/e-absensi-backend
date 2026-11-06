package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPasswordPolicyViolation(t *testing.T) {
	cases := []struct {
		name     string
		password string
		want     string
	}{
		{"valid", "Rahasia123!", PasswordPolicyOK},
		{"valid dengan simbol lain", "Sekolah#2026", PasswordPolicyOK},
		{"terlalu pendek", "Aa1!aaa", PasswordTooShort},
		{"terlalu panjang", "Aa1!" + repeat("a", 69), PasswordTooLong},
		{"tanpa huruf kecil", "RAHASIA123!", PasswordNeedsLower},
		{"tanpa huruf besar", "rahasia123!", PasswordNeedsUpper},
		{"tanpa angka", "Rahasia!!!", PasswordNeedsDigit},
		{"tanpa simbol", "Rahasia123", PasswordNeedsSymbol},
		{"mengandung spasi", "Rahasia 123!", PasswordContainsWhitespace},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, PasswordPolicyViolation(tc.password))
		})
	}
}

func TestIsStrongPassword(t *testing.T) {
	assert.True(t, IsStrongPassword("Rahasia123!"))
	assert.False(t, IsStrongPassword("aaaaaaaa"))
}

func TestPasswordPolicyMessage(t *testing.T) {
	assert.Equal(t, "Password minimal 8 karakter", PasswordPolicyMessage(PasswordTooShort))
	assert.Equal(t, "Password harus memuat simbol", PasswordPolicyMessage(PasswordNeedsSymbol))
	assert.NotEmpty(t, PasswordPolicyMessage("UNKNOWN"))
}

func repeat(s string, n int) string {
	out := ""
	for i := 0; i < n; i++ {
		out += s
	}
	return out
}
