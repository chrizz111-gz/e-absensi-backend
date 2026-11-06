package middlewares

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"e-absensi-backend/dto"
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
			setRetryAfter(c, appErr.Data)
			utils.ErrorResponseWithDetails(c, appErr.HTTPStatus, appErr.Code, appErr.Message, appErr.Errors, appErr.Data)
			return
		}

		utils.ErrorResponse(c, http.StatusInternalServerError, utils.CodeInternal, "Terjadi kesalahan pada server")
	}
}

// setRetryAfter menambahkan header Retry-After bila error membawa data cooldown.
func setRetryAfter(c *gin.Context, data interface{}) {
	seconds := 0
	switch v := data.(type) {
	case dto.RetryAfterData:
		seconds = v.RetryAfterSeconds
	case *dto.RetryAfterData:
		if v != nil {
			seconds = v.RetryAfterSeconds
		}
	}
	if seconds > 0 {
		c.Header("Retry-After", strconv.Itoa(seconds))
	}
}
