package utils

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPasswordResetTokenRoundTrip(t *testing.T) {
	t.Setenv("JWT_SECRET", "test-secret")

	token, err := GeneratePasswordResetToken(12, 34, "change_password", 5*time.Minute)
	require.NoError(t, err)

	claims, err := ParsePasswordResetToken(token)
	require.NoError(t, err)
	assert.Equal(t, int64(12), claims.UserID)
	assert.Equal(t, int64(34), claims.OTPID)
	assert.Equal(t, "change_password", claims.Purpose)
	assert.Equal(t, PasswordResetToken, claims.TokenType)
}

func TestPasswordResetTokenRejectsOtherTokenTypes(t *testing.T) {
	t.Setenv("JWT_SECRET", "test-secret")

	access, err := GenerateAccessToken(12, "budi@example.com", []string{"siswa"})
	require.NoError(t, err)

	_, err = ParsePasswordResetToken(access)
	assert.ErrorIs(t, err, ErrInvalidToken)
}

func TestPasswordResetTokenExpired(t *testing.T) {
	t.Setenv("JWT_SECRET", "test-secret")

	secret, err := accessSecret()
	require.NoError(t, err)

	token, err := signToken(secret, Claims{
		UserID:    12,
		OTPID:     34,
		Purpose:   "reset_password",
		TokenType: PasswordResetToken,
	}, -time.Minute)
	require.NoError(t, err)

	_, err = ParsePasswordResetToken(token)
	assert.ErrorIs(t, err, ErrExpiredToken)
}
