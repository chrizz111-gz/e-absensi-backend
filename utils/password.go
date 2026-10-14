package utils

import "golang.org/x/crypto/bcrypt"

// bcryptCost determines the computational cost of the bcrypt algorithm.
// DefaultCost (10) is a solid balance between security and latency for
// interactive authentication endpoints.
const bcryptCost = bcrypt.DefaultCost

// HashPassword hashes a plaintext password using bcrypt and returns the
// resulting hash encoded as a string (including the salt and cost).
func HashPassword(password string) (string, error) {
	hashed, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
	if err != nil {
		return "", err
	}
	return string(hashed), nil
}

// CheckPasswordHash compares a plaintext password against a bcrypt hash.
// It returns true only when the password matches. bcrypt's verification
// is constant-time, which protects against timing attacks.
func CheckPasswordHash(password, hash string) bool {
	if hash == "" {
		return false
	}
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}
