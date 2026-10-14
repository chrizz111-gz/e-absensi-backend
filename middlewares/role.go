package middlewares

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"e-absensi-backend/utils"
)

// RequireRoles membatasi akses ke endpoint bagi user yang memiliki minimal salah
// satu role yang diizinkan (F1.8). Otorisasi memakai role union dari database
// (diisi AuthRequired), bukan klaim di token/frontend.
func RequireRoles(allowed ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		roles, ok := CurrentRoles(c)
		if !ok || !hasAnyRole(roles, allowed) {
			utils.AbortWithError(c, http.StatusForbidden, utils.CodeForbidden, "Anda tidak memiliki hak akses")
			return
		}
		c.Next()
	}
}

// hasAnyRole mengembalikan true bila roles memuat minimal satu dari allowed.
// Daftar allowed kosong berarti tidak membatasi.
func hasAnyRole(roles, allowed []string) bool {
	if len(allowed) == 0 {
		return true
	}
	for _, role := range roles {
		for _, a := range allowed {
			if role == a {
				return true
			}
		}
	}
	return false
}
