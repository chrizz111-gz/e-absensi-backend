package repositories

import (
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"e-absensi-backend/models"
)

// OTPRepository menangani query tabel otp_codes.
type OTPRepository struct {
	db *gorm.DB
}

// NewOTPRepository membuat OTPRepository baru.
func NewOTPRepository(db *gorm.DB) *OTPRepository {
	return &OTPRepository{db: db}
}

// WithTx mengembalikan repository yang terikat pada transaksi tertentu.
func (r *OTPRepository) WithTx(tx *gorm.DB) *OTPRepository {
	return &OTPRepository{db: tx}
}

// Create menyimpan OTP baru (dalam bentuk hash).
func (r *OTPRepository) Create(otp *models.OTPCode) error {
	return r.db.Create(otp).Error
}

// FindLatestActive mengambil OTP aktif (belum dipakai) terbaru untuk user & purpose.
// Memanfaatkan partial index ix_otp_active.
func (r *OTPRepository) FindLatestActive(userID int64, purpose string) (*models.OTPCode, error) {
	var otp models.OTPCode
	err := r.db.
		Where("user_id = ? AND purpose = ? AND is_used = false", userID, purpose).
		Order("created_at DESC").
		First(&otp).Error
	if err != nil {
		return nil, translate(err)
	}
	return &otp, nil
}

// InvalidateActive menandai seluruh OTP aktif milik user & purpose sebagai
// terpakai (dipakai saat resend/permintaan kode baru).
func (r *OTPRepository) InvalidateActive(userID int64, purpose string) error {
	return r.db.Model(&models.OTPCode{}).
		Where("user_id = ? AND purpose = ? AND is_used = false", userID, purpose).
		Update("is_used", true).Error
}

// MarkUsed menandai satu OTP sebagai terpakai.
func (r *OTPRepository) MarkUsed(id int64) error {
	return r.db.Model(&models.OTPCode{}).
		Where("id = ?", id).
		Update("is_used", true).Error
}

// IncrementAttempt menaikkan attempt_count satu OTP dan mengembalikan nilai
// barunya (via RETURNING).
func (r *OTPRepository) IncrementAttempt(id int64) (int16, error) {
	var otp models.OTPCode
	err := r.db.Model(&otp).
		Clauses(clause.Returning{Columns: []clause.Column{{Name: "attempt_count"}}}).
		Where("id = ?", id).
		Update("attempt_count", gorm.Expr("attempt_count + 1")).Error
	if err != nil {
		return 0, err
	}
	return otp.AttemptCount, nil
}

// CountSentSince menghitung jumlah OTP yang dibuat sejak waktu tertentu
// (dasar pembatasan resend per jam).
func (r *OTPRepository) CountSentSince(userID int64, purpose string, since time.Time) (int64, error) {
	var count int64
	err := r.db.Model(&models.OTPCode{}).
		Where("user_id = ? AND purpose = ? AND created_at >= ?", userID, purpose, since).
		Count(&count).Error
	return count, err
}
