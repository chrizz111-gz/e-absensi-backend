package utils

import (
	"crypto/rand"
	"errors"
	"math/big"
	"strings"
)

const (
	// OTPLength is the fixed number of characters for every generated OTP.
	OTPLength = 8

	// otpCharset contains uppercase letters and digits only, excluding the
	// visually ambiguous characters I, O, 0 and 1 to reduce input mistakes
	// when the code is typed manually from an email.
	otpCharset = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
)

// ErrInvalidOTPLength is returned when a non-positive OTP length is requested.
var ErrInvalidOTPLength = errors.New("panjang OTP harus lebih dari 0")

// GenerateOTP generates a cryptographically secure OTP consisting of
// OTPLength (8) uppercase characters and digits.
func GenerateOTP() (string, error) {
	return GenerateOTPWithLength(OTPLength)
}

// GenerateOTPWithLength generates a cryptographically secure OTP of the
// requested length using crypto/rand, making the output suitable for
// security-sensitive flows such as password reset.
func GenerateOTPWithLength(length int) (string, error) {
	if length <= 0 {
		return "", ErrInvalidOTPLength
	}

	var builder strings.Builder
	builder.Grow(length)

	max := big.NewInt(int64(len(otpCharset)))
	for i := 0; i < length; i++ {
		n, err := rand.Int(rand.Reader, max)
		if err != nil {
			return "", err
		}
		builder.WriteByte(otpCharset[n.Int64()])
	}

	return builder.String(), nil
}
