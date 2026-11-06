package services

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"gorm.io/gorm"

	"e-absensi-backend/config"
	"e-absensi-backend/dto"
	"e-absensi-backend/models"
	"e-absensi-backend/repositories"
	"e-absensi-backend/utils"
)

// OTPIssueResult membawa metadata OTP yang baru dibuat/dikirim.
type OTPIssueResult struct {
	ExpiresAt          time.Time
	AttemptsRemaining  int
	ResendAfterSeconds int
}

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
func (s *OTPService) RequestChangePasswordOTP(ctx context.Context, userID int64) (*OTPIssueResult, error) {
	user, err := s.users.FindByID(userID)
	if err != nil {
		return nil, utils.ErrInternal
	}
	return s.send(user, models.PurposeChangePassword)
}

// RequestResetPasswordOTP mengirim OTP reset password. Bersifat "silent":
// keberadaan email tidak dibocorkan - selalu sukses dari sudut pemanggil.
func (s *OTPService) RequestResetPasswordOTP(ctx context.Context, email string) (*OTPIssueResult, error) {
	user, err := s.users.FindByEmail(email)
	if err != nil {
		if !errors.Is(err, repositories.ErrNotFound) {
			log.Printf("otp: gagal mencari user untuk reset (%s): %v", email, err)
		}
		return s.syntheticIssueResult(), nil
	}

	result, err := s.send(user, models.PurposeResetPassword)
	if err != nil {
		log.Printf("otp: gagal mengirim OTP reset untuk %s: %v", email, err)
		return s.syntheticIssueResult(), nil
	}
	return result, nil
}

// Check memvalidasi OTP untuk user & purpose tertentu tanpa menghanguskannya.
// OTP baru dianggap terpakai ketika password benar-benar disimpan. Aturan:
//   - hanya OTP terbaru untuk purpose yang sama,
//   - kode kedaluwarsa / attempts habis -> hangus,
//   - kode benar -> tetap aktif sampai dikonsumsi endpoint simpan password.
func (s *OTPService) Check(userID int64, purpose, code string) (*models.OTPCode, error) {
	now := s.now()
	maxAttempts := otpMaxAttempts()
	code = utils.NormalizeOTP(code)

	otp, err := s.otps.FindLatest(userID, purpose)
	if err != nil {
		if errors.Is(err, repositories.ErrNotFound) {
			return nil, utils.ErrOTPInvalid
		}
		return nil, utils.ErrInternal
	}

	if !OTPPurposeMatches(otp, purpose) {
		return nil, utils.ErrOTPInvalid
	}

	if OTPAttemptsExhausted(otp, maxAttempts) {
		_ = s.otps.MarkUsed(otp.ID)
		return nil, utils.ErrOTPTooMany.WithData(dto.OTPAttemptErrorData{
			AttemptsRemaining: 0,
			ExpiresAt:         &otp.ExpiredAt,
		})
	}

	remaining := OTPRemainingAttempts(otp.AttemptCount, maxAttempts)
	attemptData := dto.OTPAttemptErrorData{
		AttemptsRemaining: remaining,
		ExpiresAt:         &otp.ExpiredAt,
	}

	if otp.IsUsed {
		return nil, utils.ErrOTPInvalid.WithData(attemptData)
	}
	if IsOTPExpired(otp, now) {
		_ = s.otps.MarkUsed(otp.ID)
		return nil, utils.ErrOTPExpired.WithData(attemptData)
	}

	if !utils.CheckPasswordHash(code, otp.CodeHash) {
		newCount, incErr := s.otps.IncrementAttempt(otp.ID)
		if incErr != nil {
			return nil, utils.ErrInternal
		}
		remaining = OTPRemainingAttempts(newCount, maxAttempts)
		attemptData.AttemptsRemaining = remaining

		if int(newCount) >= maxAttempts {
			_ = s.otps.MarkUsed(otp.ID)
			return nil, utils.ErrOTPTooMany.WithData(dto.OTPAttemptErrorData{
				AttemptsRemaining: 0,
				ExpiresAt:         &otp.ExpiredAt,
			})
		}
		return nil, utils.ErrOTPInvalid.WithData(attemptData)
	}

	return otp, nil
}

// fakeOTPIssueResult dipakai untuk email tak terdaftar agar bentuk response
// tetap sama tanpa membocorkan keberadaan akun.
func (s *OTPService) syntheticIssueResult() *OTPIssueResult {
	return &OTPIssueResult{
		ExpiresAt:          s.now().Add(otpTTL()),
		AttemptsRemaining:  otpMaxAttempts(),
		ResendAfterSeconds: int(otpResendCooldown().Seconds()),
	}
}

// send membuat OTP baru untuk user & purpose, menghormati cooldown resend dan
// batas jumlah kirim per jam, lalu mengirimkannya lewat email.
func (s *OTPService) send(user *models.User, purpose string) (*OTPIssueResult, error) {
	now := s.now()

	existing, err := s.otps.FindLatestActive(user.ID, purpose)
	if err == nil {
		remaining := OTPSendCooldownRemaining(existing.LastSentAt, now, otpResendCooldown())
		if remaining > 0 {
			seconds := int(remaining.Seconds()) + 1
			return nil, utils.ErrOTPResendTooSoon.
				WithMessage(fmt.Sprintf("Mohon tunggu %d detik sebelum meminta kode baru", seconds)).
				WithData(dto.RetryAfterData{RetryAfterSeconds: seconds})
		}
	} else if !errors.Is(err, repositories.ErrNotFound) {
		return nil, utils.ErrInternal
	}

	count, err := s.otps.CountSentSince(user.ID, purpose, now.Add(-time.Hour))
	if err != nil {
		return nil, utils.ErrInternal
	}
	if int(count) >= otpMaxResendPerHour() {
		return nil, utils.ErrOTPResendLimit
	}

	if err := s.otps.InvalidateActive(user.ID, purpose); err != nil {
		return nil, utils.ErrInternal
	}

	code, err := utils.GenerateOTP(otpLength())
	if err != nil {
		return nil, utils.ErrInternal
	}
	hash, err := utils.HashPassword(code)
	if err != nil {
		return nil, utils.ErrInternal
	}

	expiresAt := now.Add(otpTTL())
	otp := &models.OTPCode{
		UserID:     user.ID,
		CodeHash:   hash,
		Purpose:    purpose,
		ExpiredAt:  expiresAt,
		LastSentAt: now,
	}
	if err := s.otps.Create(otp); err != nil {
		return nil, utils.ErrInternal
	}

	if err := s.mailer.SendOTP(user.Email, user.Name, code, purpose); err != nil {
		return nil, utils.ErrInternal
	}

	return &OTPIssueResult{
		ExpiresAt:          expiresAt,
		AttemptsRemaining:  otpMaxAttempts(),
		ResendAfterSeconds: int(otpResendCooldown().Seconds()),
	}, nil
}

// Helper konfigurasi dengan fallback agar service tetap aman dipakai di test
// sebelum config.Init() dijalankan.
func otpLength() int {
	if config.OTPLength > 0 {
		return config.OTPLength
	}
	return 6
}

func otpTTL() time.Duration {
	if config.OTPTTL > 0 {
		return config.OTPTTL
	}
	return 5 * time.Minute
}

func otpResendCooldown() time.Duration {
	if config.OTPResendCooldown > 0 {
		return config.OTPResendCooldown
	}
	return 60 * time.Second
}

func otpMaxResendPerHour() int {
	if config.OTPMaxResendPerHour > 0 {
		return config.OTPMaxResendPerHour
	}
	return 5
}

func otpMaxAttempts() int {
	if config.OTPMaxAttempts > 0 {
		return config.OTPMaxAttempts
	}
	return 5
}
