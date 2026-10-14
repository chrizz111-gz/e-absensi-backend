package routes

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"e-absensi-backend/config"
	"e-absensi-backend/controllers"
	"e-absensi-backend/middlewares"
	"e-absensi-backend/services"
	"e-absensi-backend/utils"
)

// RegisterRoutes memasang seluruh route API ke router Gin.
func RegisterRoutes(r *gin.Engine, db *gorm.DB) {
	r.Use(middlewares.CORS(config.CORSAllowedOrigins()))
	r.Use(middlewares.ErrorHandler())

	api := r.Group("/api")
	{
		api.GET("/status", func(c *gin.Context) {
			dbStatus := "ok"
			if sqlDB, err := db.DB(); err != nil || sqlDB.Ping() != nil {
				dbStatus = "down"
			}
			utils.SuccessResponse(c, http.StatusOK, utils.CodeSuccess, "Sistem Backend E-Absensi Siap!", gin.H{
				"database": dbStatus,
			})
		})
	}

	var googleVerifier utils.GoogleVerifier
	if clientID := config.GoogleClientID(); clientID != "" {
		googleVerifier = utils.NewIDTokenVerifier(clientID)
	}
	authService := services.NewAuthService(db, googleVerifier)
	authController := controllers.NewAuthController(authService)

	mailer := utils.NewEmailSender()
	otpService := services.NewOTPService(db, mailer)
	passwordService := services.NewPasswordService(db, otpService)
	otpController := controllers.NewOTPController(otpService, passwordService)

	v1 := r.Group("/api/v1/auth")
	v1.Use(middlewares.RateLimit(config.RateLimitRPS, config.RateLimitBurst))
	{
		v1.POST("/register", authController.Register)
		v1.POST("/login", authController.Login)
		v1.POST("/google", authController.GoogleLogin)
		v1.POST("/refresh", authController.Refresh)

		// Reset password (publik) - F1.5 & F1.6.
		v1.POST("/forgot-password", otpController.RequestResetPasswordOTP)
		v1.POST("/forgot-password/resend", otpController.RequestResetPasswordOTP)
		v1.POST("/forgot-password/verify", otpController.ResetPassword)

		authed := v1.Group("")
		authed.Use(middlewares.AuthRequired(db))
		{
			authed.POST("/logout", authController.Logout)
			authed.POST("/logout-all", authController.LogoutAll)

			// Ganti password (butuh login) - F1.4 & F1.6.
			authed.POST("/change-password/request", otpController.RequestChangePasswordOTP)
			authed.POST("/change-password/resend", otpController.RequestChangePasswordOTP)
			authed.POST("/change-password/verify", otpController.ChangePassword)
		}
	}
}
