package middlewares

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"e-absensi-backend/models"
	"e-absensi-backend/repositories"
	"e-absensi-backend/utils"
)

// Key konteks Gin untuk data autentikasi.
const (
	ContextUserKey  = "auth_user"
	ContextRolesKey = "auth_roles"
)

// AuthRequired memverifikasi access token pada header Authorization, lalu
// memuat ULANG user dan seluruh role (union) dari database. Otorisasi tidak
// pernah mempercayai klaim role dari token/frontend (F1.8).
func AuthRequired(db *gorm.DB) gin.HandlerFunc {
	users := repositories.NewUserRepository(db)

	return func(c *gin.Context) {
		token, ok := bearerToken(c)
		if !ok {
			utils.AbortWithError(c, http.StatusUnauthorized, utils.CodeUnauthorized, "Header Authorization (Bearer) tidak ditemukan")
			return
		}

		claims, err := utils.ParseAccessToken(token)
		if err != nil {
			if errors.Is(err, utils.ErrExpiredToken) {
				utils.AbortWithError(c, http.StatusUnauthorized, utils.CodeTokenExpired, "Token telah kedaluwarsa")
				return
			}
			utils.AbortWithError(c, http.StatusUnauthorized, utils.CodeUnauthorized, "Token tidak valid")
			return
		}

		user, err := users.FindByID(claims.UserID)
		if err != nil {
			utils.AbortWithError(c, http.StatusUnauthorized, utils.CodeUnauthorized, "Akun tidak ditemukan")
			return
		}
		if !user.IsActive {
			utils.AbortWithError(c, http.StatusForbidden, utils.CodeAccountInactive, "Akun tidak aktif, hubungi admin")
			return
		}

		roles, err := users.GetRoles(user.ID)
		if err != nil {
			utils.AbortWithError(c, http.StatusInternalServerError, utils.CodeInternal, "Terjadi kesalahan pada server")
			return
		}

		c.Set(ContextUserKey, user)
		c.Set(ContextRolesKey, roleNameList(roles))
		c.Next()
	}
}

// bearerToken mengekstrak token dari header "Authorization: Bearer <token>".
func bearerToken(c *gin.Context) (string, bool) {
	header := c.GetHeader("Authorization")
	if header == "" {
		return "", false
	}
	parts := strings.SplitN(header, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return "", false
	}
	token := strings.TrimSpace(parts[1])
	return token, token != ""
}

// roleNameList mengekstrak nama role dari daftar model role.
func roleNameList(roles []models.Role) []string {
	names := make([]string, 0, len(roles))
	for _, role := range roles {
		names = append(names, role.Name)
	}
	return names
}

// CurrentUser mengambil user terautentikasi dari context.
func CurrentUser(c *gin.Context) (*models.User, bool) {
	value, exists := c.Get(ContextUserKey)
	if !exists {
		return nil, false
	}
	user, ok := value.(*models.User)
	return user, ok
}

// CurrentUserID mengambil id user terautentikasi dari context.
func CurrentUserID(c *gin.Context) (int64, bool) {
	user, ok := CurrentUser(c)
	if !ok {
		return 0, false
	}
	return user.ID, true
}

// CurrentRoles mengambil daftar role (union) user terautentikasi dari context.
func CurrentRoles(c *gin.Context) ([]string, bool) {
	value, exists := c.Get(ContextRolesKey)
	if !exists {
		return nil, false
	}
	roles, ok := value.([]string)
	return roles, ok
}
