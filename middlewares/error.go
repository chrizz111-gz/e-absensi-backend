package middlewares

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"e-absensi-backend/utils"
)

// ErrorHandler menerjemahkan error yang di-set controller (lewat c.Error(err))
// menjadi envelope JSON standar. Setiap controller cukup memanggil
// c.Error(err); return pada jalur gagal.
func ErrorHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()

		if len(c.Errors) == 0 {
			return
		}

		err := c.Errors.Last().Err

		var appErr *utils.AppError
		if errors.As(err, &appErr) {
			if len(appErr.Errors) > 0 {
				utils.ErrorResponseWithDetails(c, appErr.HTTPStatus, appErr.Code, appErr.Message, appErr.Errors)
				return
			}
			utils.ErrorResponse(c, appErr.HTTPStatus, appErr.Code, appErr.Message)
			return
		}

		utils.ErrorResponse(c, http.StatusInternalServerError, utils.CodeInternal, "Terjadi kesalahan pada server")
	}
}
