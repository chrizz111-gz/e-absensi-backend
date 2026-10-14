package services

import (
	"time"

	"e-absensi-backend/models"
)

// IsOTPExpired mengembalikan true bila OTP sudah melewati masa berlaku.
func IsOTPExpired(otp *models.OTPCode, now time.Time) bool {
	return !otp.ExpiredAt.After(now)
}

// OTPAttemptsExhausted mengembalikan true bila percobaan salah sudah mencapai batas.
func OTPAttemptsExhausted(otp *models.OTPCode, maxAttempts int) bool {
	return int(otp.AttemptCount) >= maxAttempts
}

// OTPPurposeMatches memastikan OTP hanya dipakai untuk purpose yang sesuai
// (change_password tidak bisa dipakai untuk reset_password, dan sebaliknya).
func OTPPurposeMatches(otp *models.OTPCode, purpose string) bool {
	return otp.Purpose == purpose
}

// OTPSendCooldownRemaining mengembalikan sisa waktu cooldown sebelum boleh
// mengirim OTP baru; 0 berarti sudah boleh.
func OTPSendCooldownRemaining(lastSentAt, now time.Time, cooldown time.Duration) time.Duration {
	elapsed := now.Sub(lastSentAt)
	if elapsed >= cooldown {
		return 0
	}
	return cooldown - elapsed
}
