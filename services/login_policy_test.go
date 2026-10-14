package services

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"e-absensi-backend/models"
)

const (
	testMaxAttempts = 5
	testWindow      = 15 * time.Minute
	testLockDur     = 15 * time.Minute
)

func TestApplyFailedAttempt_LocksExactlyAtThreshold(t *testing.T) {
	now := time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)
	user := &models.User{}

	// Percobaan 1..4 hanya menambah counter, akun belum terkunci.
	for i := 1; i <= 4; i++ {
		locked := ApplyFailedAttempt(user, now, testMaxAttempts, testWindow, testLockDur)
		assert.Falsef(t, locked, "percobaan ke-%d seharusnya belum mengunci", i)
		assert.Equal(t, int16(i), user.FailedLoginAttempts)
	}

	// Percobaan ke-5 mengunci akun dan mereset counter/penanda waktu.
	locked := ApplyFailedAttempt(user, now, testMaxAttempts, testWindow, testLockDur)
	assert.True(t, locked, "percobaan ke-5 harus mengunci akun")
	assert.Equal(t, int16(0), user.FailedLoginAttempts)
	assert.Nil(t, user.FailedLoginAt)
	assert.NotNil(t, user.LockedUntil)
	assert.True(t, user.LockedUntil.Equal(now.Add(testLockDur)))
	assert.True(t, IsLocked(user, now))
}

func TestApplyFailedAttempt_ResetsCountingAfterWindow(t *testing.T) {
	base := time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)
	user := &models.User{}

	for i := 0; i < 4; i++ {
		ApplyFailedAttempt(user, base, testMaxAttempts, testWindow, testLockDur)
	}
	assert.Equal(t, int16(4), user.FailedLoginAttempts)

	// Kegagalan berikutnya terjadi setelah window 15 menit -> hitungan mulai dari 1.
	later := base.Add(16 * time.Minute)
	locked := ApplyFailedAttempt(user, later, testMaxAttempts, testWindow, testLockDur)
	assert.False(t, locked)
	assert.Equal(t, int16(1), user.FailedLoginAttempts)
}

func TestApplyFailedAttempt_WithinWindowAccumulates(t *testing.T) {
	base := time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)
	user := &models.User{}

	for i := 0; i < 4; i++ {
		ApplyFailedAttempt(user, base, testMaxAttempts, testWindow, testLockDur)
	}
	// Masih di dalam window -> tetap akumulatif (mengunci di percobaan ke-5).
	within := base.Add(10 * time.Minute)
	locked := ApplyFailedAttempt(user, within, testMaxAttempts, testWindow, testLockDur)
	assert.True(t, locked)
}

func TestIsLockedAndRemaining(t *testing.T) {
	now := time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)

	until := now.Add(5 * time.Minute)
	user := &models.User{LockedUntil: &until}
	assert.True(t, IsLocked(user, now))
	assert.InDelta(t, (5 * time.Minute).Seconds(), LockRemaining(user, now).Seconds(), 1)

	expired := now.Add(-time.Minute)
	user.LockedUntil = &expired
	assert.False(t, IsLocked(user, now))
	assert.Equal(t, time.Duration(0), LockRemaining(user, now))
}

func TestResetLockout(t *testing.T) {
	until := time.Now().Add(time.Minute)
	at := time.Now()
	user := &models.User{FailedLoginAttempts: 3, FailedLoginAt: &at, LockedUntil: &until}

	ResetLockout(user)

	assert.Equal(t, int16(0), user.FailedLoginAttempts)
	assert.Nil(t, user.FailedLoginAt)
	assert.Nil(t, user.LockedUntil)
}
