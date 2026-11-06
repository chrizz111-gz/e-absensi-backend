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

	result, err := h.otp.RequestChangePasswordOTP(c.Request.Context(), userID)
	if err != nil {
		c.Error(err)
		return
	}

	utils.SuccessResponse(c, http.StatusOK, utils.CodeOTPRequested, "Kode OTP telah dikirim ke email Anda", toOTPRequestedResponse(result))
}

// VerifyChangePasswordOTP menangani POST change-password/verify-otp (F1.4).
func (h *OTPController) VerifyChangePasswordOTP(c *gin.Context) {
	userID, ok := middlewares.CurrentUserID(c)
	if !ok {
		c.Error(utils.ErrUnauthorized)
		return
	}

	var req dto.VerifyOTPRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(utils.NewValidationError(utils.FormatValidationErrors(err)))
		return
	}

	result, err := h.passwords.VerifyChangePasswordOTP(c.Request.Context(), userID, req.Code)
	if err != nil {
		c.Error(err)
		return
	}

	utils.SuccessResponse(c, http.StatusOK, utils.CodeOTPVerified, "Kode OTP valid. Silakan buat kata sandi baru.", result)
}

// ConfirmChangePassword menangani POST change-password/confirm (F1.4).
func (h *OTPController) ConfirmChangePassword(c *gin.Context) {
	userID, ok := middlewares.CurrentUserID(c)
	if !ok {
		c.Error(utils.ErrUnauthorized)
		return
	}

	var req dto.SetPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(utils.NewValidationError(utils.FormatValidationErrors(err)))
		return
	}

	if err := h.passwords.ConfirmChangePassword(c.Request.Context(), userID, req.ResetToken, req.NewPassword); err != nil {
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

	result, err := h.otp.RequestResetPasswordOTP(c.Request.Context(), req.Email)
	if err != nil {
		c.Error(err)
		return
	}

	utils.SuccessResponse(c, http.StatusOK, utils.CodeOTPRequested, "Jika email terdaftar, kode OTP telah dikirim", toOTPRequestedResponse(result))
}

// VerifyResetPasswordOTP menangani POST forgot-password/verify-otp (F1.5). Publik.
func (h *OTPController) VerifyResetPasswordOTP(c *gin.Context) {
	var req dto.VerifyResetPasswordOTPRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(utils.NewValidationError(utils.FormatValidationErrors(err)))
		return
	}

	result, err := h.passwords.VerifyResetPasswordOTP(c.Request.Context(), req.Email, req.Code)
	if err != nil {
		c.Error(err)
		return
	}

	utils.SuccessResponse(c, http.StatusOK, utils.CodeOTPVerified, "Kode OTP valid. Silakan buat kata sandi baru.", result)
}

// ResetPassword menangani POST forgot-password/reset (F1.5). Publik.
func (h *OTPController) ResetPassword(c *gin.Context) {
	var req dto.SetPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(utils.NewValidationError(utils.FormatValidationErrors(err)))
		return
	}

	if err := h.passwords.ResetPasswordWithToken(c.Request.Context(), req.ResetToken, req.NewPassword); err != nil {
		c.Error(err)
		return
	}

	utils.SuccessResponse(c, http.StatusOK, utils.CodePasswordChanged, "Password berhasil direset. Semua sesi dihentikan, silakan login ulang.", nil)
}

// toOTPRequestedResponse memetakan hasil service ke DTO response.
func toOTPRequestedResponse(result *services.OTPIssueResult) dto.OTPRequestedResponse {
	return dto.OTPRequestedResponse{
		AttemptsRemaining:  result.AttemptsRemaining,
		ExpiresAt:          result.ExpiresAt,
		ResendAfterSeconds: result.ResendAfterSeconds,
	}
}
