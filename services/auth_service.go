package services

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"e-absensi-backend/config"
	"e-absensi-backend/dto"
	"e-absensi-backend/models"
	"e-absensi-backend/repositories"
	"e-absensi-backend/utils"
)

// ClientMeta membawa informasi device untuk pencatatan sesi.
type ClientMeta struct {
	IP         string
	UserAgent  string
	DeviceInfo string
}

// AuthService menangani registrasi, login, refresh, dan logout (F1.1-F1.3, F1.7).
type AuthService struct {
	db       *gorm.DB
	users    *repositories.UserRepository
	roles    *repositories.RoleRepository
	sessions *repositories.SessionRepository
	google   utils.GoogleVerifier
	now      func() time.Time
}

// NewAuthService membuat AuthService baru. google boleh nil (fitur Google
// Sign-In nonaktif bila GOOGLE_CLIENT_ID kosong).
func NewAuthService(db *gorm.DB, google utils.GoogleVerifier) *AuthService {
	return &AuthService{
		db:       db,
		users:    repositories.NewUserRepository(db),
		roles:    repositories.NewRoleRepository(db),
		sessions: repositories.NewSessionRepository(db),
		google:   google,
		now:      time.Now,
	}
}

// RegisterStudent mendaftarkan akun siswa baru (F1.1). Satu transaksi:
// simpan users + user_roles (role "siswa") + students. Email/NISN/username harus unik.
func (s *AuthService) RegisterStudent(ctx context.Context, req dto.RegisterStudentRequest) (*dto.UserResponse, error) {
	email := strings.ToLower(strings.TrimSpace(req.Email))
	nisn := strings.TrimSpace(req.NISN)

	emailExists, err := s.users.ExistsByEmail(email)
	if err != nil {
		return nil, utils.ErrInternal
	}
	if emailExists {
		return nil, utils.ErrConflict.WithField("email", utils.FieldEmailTaken, "Email sudah terdaftar")
	}

	nisnExists, err := s.users.ExistsByNISN(nisn)
	if err != nil {
		return nil, utils.ErrInternal
	}
	if nisnExists {
		return nil, utils.ErrConflict.WithField("nisn", utils.FieldNISNTaken, "NISN sudah terdaftar")
	}

	var username *string
	if trimmed := strings.ToLower(strings.TrimSpace(req.Username)); trimmed != "" {
		if _, err := s.users.FindByUsername(trimmed); err == nil {
			return nil, utils.ErrConflict.WithField("username", utils.FieldUsernameTaken, "Username sudah dipakai")
		} else if !errors.Is(err, repositories.ErrNotFound) {
			return nil, utils.ErrInternal
		}
		username = &trimmed
	}

	hash, err := utils.HashPassword(req.Password)
	if err != nil {
		return nil, utils.ErrInternal
	}

	roleSiswa, err := s.roles.FindByName(models.RoleSiswa)
	if err != nil {
		return nil, utils.ErrInternal
	}

	user := &models.User{
		Name:         strings.TrimSpace(req.Name),
		Email:        email,
		Username:     username,
		PasswordHash: &hash,
		NISN:         &nisn,
		IsActive:     true,
	}

	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		txUsers := s.users.WithTx(tx)
		if err := txUsers.Create(user); err != nil {
			return err
		}
		if err := txUsers.AssignRole(user.ID, roleSiswa.ID); err != nil {
			return err
		}
		student := models.Student{
			UserID: user.ID,
			NISN:   nisn,
		}
		return tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "user_id"}},
			DoNothing: true,
		}).Create(&student).Error
	})
	if err != nil {
		if mapped := mapRegisterWriteError(err); mapped != nil {
			return nil, mapped
		}
		return nil, utils.ErrInternal
	}

	return &dto.UserResponse{
		ID:        user.ID,
		Name:      user.Name,
		Email:     user.Email,
		Username:  user.Username,
		NISN:      user.NISN,
		AvatarURL: user.AvatarURL,
		IsActive:  user.IsActive,
		Roles:     []string{roleSiswa.Name},
	}, nil
}

// Login memproses login manual (F1.2) dengan proteksi brute-force (lockout).
func (s *AuthService) Login(ctx context.Context, req dto.LoginRequest, meta ClientMeta) (*dto.AuthResponse, error) {
	now := s.now()

	user, err := s.users.FindByIdentifier(req.Identifier)
	if err != nil {
		if errors.Is(err, repositories.ErrNotFound) {
			// Pesan generik: jangan bocorkan keberadaan akun.
			return nil, utils.ErrInvalidCredentials
		}
		return nil, utils.ErrInternal
	}

	if !user.IsActive {
		return nil, utils.ErrAccountInactive
	}

	if IsLocked(user, now) {
		return nil, utils.ErrAccountLocked.WithMessage(lockMessage(user, now))
	}

	if user.PasswordHash == nil || !utils.CheckPasswordHash(req.Password, *user.PasswordHash) {
		locked := ApplyFailedAttempt(user, now, config.MaxFailedLoginAttempts, config.LoginAttemptWindow, config.AccountLockDuration)
		if err := s.users.UpdateLoginFailure(user); err != nil {
			return nil, utils.ErrInternal
		}
		if locked {
			return nil, utils.ErrAccountLocked.WithMessage(lockMessage(user, now))
		}
		return nil, utils.ErrInvalidCredentials
	}

	if err := s.users.ResetLoginFailure(user.ID); err != nil {
		return nil, utils.ErrInternal
	}
	ResetLockout(user)

	roles, err := s.users.GetRoles(user.ID)
	if err != nil {
		return nil, utils.ErrInternal
	}

	return s.issueSession(ctx, user, roles, meta, now)
}

// GoogleLogin memproses login via Google Sign-In (F1.3) hanya dengan idToken.
func (s *AuthService) GoogleLogin(ctx context.Context, idToken string, meta ClientMeta) (*dto.AuthResponse, error) {
	return s.GoogleLoginWithInput(ctx, dto.GoogleLoginRequest{
		IDToken:    idToken,
		DeviceInfo: meta.DeviceInfo,
	}, meta)
}

// GoogleLoginWithInput memproses login & registrasi mandiri siswa via Google Sign-In (F1.3).
// Alur:
//   - akun dengan google_id cocok -> langsung login;
//   - belum ada -> dikaitkan ke akun yang emailnya sama (email harus terverifikasi);
//   - email belum terdaftar & NISN dikirim -> registrasi siswa baru (auto-fill nama & email);
//   - email belum terdaftar & NISN kosong -> kembalikan 404 beserta data profil Google.
func (s *AuthService) GoogleLoginWithInput(ctx context.Context, req dto.GoogleLoginRequest, meta ClientMeta) (*dto.AuthResponse, error) {
	if s.google == nil {
		return nil, utils.ErrGoogleDisabled
	}

	claims, err := s.google.Verify(ctx, req.IDToken)
	if err != nil {
		return nil, utils.ErrGoogleTokenInvalid
	}
	if !claims.EmailVerified {
		return nil, utils.ErrGoogleEmailUnverified
	}

	now := s.now()

	user, err := s.users.FindByGoogleID(claims.Subject)
	if err != nil && !errors.Is(err, repositories.ErrNotFound) {
		return nil, utils.ErrInternal
	}

	needLink := false
	if errors.Is(err, repositories.ErrNotFound) {
		user, err = s.users.FindByEmail(claims.Email)
		if err != nil {
			if errors.Is(err, repositories.ErrNotFound) {
				nisn := strings.TrimSpace(req.NISN)
				if nisn == "" {
					return nil, utils.ErrAccountNotFound.WithData(dto.GoogleProfileData{
						Email:     strings.ToLower(strings.TrimSpace(claims.Email)),
						Name:      strings.TrimSpace(claims.Name),
						AvatarURL: strings.TrimSpace(claims.Picture),
					})
				}
				return s.registerGoogleStudent(ctx, claims, req, meta, now)
			}
			return nil, utils.ErrInternal
		}
		needLink = true
	}

	if !user.IsActive {
		return nil, utils.ErrAccountInactive
	}

	if IsLocked(user, now) {
		return nil, utils.ErrAccountLocked.WithMessage(lockMessage(user, now))
	}

	if needLink {
		// Email sudah terdaftar: kaitkan akun Google ke user tersebut.
		if err := s.users.LinkGoogleIdentity(user.ID, claims.Subject, claims.Picture); err != nil {
			return nil, utils.ErrInternal
		}
	}

	if user.FailedLoginAttempts > 0 || user.FailedLoginAt != nil || user.LockedUntil != nil {
		if err := s.users.ResetLoginFailure(user.ID); err != nil {
			return nil, utils.ErrInternal
		}
		ResetLockout(user)
	}

	roles, err := s.users.GetRoles(user.ID)
	if err != nil {
		return nil, utils.ErrInternal
	}

	return s.issueSession(ctx, user, roles, meta, now)
}

// registerGoogleStudent membuat akun siswa baru dari klaim Google + NISN,
// membuat profil students, lalu langsung menerbitkan sesi login.
func (s *AuthService) registerGoogleStudent(ctx context.Context, claims *utils.GoogleClaims, req dto.GoogleLoginRequest, meta ClientMeta, now time.Time) (*dto.AuthResponse, error) {
	nisn := strings.TrimSpace(req.NISN)
	nisnExists, err := s.users.ExistsByNISN(nisn)
	if err != nil {
		return nil, utils.ErrInternal
	}
	if nisnExists {
		return nil, utils.ErrConflict.WithField("nisn", utils.FieldNISNTaken, "NISN sudah terdaftar")
	}

	name := strings.TrimSpace(req.Name)
	if name == "" {
		name = strings.TrimSpace(claims.Name)
	}
	if len([]rune(name)) < 2 {
		name = strings.Split(claims.Email, "@")[0]
	}

	email := strings.ToLower(strings.TrimSpace(claims.Email))
	googleID := claims.Subject
	var avatarURL *string
	if pic := strings.TrimSpace(claims.Picture); pic != "" {
		avatarURL = &pic
	}

	roleSiswa, err := s.roles.FindByName(models.RoleSiswa)
	if err != nil {
		return nil, utils.ErrInternal
	}

	user := &models.User{
		Name:      name,
		Email:     email,
		NISN:      &nisn,
		GoogleID:  &googleID,
		AvatarURL: avatarURL,
		IsActive:  true,
	}

	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		txUsers := s.users.WithTx(tx)
		if err := txUsers.Create(user); err != nil {
			return err
		}
		if err := txUsers.AssignRole(user.ID, roleSiswa.ID); err != nil {
			return err
		}
		student := models.Student{
			UserID: user.ID,
			NISN:   nisn,
		}
		return tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "user_id"}},
			DoNothing: true,
		}).Create(&student).Error
	})
	if err != nil {
		if mapped := mapRegisterWriteError(err); mapped != nil {
			return nil, mapped
		}
		return nil, utils.ErrInternal
	}

	return s.issueSession(ctx, user, []models.Role{*roleSiswa}, meta, now)
}

// Refresh menukar refresh token dengan pasangan token baru (rotasi). Token lama
// digantikan sehingga pemakaian ulang token lama otomatis ditolak.
func (s *AuthService) Refresh(ctx context.Context, refreshToken string, meta ClientMeta) (*dto.AuthResponse, error) {
	now := s.now()

	claims, err := utils.ParseRefreshToken(refreshToken)
	if err != nil {
		if errors.Is(err, utils.ErrExpiredToken) {
			return nil, utils.ErrTokenExpired
		}
		return nil, utils.ErrSessionRevoked
	}

	session, err := s.sessions.FindByTokenHash(utils.HashToken(refreshToken))
	if err != nil {
		// Tidak ditemukan: kemungkinan token sudah dirotasi/dicabut.
		return nil, utils.ErrSessionRevoked
	}

	if session.RevokedAt != nil || !session.ExpiredAt.After(now) || session.UserID != claims.UserID {
		return nil, utils.ErrSessionRevoked
	}

	user, err := s.users.FindByID(session.UserID)
	if err != nil {
		return nil, utils.ErrSessionRevoked
	}
	if !user.IsActive {
		return nil, utils.ErrAccountInactive
	}

	roles, err := s.users.GetRoles(user.ID)
	if err != nil {
		return nil, utils.ErrInternal
	}

	newRefresh, err := utils.GenerateRefreshToken(user.ID, uuid.NewString())
	if err != nil {
		return nil, utils.ErrInternal
	}
	if err := s.sessions.RotateToken(session.ID, utils.HashToken(newRefresh), now); err != nil {
		return nil, utils.ErrInternal
	}

	accessToken, err := utils.GenerateAccessToken(user.ID, user.Email, roleNames(roles))
	if err != nil {
		return nil, utils.ErrInternal
	}

	return &dto.AuthResponse{
		User:         toUserResponse(user, roles),
		AccessToken:  accessToken,
		RefreshToken: newRefresh,
		TokenType:    "Bearer",
		ExpiresIn:    int(utils.AccessTokenTTL().Seconds()),
	}, nil
}

// Logout mencabut sesi pada device yang mengirim refresh token ini (F1.3).
// Bersifat idempotent: token tak dikenal tetap dianggap berhasil.
func (s *AuthService) Logout(ctx context.Context, refreshToken string, userID int64) error {
	if _, err := utils.ParseRefreshToken(refreshToken); err != nil {
		return nil
	}

	session, err := s.sessions.FindByTokenHash(utils.HashToken(refreshToken))
	if err != nil || session.UserID != userID {
		return nil
	}
	return s.sessions.Revoke(session.ID, s.now())
}

// LogoutAll mencabut SELURUH sesi aktif user (force logout semua device).
func (s *AuthService) LogoutAll(ctx context.Context, userID int64) (int64, error) {
	return s.sessions.RevokeAllForUser(userID, s.now())
}

// issueSession membuat baris sesi baru lalu menerbitkan access + refresh token.
func (s *AuthService) issueSession(ctx context.Context, user *models.User, roles []models.Role, meta ClientMeta, now time.Time) (*dto.AuthResponse, error) {
	refreshToken, err := utils.GenerateRefreshToken(user.ID, uuid.NewString())
	if err != nil {
		return nil, utils.ErrInternal
	}

	session := &models.Session{
		UserID:           user.ID,
		RefreshTokenHash: utils.HashToken(refreshToken),
		ExpiredAt:        now.Add(utils.RefreshTokenTTL()),
	}
	if v := strings.TrimSpace(meta.DeviceInfo); v != "" {
		session.DeviceInfo = &v
	}
	if v := strings.TrimSpace(meta.UserAgent); v != "" {
		session.UserAgent = &v
	}
	if v := strings.TrimSpace(meta.IP); v != "" {
		session.IPAddress = &v
	}

	if err := s.sessions.Create(session); err != nil {
		return nil, utils.ErrInternal
	}

	accessToken, err := utils.GenerateAccessToken(user.ID, user.Email, roleNames(roles))
	if err != nil {
		return nil, utils.ErrInternal
	}

	return &dto.AuthResponse{
		User:         toUserResponse(user, roles),
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		TokenType:    "Bearer",
		ExpiresIn:    int(utils.AccessTokenTTL().Seconds()),
	}, nil
}

// mapRegisterWriteError memetakan unique violation PostgreSQL saat registrasi
// ke 409 Conflict yang spesifik per field.
func mapRegisterWriteError(err error) *utils.AppError {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "23505" {
		return nil
	}
	switch pgErr.ConstraintName {
	case "uq_users_email_lower":
		return utils.ErrConflict.WithField("email", utils.FieldEmailTaken, "Email sudah terdaftar")
	case "uq_users_nisn", "uq_students_nisn":
		return utils.ErrConflict.WithField("nisn", utils.FieldNISNTaken, "NISN sudah terdaftar")
	case "uq_users_username":
		return utils.ErrConflict.WithField("username", utils.FieldUsernameTaken, "Username sudah dipakai")
	default:
		return utils.ErrConflict
	}
}

// lockMessage menyusun pesan sisa waktu lock (minimal 1 menit).
func lockMessage(user *models.User, now time.Time) string {
	remaining := LockRemaining(user, now)
	minutes := int(remaining.Round(time.Minute).Minutes())
	if minutes < 1 {
		minutes = 1
	}
	return fmt.Sprintf("Akun terkunci sementara karena percobaan login berlebih. Coba lagi dalam %d menit", minutes)
}

// roleNames mengekstrak nama-nama role dari daftar role user (union).
func roleNames(roles []models.Role) []string {
	names := make([]string, 0, len(roles))
	for _, role := range roles {
		names = append(names, role.Name)
	}
	return names
}

// toUserResponse memetakan model user + role ke DTO yang aman.
func toUserResponse(user *models.User, roles []models.Role) dto.UserResponse {
	return dto.UserResponse{
		ID:        user.ID,
		Name:      user.Name,
		Email:     user.Email,
		Username:  user.Username,
		NISN:      user.NISN,
		AvatarURL: user.AvatarURL,
		IsActive:  user.IsActive,
		Roles:     roleNames(roles),
	}
}
