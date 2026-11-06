package services

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"

	"e-absensi-backend/dto"
	"e-absensi-backend/models"
	"e-absensi-backend/repositories"
	"e-absensi-backend/utils"
)

// validInactiveReasons adalah nilai enum pada constraint ck_users_inactive_reason.
var validInactiveReasons = map[string]bool{
	"graduated":   true,
	"transferred": true,
	"other":       true,
}

// UserService menangani CRUD user dan role assignment Epic 2 Fase 3.
type UserService struct {
	db       *gorm.DB
	users    *repositories.UserRepository
	roles    *repositories.RoleRepository
	sessions *repositories.SessionRepository
	now      func() time.Time
}

// NewUserService membuat UserService baru.
func NewUserService(db *gorm.DB) *UserService {
	return &UserService{
		db:       db,
		users:    repositories.NewUserRepository(db),
		roles:    repositories.NewRoleRepository(db),
		sessions: repositories.NewSessionRepository(db),
		now:      time.Now,
	}
}

// List mengembalikan daftar user dengan pagination, pencarian, filter role, dan
// filter status. Seluruh user pada halaman membawa role masing-masing.
func (s *UserService) List(ctx context.Context, filter dto.UserListFilter) (*dto.UserListResponse, error) {
	if filter.Page < 1 {
		filter.Page = 1
	}
	if filter.Limit < 1 {
		filter.Limit = 20
	}
	if filter.Limit > 100 {
		return nil, validationError("limit", utils.FieldInvalidLimit, "limit maksimal 100")
	}

	status := strings.ToLower(strings.TrimSpace(filter.Status))
	if status != "" && status != "active" && status != "inactive" {
		return nil, validationError("status", utils.FieldInvalidStatus, "status harus active atau inactive")
	}

	users, total, err := s.users.List(repositories.UserListParams{
		Page:   filter.Page,
		Limit:  filter.Limit,
		Q:      strings.TrimSpace(filter.Q),
		Roles:  normalizeRoleNames(filter.Roles),
		Status: status,
	})
	if err != nil {
		return nil, utils.ErrInternal
	}

	ids := make([]int64, 0, len(users))
	for _, user := range users {
		ids = append(ids, user.ID)
	}
	rolesByUser, err := s.users.GetRolesByUserIDs(ids)
	if err != nil {
		return nil, utils.ErrInternal
	}

	items := make([]dto.UserAdminResponse, 0, len(users))
	for _, user := range users {
		items = append(items, toUserAdminResponse(&user, rolesByUser[user.ID]))
	}

	totalPages := 0
	if total > 0 {
		totalPages = int((total + int64(filter.Limit) - 1) / int64(filter.Limit))
	}

	return &dto.UserListResponse{
		Users: items,
		Pagination: dto.PaginationResponse{
			Page:       filter.Page,
			Limit:      filter.Limit,
			Total:      total,
			TotalPages: totalPages,
		},
	}, nil
}

// Get mengambil detail satu user beserta role-nya.
func (s *UserService) Get(ctx context.Context, userID int64) (*dto.UserAdminResponse, error) {
	user, err := s.users.FindByID(userID)
	if err != nil {
		if errors.Is(err, repositories.ErrNotFound) {
			return nil, utils.ErrUserNotFound
		}
		return nil, utils.ErrInternal
	}
	roles, err := s.users.GetRoles(userID)
	if err != nil {
		return nil, utils.ErrInternal
	}
	resp := toUserAdminResponse(user, roles)
	return &resp, nil
}

// Create membuat akun staf baru tanpa password/Google, lalu memberikan role
// assignable dalam satu transaksi.
func (s *UserService) Create(ctx context.Context, req dto.CreateUserRequest) (*dto.UserAdminResponse, error) {
	name := strings.TrimSpace(req.Name)
	if err := validateName(name); err != nil {
		return nil, err
	}
	email := normalizeEmail(req.Email)
	if err := validateEmail(email); err != nil {
		return nil, err
	}
	username, err := normalizeOptionalUsername(req.Username)
	if err != nil {
		return nil, err
	}
	roles, err := s.requireAssignableRoles(req.Roles, true)
	if err != nil {
		return nil, err
	}

	if exists, err := s.users.ExistsByEmail(email); err != nil {
		return nil, utils.ErrInternal
	} else if exists {
		return nil, utils.ErrConflict.WithField("email", utils.FieldEmailTaken, "Email sudah terdaftar")
	}
	if username != nil {
		if _, err := s.users.FindByUsername(*username); err == nil {
			return nil, utils.ErrConflict.WithField("username", utils.FieldUsernameTaken, "Username sudah dipakai")
		} else if !errors.Is(err, repositories.ErrNotFound) {
			return nil, utils.ErrInternal
		}
	}

	user := &models.User{
		Name:     name,
		Email:    email,
		Username: username,
		IsActive: true,
	}

	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		txUsers := s.users.WithTx(tx)
		if err := txUsers.Create(user); err != nil {
			return err
		}
		roleIDs := make([]int16, 0, len(roles))
		for _, role := range roles {
			roleIDs = append(roleIDs, role.ID)
		}
		return txUsers.AssignRoles(user.ID, roleIDs)
	})
	if err != nil {
		if appErr := asAppError(err); appErr != nil {
			return nil, appErr
		}
		if mapped := mapUserWriteError(err); mapped != nil {
			return nil, mapped
		}
		return nil, utils.ErrInternal
	}

	return s.Get(ctx, user.ID)
}

// SetStatus mengaktifkan/menonaktifkan user. Deaktivasi mencabut seluruh sesi
// dan tidak boleh menghabiskan Admin aktif terakhir.
func (s *UserService) SetStatus(ctx context.Context, actorID, targetID int64, req dto.UpdateUserStatusRequest) (*dto.UserAdminResponse, error) {
	if req.IsActive == nil {
		return nil, validationError("is_active", utils.FieldRequired, "is_active wajib diisi")
	}
	if actorID == targetID {
		return nil, utils.ErrSelfAction
	}

	reason := ""
	if req.InactiveReason != nil {
		reason = strings.ToLower(strings.TrimSpace(*req.InactiveReason))
	}
	if !*req.IsActive {
		if reason == "" {
			return nil, validationError("inactive_reason", utils.FieldRequired, "inactive_reason wajib diisi saat menonaktifkan akun")
		}
		if !validInactiveReasons[reason] {
			return nil, validationError("inactive_reason", utils.FieldInvalidReason, "inactive_reason harus graduated, transferred, atau other")
		}
	}

	now := s.now()
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		txUsers := s.users.WithTx(tx)

		admins, err := txUsers.LockAdminUsers()
		if err != nil {
			return utils.ErrInternal
		}
		user, err := txUsers.FindByIDForUpdate(targetID)
		if err != nil {
			if errors.Is(err, repositories.ErrNotFound) {
				return utils.ErrUserNotFound
			}
			return utils.ErrInternal
		}

		if *req.IsActive {
			return txUsers.UpdateStatus(targetID, map[string]interface{}{
				"is_active":             true,
				"inactive_reason":       nil,
				"deactivated_at":        nil,
				"failed_login_attempts": 0,
				"failed_login_at":       nil,
				"locked_until":          nil,
			})
		}

		roles, err := txUsers.GetRoles(targetID)
		if err != nil {
			return utils.ErrInternal
		}
		if user.IsActive && hasRole(roles, models.RoleAdmin) && countActiveAdmins(admins) <= 1 {
			return utils.ErrLastAdmin
		}

		if err := txUsers.UpdateStatus(targetID, map[string]interface{}{
			"is_active":       false,
			"inactive_reason": reason,
			"deactivated_at":  now,
		}); err != nil {
			return utils.ErrInternal
		}
		if _, err := s.sessions.WithTx(tx).RevokeAllForUser(targetID, now); err != nil {
			return utils.ErrInternal
		}
		return nil
	})
	if err != nil {
		if appErr := asAppError(err); appErr != nil {
			return nil, appErr
		}
		return nil, utils.ErrInternal
	}

	return s.Get(ctx, targetID)
}

// ReplaceRoles mengganti role assignable user, mempertahankan role terkunci,
// serta menegakkan proteksi self-admin-removal dan last-admin.
func (s *UserService) ReplaceRoles(ctx context.Context, actorID, targetID int64, req dto.UpdateUserRolesRequest) (*dto.UserAdminResponse, error) {
	requestedRoles, err := s.requireAssignableRoles(req.Roles, false)
	if err != nil {
		return nil, err
	}
	requestedIDs := make([]int16, 0, len(requestedRoles))
	requestedHasAdmin := false
	for _, role := range requestedRoles {
		requestedIDs = append(requestedIDs, role.ID)
		if role.Name == models.RoleAdmin {
			requestedHasAdmin = true
		}
	}

	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		txUsers := s.users.WithTx(tx)

		admins, err := txUsers.LockAdminUsers()
		if err != nil {
			return utils.ErrInternal
		}
		if _, err := txUsers.FindByIDForUpdate(targetID); err != nil {
			if errors.Is(err, repositories.ErrNotFound) {
				return utils.ErrUserNotFound
			}
			return utils.ErrInternal
		}

		currentRoles, err := txUsers.GetRoles(targetID)
		if err != nil {
			return utils.ErrInternal
		}
		targetHasAdmin := hasRole(currentRoles, models.RoleAdmin)

		if actorID == targetID && targetHasAdmin && !requestedHasAdmin {
			return utils.ErrSelfAction
		}
		if targetHasAdmin && !requestedHasAdmin && len(admins) <= 1 {
			return utils.ErrLastAdmin
		}

		if err := txUsers.DeleteAssignableRoles(targetID); err != nil {
			return utils.ErrInternal
		}
		// Role terkunci tidak pernah dihapus oleh DeleteAssignableRoles, jadi
		// cukup menambahkan role assignable yang diminta.
		return txUsers.AssignRoles(targetID, requestedIDs)
	})
	if err != nil {
		if appErr := asAppError(err); appErr != nil {
			return nil, appErr
		}
		return nil, utils.ErrInternal
	}

	return s.Get(ctx, targetID)
}

// Update memperbarui name/email/username dengan optimistic locking updated_at.
// Akun siswa tidak boleh diubah lewat endpoint ini.
func (s *UserService) Update(ctx context.Context, targetID int64, req dto.UpdateUserRequest) (*dto.UserAdminResponse, error) {
	if req.UpdatedAt == nil {
		return nil, validationError("updated_at", utils.FieldRequired, "updated_at wajib diisi")
	}

	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		txUsers := s.users.WithTx(tx)

		user, err := txUsers.FindByID(targetID)
		if err != nil {
			if errors.Is(err, repositories.ErrNotFound) {
				return utils.ErrUserNotFound
			}
			return utils.ErrInternal
		}
		roles, err := txUsers.GetRoles(targetID)
		if err != nil {
			return utils.ErrInternal
		}
		if isStudent(roles) {
			return validationError("id", utils.FieldStudentLocked, "Akun siswa tidak dapat diubah lewat manajemen akun")
		}

		updates := make(map[string]interface{})
		if req.Name != nil {
			name := strings.TrimSpace(*req.Name)
			if err := validateName(name); err != nil {
				return err
			}
			updates["name"] = name
		}
		if req.Email != nil {
			email := normalizeEmail(*req.Email)
			if err := validateEmail(email); err != nil {
				return err
			}
			if email != user.Email {
				updates["email"] = email
				// Email berubah -> identitas Google lama tidak lagi valid.
				updates["google_id"] = nil
			}
		}
		if req.Username != nil {
			username, err := normalizeOptionalUsername(*req.Username)
			if err != nil {
				return err
			}
			updates["username"] = username
		}
		if len(updates) == 0 {
			return validationError("request", utils.FieldNoUpdateFields, "Tidak ada field yang diubah")
		}
		updates["updated_at"] = s.now()

		rows, err := txUsers.UpdateProfile(targetID, *req.UpdatedAt, updates)
		if err != nil {
			return err
		}
		if rows == 0 {
			return utils.ErrStaleData
		}
		return nil
	})
	if err != nil {
		if appErr := asAppError(err); appErr != nil {
			return nil, appErr
		}
		if mapped := mapUserWriteError(err); mapped != nil {
			return nil, mapped
		}
		return nil, utils.ErrInternal
	}

	return s.Get(ctx, targetID)
}

// Delete menghapus akun staf. Akun siswa, akun sendiri, dan Admin terakhir
// dilindungi. Riwayat relasi FK ON DELETE RESTRICT dipetakan ke 409.
func (s *UserService) Delete(ctx context.Context, actorID, targetID int64) error {
	if actorID == targetID {
		return utils.ErrSelfAction
	}

	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		txUsers := s.users.WithTx(tx)

		admins, err := txUsers.LockAdminUsers()
		if err != nil {
			return utils.ErrInternal
		}
		if _, err := txUsers.FindByIDForUpdate(targetID); err != nil {
			if errors.Is(err, repositories.ErrNotFound) {
				return utils.ErrUserNotFound
			}
			return utils.ErrInternal
		}

		roles, err := txUsers.GetRoles(targetID)
		if err != nil {
			return utils.ErrInternal
		}
		if isStudent(roles) {
			return validationError("id", utils.FieldStudentLocked, "Akun siswa tidak dapat dihapus lewat manajemen akun")
		}
		if hasRole(roles, models.RoleAdmin) && len(admins) <= 1 {
			return utils.ErrLastAdmin
		}

		rows, err := txUsers.Delete(targetID)
		if err != nil {
			return err
		}
		if rows == 0 {
			return utils.ErrUserNotFound
		}
		return nil
	})
	if err != nil {
		if appErr := asAppError(err); appErr != nil {
			return appErr
		}
		if isForeignKeyViolation(err) {
			return utils.ErrAccountHasHistory
		}
		return utils.ErrInternal
	}
	return nil
}

// requireAssignableRoles menormalisasi nama role, memastikan semuanya ada dan
// is_assignable=true. requireOne dipakai POST /users.
func (s *UserService) requireAssignableRoles(names []string, requireOne bool) ([]models.Role, error) {
	normalized := normalizeRoleNames(names)
	if len(normalized) == 0 {
		if requireOne {
			return nil, validationError("roles", utils.FieldRequired, "roles minimal berisi satu role")
		}
		return []models.Role{}, nil
	}

	roles, err := s.roles.FindByNames(normalized)
	if err != nil {
		return nil, utils.ErrInternal
	}
	if len(roles) != len(normalized) {
		return nil, validationError("roles", utils.FieldRoleNotFound, "Ada role yang tidak dikenal")
	}
	for _, role := range roles {
		if !role.IsAssignable {
			return nil, validationError("roles", utils.FieldRoleNotAssignable, "Role "+role.Name+" tidak dapat diberikan lewat endpoint ini")
		}
	}
	return roles, nil
}

// normalizeRoleNames trim, lowercase, dan membuang duplikat dengan urutan stabil.
func normalizeRoleNames(names []string) []string {
	seen := make(map[string]bool, len(names))
	result := make([]string, 0, len(names))
	for _, name := range names {
		value := strings.ToLower(strings.TrimSpace(name))
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		result = append(result, value)
	}
	return result
}

// normalizeEmail menormalkan email ke lowercase tanpa spasi tepi.
func normalizeEmail(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

// normalizeOptionalUsername mengembalikan nil untuk string kosong.
func normalizeOptionalUsername(value string) (*string, error) {
	username := strings.ToLower(strings.TrimSpace(value))
	if username == "" {
		return nil, nil
	}
	if !utils.IsValidUsername(username) {
		return nil, validationError("username", utils.FieldInvalidUsername, "Username 3-30 karakter, hanya huruf kecil, angka, titik, dan underscore")
	}
	return &username, nil
}

// validateName memvalidasi nama setelah trim.
func validateName(name string) error {
	if name == "" {
		return validationError("name", utils.FieldRequired, "name wajib diisi")
	}
	count := len([]rune(name))
	if count < 2 {
		return validationError("name", utils.FieldTooShort, "name minimal 2 karakter")
	}
	if count > 150 {
		return validationError("name", utils.FieldTooLong, "name maksimal 150 karakter")
	}
	return nil
}

// validateEmail memvalidasi email setelah normalisasi.
func validateEmail(email string) error {
	if email == "" {
		return validationError("email", utils.FieldRequired, "email wajib diisi")
	}
	if len(email) > 255 {
		return validationError("email", utils.FieldTooLong, "email maksimal 255 karakter")
	}
	if !utils.IsValidEmail(email) {
		return validationError("email", utils.FieldInvalidEmail, "email harus berupa email yang valid")
	}
	return nil
}

// validationError membuat error 422 dengan satu detail field.
func validationError(field, code, message string) *utils.AppError {
	return utils.NewValidationError([]utils.FieldError{{
		Field:   field,
		Code:    code,
		Message: message,
	}})
}

// asAppError mengambil *AppError dari error berlapis.
func asAppError(err error) *utils.AppError {
	var appErr *utils.AppError
	if errors.As(err, &appErr) {
		return appErr
	}
	return nil
}

// mapUserWriteError memetakan unique violation PostgreSQL ke 409 yang ramah.
func mapUserWriteError(err error) *utils.AppError {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "23505" {
		return nil
	}
	switch pgErr.ConstraintName {
	case "uq_users_email_lower":
		return utils.ErrConflict.WithField("email", utils.FieldEmailTaken, "Email sudah terdaftar")
	case "uq_users_username":
		return utils.ErrConflict.WithField("username", utils.FieldUsernameTaken, "Username sudah dipakai")
	default:
		return utils.ErrConflict.WithMessage("Email atau username sudah dipakai")
	}
}

// isForeignKeyViolation memetakan FK saat delete ke ACCOUNT_HAS_HISTORY.
// PostgreSQL memakai 23503 untuk foreign_key_violation dan 23001 untuk
// restrict_violation (hasil ON DELETE RESTRICT).
func isForeignKeyViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && (pgErr.Code == "23503" || pgErr.Code == "23001")
}

// hasRole memeriksa keberadaan role berdasarkan nama.
func hasRole(roles []models.Role, name string) bool {
	for _, role := range roles {
		if role.Name == name {
			return true
		}
	}
	return false
}

// isStudent menganggap akun siswa bila punya role berkategori student.
func isStudent(roles []models.Role) bool {
	for _, role := range roles {
		if role.Category == "student" || role.Name == models.RoleSiswa || role.Name == models.RoleSiswaInti {
			return true
		}
	}
	return false
}

// countActiveAdmins menghitung Admin aktif dari baris yang sudah dikunci.
func countActiveAdmins(admins []repositories.AdminLock) int {
	count := 0
	for _, admin := range admins {
		if admin.IsActive {
			count++
		}
	}
	return count
}

// toUserAdminResponse memetakan model ke DTO admin yang tidak membocorkan
// password_hash/google_id.
func toUserAdminResponse(user *models.User, roles []models.Role) dto.UserAdminResponse {
	roleItems := make([]dto.UserRoleSummary, 0, len(roles))
	for _, role := range roles {
		roleItems = append(roleItems, dto.UserRoleSummary{
			ID:           role.ID,
			Name:         role.Name,
			Label:        role.Label,
			Category:     role.Category,
			IsAssignable: role.IsAssignable,
			Description:  role.Description,
		})
	}
	return dto.UserAdminResponse{
		ID:             user.ID,
		Name:           user.Name,
		Email:          user.Email,
		Username:       user.Username,
		NISN:           user.NISN,
		AvatarURL:      user.AvatarURL,
		IsActive:       user.IsActive,
		InactiveReason: user.InactiveReason,
		DeactivatedAt:  user.DeactivatedAt,
		CreatedAt:      user.CreatedAt,
		UpdatedAt:      user.UpdatedAt,
		Roles:          roleItems,
	}
}
