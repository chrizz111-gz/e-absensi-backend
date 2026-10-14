package services

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"e-absensi-backend/models"
)

func TestIsOTPExpired(t *testing.T) {
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)

	cases := []struct {
		name      string
		expiredAt time.Time
		want      bool
	}{
		{"masih berlaku", now.Add(30 * time.Second), false},
		{"tepat di batas", now, true},
		{"kedaluwarsa", now.Add(-time.Second), true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			otp := &models.OTPCode{ExpiredAt: tc.expiredAt}
			assert.Equal(t, tc.want, IsOTPExpired(otp, now))
		})
	}
}

func TestOTPAttemptsExhausted(t *testing.T) {
	assert.False(t, OTPAttemptsExhausted(&models.OTPCode{AttemptCount: 4}, 5))
	assert.True(t, OTPAttemptsExhausted(&models.OTPCode{AttemptCount: 5}, 5))
	assert.True(t, OTPAttemptsExhausted(&models.OTPCode{AttemptCount: 6}, 5))
}

func TestOTPPurposeMatches(t *testing.T) {
	otp := &models.OTPCode{Purpose: models.PurposeChangePassword}
	assert.True(t, OTPPurposeMatches(otp, models.PurposeChangePassword))
	assert.False(t, OTPPurposeMatches(otp, models.PurposeResetPassword))
}

func TestOTPSendCooldownRemaining(t *testing.T) {
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	cooldown := 60 * time.Second

	t.Run("baru dikirim - masih cooldown", func(t *testing.T) {
		remaining := OTPSendCooldownRemaining(now.Add(-10*time.Second), now, cooldown)
		assert.Equal(t, 50*time.Second, remaining)
	})

	t.Run("sudah lewat cooldown", func(t *testing.T) {
		remaining := OTPSendCooldownRemaining(now.Add(-90*time.Second), now, cooldown)
		assert.Zero(t, remaining)
	})

	t.Run("tepat di batas cooldown", func(t *testing.T) {
		remaining := OTPSendCooldownRemaining(now.Add(-cooldown), now, cooldown)
		assert.Zero(t, remaining)
	})
}
