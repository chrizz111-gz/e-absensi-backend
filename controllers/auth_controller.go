package controllers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"e-absensi-backend/dto"
	"e-absensi-backend/middlewares"
	"e-absensi-backend/services"
	"e-absensi-backend/utils"
)

// AuthController menangani endpoint registrasi, login, refresh, dan logout.
type AuthController struct {
	auth *services.AuthService
}

// NewAuthController membuat AuthController baru.
func NewAuthController(auth *services.AuthService) *AuthController {
	return &AuthController{auth: auth}
}

// Register menangani POST /api/v1/auth/register (F1.1 - registrasi siswa).
func (h *AuthController) Register(c *gin.Context) {
	var req dto.RegisterStudentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(utils.NewValidationError(utils.FormatValidationErrors(err)))
		return
	}

	user, err := h.auth.RegisterStudent(c.Request.Context(), req)
	if err != nil {
		c.Error(err)
		return
	}

	utils.SuccessResponse(c, http.StatusCreated, utils.CodeRegisterSuccess, "Registrasi berhasil", user)
}

// Login menangani POST /api/v1/auth/login (F1.2 - login manual).
func (h *AuthController) Login(c *gin.Context) {
	var req dto.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(utils.NewValidationError(utils.FormatValidationErrors(err)))
		return
	}

	result, err := h.auth.Login(c.Request.Context(), req, clientMeta(c, req.DeviceInfo))
	if err != nil {
		c.Error(err)
		return
	}

	utils.SuccessResponse(c, http.StatusOK, utils.CodeLoginSuccess, "Login berhasil", result)
}

// GoogleLogin menangani POST /api/v1/auth/google (F1.3 - login via Google).
func (h *AuthController) GoogleLogin(c *gin.Context) {
	var req dto.GoogleLoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(utils.NewValidationError(utils.FormatValidationErrors(err)))
		return
	}

	result, err := h.auth.GoogleLogin(c.Request.Context(), req.IDToken, clientMeta(c, req.DeviceInfo))
	if err != nil {
		c.Error(err)
		return
	}

	utils.SuccessResponse(c, http.StatusOK, utils.CodeGoogleLoginSuccess, "Login Google berhasil", result)
}

// Refresh menangani POST /api/v1/auth/refresh (rotasi token).
func (h *AuthController) Refresh(c *gin.Context) {
	var req dto.RefreshRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(utils.NewValidationError(utils.FormatValidationErrors(err)))
		return
	}

	result, err := h.auth.Refresh(c.Request.Context(), req.RefreshToken, clientMeta(c, ""))
	if err != nil {
		c.Error(err)
		return
	}

	utils.SuccessResponse(c, http.StatusOK, utils.CodeRefreshSuccess, "Token berhasil diperbarui", result)
}

// Logout menangani POST /api/v1/auth/logout (F1.3 - logout device ini saja).
func (h *AuthController) Logout(c *gin.Context) {
	userID, ok := middlewares.CurrentUserID(c)
	if !ok {
		c.Error(utils.ErrUnauthorized)
		return
	}

	var req dto.LogoutRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(utils.NewValidationError(utils.FormatValidationErrors(err)))
		return
	}

	if err := h.auth.Logout(c.Request.Context(), req.RefreshToken, userID); err != nil {
		c.Error(err)
		return
	}

	utils.SuccessResponse(c, http.StatusOK, utils.CodeLogoutSuccess, "Logout berhasil", nil)
}

// LogoutAll menangani POST /api/v1/auth/logout-all (force logout semua device).
func (h *AuthController) LogoutAll(c *gin.Context) {
	userID, ok := middlewares.CurrentUserID(c)
	if !ok {
		c.Error(utils.ErrUnauthorized)
		return
	}

	revoked, err := h.auth.LogoutAll(c.Request.Context(), userID)
	if err != nil {
		c.Error(err)
		return
	}

	utils.SuccessResponse(c, http.StatusOK, utils.CodeLogoutSuccess, "Semua sesi berhasil dihentikan", gin.H{"revoked_sessions": revoked})
}

// clientMeta membangun informasi device dari request.
func clientMeta(c *gin.Context, deviceInfo string) services.ClientMeta {
	return services.ClientMeta{
		IP:         c.ClientIP(),
		UserAgent:  c.Request.UserAgent(),
		DeviceInfo: deviceInfo,
	}
}
