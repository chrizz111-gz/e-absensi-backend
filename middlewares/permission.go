package middlewares

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"e-absensi-backend/repositories"
	"e-absensi-backend/utils"
)

// RequirePermission membatasi akses endpoint berdasarkan permissionCode yang
// dimiliki minimal salah satu role user. Middleware ini HARUS dipasang setelah
// AuthRequired, karena memakai user yang sudah dimuat dari token di context.
//
// Default deny: user tanpa permission mendapat 403. Token tidak valid atau
// akun nonaktif sudah ditolak AuthRequired dengan 401.
func RequirePermission(db *gorm.DB, permissionCode string) gin.HandlerFunc {
	return requirePermission(repositories.NewPermissionRepository(db), permissionCode)
}

// permissionChecker memisahkan middleware dari GORM agar bisa diuji dengan fake.
type permissionChecker interface {
	UserHasPermission(userID int64, permissionCode string) (bool, error)
}

// requirePermission menyusun handler dengan checker yang bisa diganti pada test.
func requirePermission(checker permissionChecker, permissionCode string) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, ok := CurrentUserID(c)
		if !ok {
			utils.AbortWithError(c, http.StatusUnauthorized, utils.CodeUnauthorized, "Autentikasi diperlukan atau token tidak valid")
			return
		}

		allowed, err := checker.UserHasPermission(userID, permissionCode)
		if err != nil {
			utils.AbortWithError(c, http.StatusInternalServerError, utils.CodeInternal, "Terjadi kesalahan pada server")
			return
		}
		if !allowed {
			utils.AbortWithError(c, http.StatusForbidden, utils.CodeForbidden, "Anda tidak memiliki hak akses")
			return
		}

		c.Next()
	}
}
