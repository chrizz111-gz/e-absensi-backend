package services

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"gorm.io/gorm"

	"e-absensi-backend/config"
	"e-absensi-backend/models"
	"e-absensi-backend/repositories"
	"e-absensi-backend/utils"
)

// OTPService menangani pembuatan, pengiriman, dan verifikasi OTP
// (F1.4 ganti password, F1.5 reset password, F1.6 lifecycle).
type OTPService struct {
	users  *repositories.UserRepository
	otps   *repositories.OTPRepository
	mailer utils.EmailSender
	now    func() time.Time
}

// NewOTPService membuat OTPService baru.
func NewOTPService(db *gorm.DB, mailer utils.EmailSender) *OTPService {
	return &OTPService{
		users:  repositories.NewUserRepository(db),
		otps:   repositories.NewOTPRepository(db),
		mailer: mailer,
		now:    time.Now,
	}
}

// RequestChangePasswordOTP mengirim (atau mengirim ulang) OTP untuk ganti password.
func (s *OTPService) RequestChangePasswordOTP(ctx context.Context, userID int64) error {
	user, err := s.users.FindByID(userID)
	if err != nil {
		return utils.ErrInternal
	}
	return s.send(user, models.PurposeChangePassword)
}

// RequestResetPasswordOTP mengirim OTP reset password. Bersifat "silent":
// keberadaan email tidak dibocorkan - selalu sukses dari sudut pemanggil.
func (s *OTPService) RequestResetPasswordOTP(ctx context.Context, email string) error {
	user, err := s.users.FindByEmail(email)
	if err != nil {
		if !errors.Is(err, repositories.ErrNotFound) {
			log.Printf("otp: gagal mencari user untuk reset (%s): %v", email, err)
		}
		return nil
	}

	if err := s.send(user, models.PurposeResetPassword); err != nil {
		log.Printf("otp: gagal mengirim OTP reset untuk %s: %v", email, err)
	}
	return nil
}

// Verify memvalidasi OTP untuk user & purpose tertentu. Aturan:
//   - hanya OTP aktif terbaru untuk purpose yang sama,
//   - kode kedaluwarsa / attempts habis -> hangus,
//   - kode benar -> ditandai terpakai (sekali pakai).
func (s *OTPService) Verify(userID int64, purpose, code string) error {
	now := s.now()

	otp, err := s.otps.FindLatestActive(userID, purpose)
	if err != nil {
		if errors.Is(err, repositories.ErrNotFound) {
			return utils.ErrOTPInvalid
		}
		return utils.ErrInternal
	}

	if !OTPPurposeMatches(otp, purpose) {
		return utils.ErrOTPInvalid
	}
	if OTPAttemptsExhausted(otp, config.OTPMaxAttempts) {
		_ = s.otps.MarkUsed(otp.ID)
		return utils.ErrOTPTooMany
	}
	if IsOTPExpired(otp, now) {
		_ = s.otps.MarkUsed(otp.ID)
		return utils.ErrOTPExpired
	}

	if !utils.CheckPasswordHash(code, otp.CodeHash) {
		newCount, incErr := s.otps.IncrementAttempt(otp.ID)
		if incErr != nil {
			return utils.ErrInternal
		}
		if int(newCount) >= config.OTPMaxAttempts {
			_ = s.otps.MarkUsed(otp.ID)
			return utils.ErrOTPTooMany
		}
		return utils.ErrOTPInvalid
	}

	if err := s.otps.MarkUsed(otp.ID); err != nil {
		return utils.ErrInternal
	}
	return nil
}

// send membuat OTP baru untuk user & purpose, menghormati cooldown resend dan
// batas jumlah kirim per jam, lalu mengirimkannya lewat email.
func (s *OTPService) send(user *models.User, purpose string) error {
	now := s.now()

	existing, err := s.otps.FindLatestActive(user.ID, purpose)
	if err == nil {
		remaining := OTPSendCooldownRemaining(existing.LastSentAt, now, config.OTPResendCooldown)
		if remaining > 0 {
			seconds := int(remaining.Seconds()) + 1
			return utils.ErrOTPResendTooSoon.WithMessage(
				fmt.Sprintf("Mohon tunggu %d detik sebelum meminta kode baru", seconds),
			)
		}
	} else if !errors.Is(err, repositories.ErrNotFound) {
		return utils.ErrInternal
	}

	count, err := s.otps.CountSentSince(user.ID, purpose, now.Add(-time.Hour))
	if err != nil {
		return utils.ErrInternal
	}
	if int(count) >= config.OTPMaxResendPerHour {
		return utils.ErrOTPResendLimit
	}

	if err := s.otps.InvalidateActive(user.ID, purpose); err != nil {
		return utils.ErrInternal
	}

	code, err := utils.GenerateOTP()
	if err != nil {
		return utils.ErrInternal
	}
	hash, err := utils.HashPassword(code)
	if err != nil {
		return utils.ErrInternal
	}

	otp := &models.OTPCode{
		UserID:     user.ID,
		CodeHash:   hash,
		Purpose:    purpose,
		ExpiredAt:  now.Add(config.OTPTTL),
		LastSentAt: now,
	}
	if err := s.otps.Create(otp); err != nil {
		return utils.ErrInternal
	}

	if err := s.mailer.SendOTP(user.Email, user.Name, code, purpose); err != nil {
		return utils.ErrInternal
	}
	return nil
}
