package services

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"

	"e-absensi-backend/models"
	"e-absensi-backend/repositories"
	"e-absensi-backend/utils"
)

// PasswordService menangani ganti & reset password, termasuk kewajiban
// mencabut SELURUH sesi user setelah password berubah (F1.4/F1.5).
type PasswordService struct {
	users    *repositories.UserRepository
	sessions *repositories.SessionRepository
	otp      *OTPService
	now      func() time.Time
}

// NewPasswordService membuat PasswordService baru.
func NewPasswordService(db *gorm.DB, otp *OTPService) *PasswordService {
	return &PasswordService{
		users:    repositories.NewUserRepository(db),
		sessions: repositories.NewSessionRepository(db),
		otp:      otp,
		now:      time.Now,
	}
}

// ChangePassword memverifikasi OTP (purpose change_password), mengganti password,
// lalu mencabut seluruh sesi user (force logout semua device).
func (s *PasswordService) ChangePassword(ctx context.Context, userID int64, code, newPassword string) error {
	if err := s.otp.Verify(userID, models.PurposeChangePassword, code); err != nil {
		return err
	}
	return s.applyNewPassword(userID, newPassword)
}

// ResetPassword mencari user via email, memverifikasi OTP (purpose reset_password),
// mengganti password, lalu mencabut seluruh sesi user.
func (s *PasswordService) ResetPassword(ctx context.Context, email, code, newPassword string) error {
	user, err := s.users.FindByEmail(email)
	if err != nil {
		if errors.Is(err, repositories.ErrNotFound) {
			// Pesan generik: jangan bocorkan keberadaan akun.
			return utils.ErrOTPInvalid
		}
		return utils.ErrInternal
	}
	if err := s.otp.Verify(user.ID, models.PurposeResetPassword, code); err != nil {
		return err
	}
	return s.applyNewPassword(user.ID, newPassword)
}

// applyNewPassword menyimpan hash password baru dan mencabut seluruh sesi aktif.
func (s *PasswordService) applyNewPassword(userID int64, newPassword string) error {
	hash, err := utils.HashPassword(newPassword)
	if err != nil {
		return utils.ErrInternal
	}

	if err := s.users.UpdatePassword(userID, hash, s.now()); err != nil {
		return utils.ErrInternal
	}

	if _, err := s.sessions.RevokeAllForUser(userID, s.now()); err != nil {
		return utils.ErrInternal
	}
	return nil
}
