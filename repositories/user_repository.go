package repositories

import (
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"e-absensi-backend/models"
)

// UserRepository menangani seluruh query tabel users dan role-nya.
type UserRepository struct {
	db *gorm.DB
}

// NewUserRepository membuat UserRepository baru.
func NewUserRepository(db *gorm.DB) *UserRepository {
	return &UserRepository{db: db}
}

// WithTx mengembalikan repository yang terikat pada transaksi tertentu.
func (r *UserRepository) WithTx(tx *gorm.DB) *UserRepository {
	return &UserRepository{db: tx}
}

// FindByID mencari user berdasarkan primary key.
func (r *UserRepository) FindByID(id int64) (*models.User, error) {
	var user models.User
	if err := r.db.First(&user, "id = ?", id).Error; err != nil {
		return nil, translate(err)
	}
	return &user, nil
}

// FindByEmail mencari user berdasarkan email (case-insensitive).
func (r *UserRepository) FindByEmail(email string) (*models.User, error) {
	var user models.User
	err := r.db.Where("lower(email) = lower(?)", strings.TrimSpace(email)).First(&user).Error
	if err != nil {
		return nil, translate(err)
	}
	return &user, nil
}

// FindByUsername mencari user berdasarkan username (case-sensitive).
func (r *UserRepository) FindByUsername(username string) (*models.User, error) {
	var user models.User
	if err := r.db.Where("username = ?", strings.TrimSpace(username)).First(&user).Error; err != nil {
		return nil, translate(err)
	}
	return &user, nil
}

// FindByIdentifier mencari user lewat email (case-insensitive) atau username.
func (r *UserRepository) FindByIdentifier(identifier string) (*models.User, error) {
	var user models.User
	id := strings.TrimSpace(identifier)
	err := r.db.Where("lower(email) = lower(?) OR username = ?", id, id).First(&user).Error
	if err != nil {
		return nil, translate(err)
	}
	return &user, nil
}

// FindByNISN mencari user berdasarkan NISN.
func (r *UserRepository) FindByNISN(nisn string) (*models.User, error) {
	var user models.User
	if err := r.db.Where("nisn = ?", strings.TrimSpace(nisn)).First(&user).Error; err != nil {
		return nil, translate(err)
	}
	return &user, nil
}

// FindByGoogleID mencari user berdasarkan sub Google.
func (r *UserRepository) FindByGoogleID(googleID string) (*models.User, error) {
	var user models.User
	if err := r.db.Where("google_id = ?", googleID).First(&user).Error; err != nil {
		return nil, translate(err)
	}
	return &user, nil
}

// ExistsByEmail memeriksa keunikan email (case-insensitive).
func (r *UserRepository) ExistsByEmail(email string) (bool, error) {
	var count int64
	err := r.db.Model(&models.User{}).
		Where("lower(email) = lower(?)", strings.TrimSpace(email)).
		Count(&count).Error
	return count > 0, err
}

// ExistsByNISN memeriksa keunikan NISN.
func (r *UserRepository) ExistsByNISN(nisn string) (bool, error) {
	var count int64
	err := r.db.Model(&models.User{}).
		Where("nisn = ?", strings.TrimSpace(nisn)).
		Count(&count).Error
	return count > 0, err
}

// Create menyimpan user baru. Kolom id (IDENTITY) diisi otomatis oleh DB.
func (r *UserRepository) Create(user *models.User) error {
	return r.db.Create(user).Error
}

// AssignRole memberikan role ke user (idempotent).
func (r *UserRepository) AssignRole(userID int64, roleID int16) error {
	userRole := models.UserRole{UserID: userID, RoleID: roleID}
	return r.db.Clauses(clause.OnConflict{DoNothing: true}).Create(&userRole).Error
}

// GetRoles mengembalikan seluruh role user (union) dari tabel user_roles.
func (r *UserRepository) GetRoles(userID int64) ([]models.Role, error) {
	var roles []models.Role
	err := r.db.Model(&models.Role{}).
		Joins("JOIN user_roles ON user_roles.role_id = roles.id").
		Where("user_roles.user_id = ?", userID).
		Order("roles.id").
		Find(&roles).Error
	return roles, err
}

// UpdateLoginFailure menyimpan counter gagal, waktu gagal terakhir, dan
// locked_until setelah percobaan login gagal.
func (r *UserRepository) UpdateLoginFailure(user *models.User) error {
	return r.db.Model(&models.User{}).Where("id = ?", user.ID).Updates(map[string]interface{}{
		"failed_login_attempts": user.FailedLoginAttempts,
		"failed_login_at":       user.FailedLoginAt,
		"locked_until":          user.LockedUntil,
	}).Error
}

// ResetLoginFailure mengosongkan state lockout setelah login berhasil.
func (r *UserRepository) ResetLoginFailure(userID int64) error {
	return r.db.Model(&models.User{}).Where("id = ?", userID).Updates(map[string]interface{}{
		"failed_login_attempts": 0,
		"failed_login_at":       nil,
		"locked_until":          nil,
	}).Error
}

// UpdatePassword memperbarui hash password dan jejak waktu perubahan.
func (r *UserRepository) UpdatePassword(userID int64, hash string, changedAt time.Time) error {
	return r.db.Model(&models.User{}).Where("id = ?", userID).Updates(map[string]interface{}{
		"password_hash":       hash,
		"password_changed_at": changedAt,
	}).Error
}

// LinkGoogleIdentity mengaitkan akun Google (sub) ke user yang sudah ada,
// hanya bila belum pernah dikaitkan. Avatar diisi bila belum ada.
func (r *UserRepository) LinkGoogleIdentity(userID int64, googleID, avatarURL string) error {
	updates := map[string]interface{}{"google_id": googleID}
	if strings.TrimSpace(avatarURL) != "" {
		updates["avatar_url"] = avatarURL
	}
	return r.db.Model(&models.User{}).
		Where("id = ? AND google_id IS NULL", userID).
		Updates(updates).Error
}
