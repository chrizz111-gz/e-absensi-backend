package services

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"

	"e-absensi-backend/config"
	"e-absensi-backend/dto"
	"e-absensi-backend/models"
	"e-absensi-backend/repositories"
	"e-absensi-backend/utils"
)

// PasswordService menangani verifikasi OTP, penerbitan reset token, serta
// penyimpanan password baru. Setelah password berubah, SELURUH sesi user
// dicabut (F1.4/F1.5).
type PasswordService struct {
	db       *gorm.DB
	users    *repositories.UserRepository
	sessions *repositories.SessionRepository
	otps     *repositories.OTPRepository
	otp      *OTPService
	now      func() time.Time
}

// NewPasswordService membuat PasswordService baru.
func NewPasswordService(db *gorm.DB, otp *OTPService) *PasswordService {
	return &PasswordService{
		db:       db,
		users:    repositories.NewUserRepository(db),
		sessions: repositories.NewSessionRepository(db),
		otps:     repositories.NewOTPRepository(db),
		otp:      otp,
		now:      time.Now,
	}
}

// VerifyChangePasswordOTP memeriksa OTP ganti password milik user yang sedang
// login, lalu menerbitkan reset token sekali pakai.
func (s *PasswordService) VerifyChangePasswordOTP(ctx context.Context, userID int64, code string) (*dto.OTPVerifiedResponse, error) {
	otp, err := s.otp.Check(userID, models.PurposeChangePassword, code)
	if err != nil {
		return nil, err
	}
	return s.issueResetToken(otp)
}

// VerifyResetPasswordOTP mencari user via email, memeriksa OTP reset, lalu
// menerbitkan reset token sekali pakai.
func (s *PasswordService) VerifyResetPasswordOTP(ctx context.Context, email, code string) (*dto.OTPVerifiedResponse, error) {
	user, err := s.users.FindByEmail(email)
	if err != nil {
		if errors.Is(err, repositories.ErrNotFound) {
			// Pesan generik: jangan bocorkan keberadaan akun.
			return nil, utils.ErrOTPInvalid
		}
		return nil, utils.ErrInternal
	}

	otp, err := s.otp.Check(user.ID, models.PurposeResetPassword, code)
	if err != nil {
		return nil, err
	}
	return s.issueResetToken(otp)
}

// ConfirmChangePassword menyimpan password baru untuk alur ganti password
// (butuh login). Token wajib milik user yang sedang login.
func (s *PasswordService) ConfirmChangePassword(ctx context.Context, userID int64, resetToken, newPassword string) error {
	return s.setPasswordWithToken(ctx, resetToken, newPassword, models.PurposeChangePassword, &userID)
}

// ResetPasswordWithToken menyimpan password baru untuk alur lupa password
// (publik). Token wajib bertipe reset_password.
func (s *PasswordService) ResetPasswordWithToken(ctx context.Context, resetToken, newPassword string) error {
	return s.setPasswordWithToken(ctx, resetToken, newPassword, models.PurposeResetPassword, nil)
}

// issueResetToken membuat token berumur pendek yang terikat pada OTP. Umur
// token tidak boleh melebihi sisa umur OTP.
func (s *PasswordService) issueResetToken(otp *models.OTPCode) (*dto.OTPVerifiedResponse, error) {
	now := s.now()

	ttl := config.PasswordResetTokenTTL
	if ttl <= 0 {
		ttl = 10 * time.Minute
	}
	if remaining := otp.ExpiredAt.Sub(now); remaining < ttl {
		ttl = remaining
	}
	if ttl <= 0 {
		return nil, utils.ErrOTPExpired
	}

	token, err := utils.GeneratePasswordResetToken(otp.UserID, otp.ID, otp.Purpose, ttl)
	if err != nil {
		return nil, utils.ErrInternal
	}

	return &dto.OTPVerifiedResponse{
		ResetToken:        token,
		ExpiresAt:         now.Add(ttl),
		AttemptsRemaining: OTPRemainingAttempts(otp.AttemptCount, otpMaxAttempts()),
	}, nil
}

// setPasswordWithToken memvalidasi token, mengonsumsi OTP secara atomik,
// menyimpan hash password baru, lalu mencabut seluruh sesi. Semua langkah
// berjalan dalam satu transaksi agar OTP tidak hangus bila penyimpanan gagal.
func (s *PasswordService) setPasswordWithToken(ctx context.Context, resetToken, newPassword, expectedPurpose string, expectedUserID *int64) error {
	// Defense in depth: controller sudah memvalidasi lewat binding, tetapi
	// service tetap menolak password lemah bila dipanggil dari jalur lain.
	if code := utils.PasswordPolicyViolation(newPassword); code != utils.PasswordPolicyOK {
		return utils.NewValidationError([]utils.FieldError{{
			Field:   "new_password",
			Code:    code,
			Message: utils.PasswordPolicyMessage(code),
		}})
	}

	claims, err := utils.ParsePasswordResetToken(resetToken)
	if err != nil {
		if errors.Is(err, utils.ErrExpiredToken) {
			return utils.ErrResetTokenExpired
		}
		return utils.ErrResetTokenInvalid
	}

	if claims.UserID <= 0 || claims.OTPID <= 0 || claims.Purpose != expectedPurpose {
		return utils.ErrResetTokenInvalid
	}
	if expectedUserID != nil && claims.UserID != *expectedUserID {
		return utils.ErrResetTokenInvalid
	}

	now := s.now()
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		users := s.users.WithTx(tx)

		user, err := users.FindByID(claims.UserID)
		if err != nil {
			return utils.ErrResetTokenInvalid
		}

		// Password baru tidak boleh sama dengan password lama.
		if user.PasswordHash != nil && utils.CheckPasswordHash(newPassword, *user.PasswordHash) {
			return utils.ErrPasswordSame
		}

		consumed, err := s.otps.WithTx(tx).ConsumeIfActive(claims.OTPID, claims.UserID, claims.Purpose, now)
		if err != nil {
			return utils.ErrInternal
		}
		if !consumed {
			return utils.ErrResetTokenInvalid
		}

		hash, err := utils.HashPassword(newPassword)
		if err != nil {
			return utils.ErrInternal
		}
		if err := users.UpdatePassword(user.ID, hash, now); err != nil {
			return utils.ErrInternal
		}

		if _, err := s.sessions.WithTx(tx).RevokeAllForUser(user.ID, now); err != nil {
			return utils.ErrInternal
		}
		return nil
	})
}
