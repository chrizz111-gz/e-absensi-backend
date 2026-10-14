package services

import (
	"time"

	"e-absensi-backend/models"
)

// ApplyFailedAttempt memutakhirkan state lockout user setelah SATU kegagalan
// login, mengikuti PRD Epic 1 Bagian 3.B:
//   - Kegagalan dihitung dalam window `window` (default 15 menit) sejak
//     kegagalan terakhir. Bila sudah lewat window, counter dimulai ulang.
//   - Mencapai `maxAttempts` (default 5) -> akun terkunci selama `lockDuration`
//     (default 15 menit), lalu counter & penanda waktu direset.
//
// Mengembalikan true bila akun BARU SAJA terkunci pada percobaan ini.
func ApplyFailedAttempt(user *models.User, now time.Time, maxAttempts int, window, lockDuration time.Duration) bool {
	if user.FailedLoginAt == nil || now.Sub(*user.FailedLoginAt) > window {
		user.FailedLoginAttempts = 0
	}

	user.FailedLoginAttempts++
	attemptedAt := now
	user.FailedLoginAt = &attemptedAt

	if int(user.FailedLoginAttempts) >= maxAttempts {
		lockedUntil := now.Add(lockDuration)
		user.LockedUntil = &lockedUntil
		user.FailedLoginAttempts = 0
		user.FailedLoginAt = nil
		return true
	}
	return false
}

// IsLocked mengembalikan true bila akun masih terkunci pada waktu `now`.
func IsLocked(user *models.User, now time.Time) bool {
	return user.LockedUntil != nil && user.LockedUntil.After(now)
}

// LockRemaining mengembalikan sisa durasi lock; 0 bila tidak terkunci.
func LockRemaining(user *models.User, now time.Time) time.Duration {
	if !IsLocked(user, now) {
		return 0
	}
	return user.LockedUntil.Sub(now)
}

// ResetLockout mengosongkan seluruh state lockout setelah login berhasil.
func ResetLockout(user *models.User) {
	user.FailedLoginAttempts = 0
	user.FailedLoginAt = nil
	user.LockedUntil = nil
}
