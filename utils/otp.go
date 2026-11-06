package utils

import (
	"crypto/rand"
	"errors"
	"math/big"
	"strings"
)

// otpAlphabet adalah charset OTP: A-Z tanpa I/L/O, ditambah angka 2-9.
// Karakter 0/O/1/I/L sengaja dihilangkan agar tidak ambigu saat diketik.
const otpAlphabet = "ABCDEFGHJKMNPQRSTUVWXYZ23456789"

// ErrInvalidOTPLength is returned when a non-positive OTP length is requested.
var ErrInvalidOTPLength = errors.New("panjang OTP harus lebih dari 0")

// GenerateOTP generates a cryptographically secure alphanumeric OTP with
// exactly length characters. Untuk length >= 2, kode dijamin memuat minimal
// satu huruf dan satu angka.
func GenerateOTP(length int) (string, error) {
	if length <= 0 {
		return "", ErrInvalidOTPLength
	}

	for {
		code, err := randomOTP(length)
		if err != nil {
			return "", err
		}
		if length < 2 || (hasOTPLetter(code) && hasOTPDigit(code)) {
			return code, nil
		}
	}
}

// randomOTP mengambil karakter dari otpAlphabet menggunakan crypto/rand.
func randomOTP(length int) (string, error) {
	limit := big.NewInt(int64(len(otpAlphabet)))
	var b strings.Builder
	b.Grow(length)

	for i := 0; i < length; i++ {
		n, err := rand.Int(rand.Reader, limit)
		if err != nil {
			return "", err
		}
		b.WriteByte(otpAlphabet[int(n.Int64())])
	}
	return b.String(), nil
}

// NormalizeOTP menyeragamkan input user: spasi tepi dibuang dan huruf diubah
// ke uppercase, sehingga input "a7k2q9" tetap cocok dengan kode "A7K2Q9".
func NormalizeOTP(code string) string {
	return strings.ToUpper(strings.TrimSpace(code))
}

// IsOTPCode reports whether code (setelah dinormalisasi) tepat length karakter
// dan hanya memakai charset OTP yang diizinkan.
func IsOTPCode(code string, length int) bool {
	code = NormalizeOTP(code)
	if length <= 0 || len(code) != length {
		return false
	}
	for i := 0; i < len(code); i++ {
		if strings.IndexByte(otpAlphabet, code[i]) < 0 {
			return false
		}
	}
	return true
}

// hasOTPLetter melaporkan apakah s mengandung minimal satu huruf A-Z.
func hasOTPLetter(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] >= 'A' && s[i] <= 'Z' {
			return true
		}
	}
	return false
}

// hasOTPDigit melaporkan apakah s mengandung minimal satu angka 2-9.
func hasOTPDigit(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] >= '2' && s[i] <= '9' {
			return true
		}
	}
	return false
}
