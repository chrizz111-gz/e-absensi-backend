package repositories

import (
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"e-absensi-backend/models"
	"e-absensi-backend/utils"
)

// UserListParams adalah filter daftar user yang sudah dinormalisasi service.
type UserListParams struct {
	Page            int
	Limit           int
	Q               string
	Roles           []string
	Status          string
	IncludeArchived bool
}

// AdminLock adalah baris admin yang dikunci selama operasi proteksi last-admin.
type AdminLock struct {
	ID       int64 `gorm:"column:id"`
	IsActive bool  `gorm:"column:is_active"`
}

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

// FindByUsername mencari user berdasarkan username (case-insensitive). Username
// disimpan lowercase, tetapi pencarian tetap toleran terhadap input campuran.
func (r *UserRepository) FindByUsername(username string) (*models.User, error) {
	var user models.User
	if err := r.db.Where("lower(username) = lower(?)", strings.TrimSpace(username)).First(&user).Error; err != nil {
		return nil, translate(err)
	}
	return &user, nil
}

// FindByIdentifier mencari user lewat email atau username (keduanya
// case-insensitive).
func (r *UserRepository) FindByIdentifier(identifier string) (*models.User, error) {
	var user models.User
	id := strings.TrimSpace(identifier)
	err := r.db.Where("lower(email) = lower(?) OR lower(username) = lower(?)", id, id).First(&user).Error
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

// List mengambil satu halaman user beserta total baris. Pencarian q bersifat
// partial dan wildcard %/_ di-escape sebagai teks literal.
func (r *UserRepository) List(params UserListParams) ([]models.User, int64, error) {
	applyFilters := func(db *gorm.DB) *gorm.DB {
		query := db.Model(&models.User{})

		if !params.IncludeArchived {
			query = query.Where("users.archived_at IS NULL")
		}

		if q := strings.TrimSpace(params.Q); q != "" {
			like := "%" + utils.EscapeLike(q) + "%"
			query = query.Where(
				`(users.name ILIKE ? ESCAPE '\' OR users.email ILIKE ? ESCAPE '\' OR COALESCE(users.nisn, '') ILIKE ? ESCAPE '\')`,
				like, like, like,
			)
		}

		if len(params.Roles) > 0 {
			query = query.Where(
				`EXISTS (
					SELECT 1
					FROM user_roles ur
					JOIN roles r ON r.id = ur.role_id
					WHERE ur.user_id = users.id AND r.name IN ?
				)`,
				params.Roles,
			)
		}

		switch params.Status {
		case "active":
			query = query.Where("users.is_active = true")
		case "inactive":
			query = query.Where("users.is_active = false")
		}

		return query
	}

	var total int64
	if err := applyFilters(r.db).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var users []models.User
	offset := (params.Page - 1) * params.Limit
	err := applyFilters(r.db).
		Order("users.name ASC, users.id ASC").
		Offset(offset).
		Limit(params.Limit).
		Find(&users).Error
	if err != nil {
		return nil, 0, err
	}
	return users, total, nil
}

// FindByIDForUpdate membaca user dan mengunci barisnya sampai transaksi selesai.
func (r *UserRepository) FindByIDForUpdate(id int64) (*models.User, error) {
	var user models.User
	err := r.db.Clauses(clause.Locking{Strength: "UPDATE"}).
		First(&user, "id = ?", id).Error
	if err != nil {
		return nil, translate(err)
	}
	return &user, nil
}

// GetRolesByUserIDs mengambil role seluruh user pada satu halaman tanpa N+1.
func (r *UserRepository) GetRolesByUserIDs(userIDs []int64) (map[int64][]models.Role, error) {
	result := make(map[int64][]models.Role, len(userIDs))
	if len(userIDs) == 0 {
		return result, nil
	}

	var rows []models.UserRole
	if err := r.db.Preload("Role").
		Where("user_id IN ?", userIDs).
		Order("user_id, role_id").
		Find(&rows).Error; err != nil {
		return nil, err
	}
	for _, row := range rows {
		result[row.UserID] = append(result[row.UserID], row.Role)
	}
	return result, nil
}

// LockAdminUsers mengunci seluruh baris user yang memiliki role admin. Dipakai
// untuk mencegah dua transaksi bersamaan sama-sama meloloskan "last Admin".
func (r *UserRepository) LockAdminUsers() ([]AdminLock, error) {
	var admins []AdminLock
	err := r.db.Raw(`
		SELECT u.id, u.is_active
		FROM users u
		JOIN user_roles ur ON ur.user_id = u.id
		JOIN roles r ON r.id = ur.role_id
		WHERE r.name = ?
		ORDER BY u.id
		FOR UPDATE OF u
	`, models.RoleAdmin).Scan(&admins).Error
	if err != nil {
		return nil, err
	}
	return admins, nil
}

// UpdateStatus menerapkan perubahan status akun. Pemanggil bertanggung jawab
// menyiapkan seluruh kolom agar konsisten dengan CHECK ck_users_inactive_state.
func (r *UserRepository) UpdateStatus(userID int64, updates map[string]interface{}) error {
	return r.db.Model(&models.User{}).Where("id = ?", userID).Updates(updates).Error
}

// UpdateProfile menerapkan perubahan profil dengan optimistic locking pada
// updated_at. RowsAffected=0 berarti data sudah berubah sejak dibaca.
func (r *UserRepository) UpdateProfile(userID int64, expectedUpdatedAt time.Time, updates map[string]interface{}) (int64, error) {
	res := r.db.Model(&models.User{}).
		Where("id = ? AND updated_at = ?", userID, expectedUpdatedAt).
		Updates(updates)
	return res.RowsAffected, res.Error
}

// DeleteAssignableRoles mencabut seluruh role assignable user. Role terkunci
// (wali_kelas/siswa_inti/siswa) tidak disentuh.
func (r *UserRepository) DeleteAssignableRoles(userID int64) error {
	return r.db.Exec(`
		DELETE FROM user_roles
		WHERE user_id = ?
		  AND role_id IN (SELECT id FROM roles WHERE is_assignable = true)
	`, userID).Error
}

// AssignRoles memberikan beberapa role ke user secara idempotent.
func (r *UserRepository) AssignRoles(userID int64, roleIDs []int16) error {
	if len(roleIDs) == 0 {
		return nil
	}
	rows := make([]models.UserRole, 0, len(roleIDs))
	for _, roleID := range roleIDs {
		rows = append(rows, models.UserRole{UserID: userID, RoleID: roleID})
	}
	return r.db.Clauses(clause.OnConflict{DoNothing: true}).Create(&rows).Error
}

// Delete menghapus user dan mengembalikan jumlah baris yang terhapus.
func (r *UserRepository) Delete(userID int64) (int64, error) {
	res := r.db.Delete(&models.User{}, userID)
	return res.RowsAffected, res.Error
}
