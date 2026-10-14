package repositories

import (
	"time"

	"gorm.io/gorm"

	"e-absensi-backend/models"
)

// SessionRepository menangani query tabel sessions (manajemen sesi multi-device).
type SessionRepository struct {
	db *gorm.DB
}

// NewSessionRepository membuat SessionRepository baru.
func NewSessionRepository(db *gorm.DB) *SessionRepository {
	return &SessionRepository{db: db}
}

// WithTx mengembalikan repository yang terikat pada transaksi tertentu.
func (r *SessionRepository) WithTx(tx *gorm.DB) *SessionRepository {
	return &SessionRepository{db: tx}
}

// Create menyimpan sesi baru.
func (r *SessionRepository) Create(session *models.Session) error {
	return r.db.Create(session).Error
}

// FindByTokenHash mencari sesi aktif maupun tidak berdasarkan hash refresh token.
func (r *SessionRepository) FindByTokenHash(hash string) (*models.Session, error) {
	var session models.Session
	if err := r.db.Where("refresh_token_hash = ?", hash).First(&session).Error; err != nil {
		return nil, translate(err)
	}
	return &session, nil
}

// Revoke menandai satu sesi tercabut (soft revoke).
func (r *SessionRepository) Revoke(id int64, at time.Time) error {
	return r.db.Model(&models.Session{}).
		Where("id = ? AND revoked_at IS NULL", id).
		Update("revoked_at", at).Error
}

// RevokeAllForUser mencabut seluruh sesi aktif milik user (force logout semua
// device). Dipakai saat ganti/reset password.
func (r *SessionRepository) RevokeAllForUser(userID int64, at time.Time) (int64, error) {
	res := r.db.Model(&models.Session{}).
		Where("user_id = ? AND revoked_at IS NULL", userID).
		Update("revoked_at", at)
	return res.RowsAffected, res.Error
}

// RotateToken mengganti hash refresh token pada sesi yang sama (rotasi) dan
// memperbarui last_used_at.
func (r *SessionRepository) RotateToken(id int64, newHash string, usedAt time.Time) error {
	return r.db.Model(&models.Session{}).
		Where("id = ? AND revoked_at IS NULL", id).
		Updates(map[string]interface{}{
			"refresh_token_hash": newHash,
			"last_used_at":       usedAt,
		}).Error
}

// Touch memperbarui last_used_at sesi.
func (r *SessionRepository) Touch(id int64, at time.Time) error {
	return r.db.Model(&models.Session{}).
		Where("id = ?", id).
		Update("last_used_at", at).Error
}
