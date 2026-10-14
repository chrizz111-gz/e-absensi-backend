package controllers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"e-absensi-backend/dto"
	"e-absensi-backend/middlewares"
	"e-absensi-backend/services"
	"e-absensi-backend/utils"
)

// OTPController menangani endpoint OTP ganti & reset password.
type OTPController struct {
	otp       *services.OTPService
	passwords *services.PasswordService
}

// NewOTPController membuat OTPController baru.
func NewOTPController(otp *services.OTPService, passwords *services.PasswordService) *OTPController {
	return &OTPController{otp: otp, passwords: passwords}
}

// RequestChangePasswordOTP menangani POST change-password/request & /resend (F1.4/F1.6).
func (h *OTPController) RequestChangePasswordOTP(c *gin.Context) {
	userID, ok := middlewares.CurrentUserID(c)
	if !ok {
		c.Error(utils.ErrUnauthorized)
		return
	}

	if err := h.otp.RequestChangePasswordOTP(c.Request.Context(), userID); err != nil {
		c.Error(err)
		return
	}

	utils.SuccessResponse(c, http.StatusOK, utils.CodeOTPRequested, "Kode OTP telah dikirim ke email Anda", nil)
}

// ChangePassword menangani POST change-password/verify (F1.4).
func (h *OTPController) ChangePassword(c *gin.Context) {
	userID, ok := middlewares.CurrentUserID(c)
	if !ok {
		c.Error(utils.ErrUnauthorized)
		return
	}

	var req dto.ChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(utils.NewValidationError(utils.FormatValidationErrors(err)))
		return
	}

	if err := h.passwords.ChangePassword(c.Request.Context(), userID, req.Code, req.NewPassword); err != nil {
		c.Error(err)
		return
	}

	utils.SuccessResponse(c, http.StatusOK, utils.CodePasswordChanged, "Password berhasil diubah. Semua sesi dihentikan, silakan login ulang.", nil)
}

// RequestResetPasswordOTP menangani POST forgot-password & /resend (F1.5/F1.6). Publik.
func (h *OTPController) RequestResetPasswordOTP(c *gin.Context) {
	var req dto.ForgotPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(utils.NewValidationError(utils.FormatValidationErrors(err)))
		return
	}

	if err := h.otp.RequestResetPasswordOTP(c.Request.Context(), req.Email); err != nil {
		c.Error(err)
		return
	}

	utils.SuccessResponse(c, http.StatusOK, utils.CodeOTPRequested, "Jika email terdaftar, kode OTP telah dikirim", nil)
}

// ResetPassword menangani POST forgot-password/verify (F1.5). Publik.
func (h *OTPController) ResetPassword(c *gin.Context) {
	var req dto.ResetPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(utils.NewValidationError(utils.FormatValidationErrors(err)))
		return
	}

	if err := h.passwords.ResetPassword(c.Request.Context(), req.Email, req.Code, req.NewPassword); err != nil {
		c.Error(err)
		return
	}

	utils.SuccessResponse(c, http.StatusOK, utils.CodePasswordChanged, "Password berhasil direset. Semua sesi dihentikan, silakan login ulang.", nil)
}
