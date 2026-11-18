package routes

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"e-absensi-backend/config"
	"e-absensi-backend/controllers"
	"e-absensi-backend/middlewares"
	"e-absensi-backend/models"
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
	rbacController := controllers.NewRBACController(db)

	auditLogger := services.NewAuditLogger()
	userService := services.NewUserService(db, auditLogger)
	userController := controllers.NewUserController(userService)

	// Epic 3 Services & Controllers
	academicYearService := services.NewAcademicYearService(db, auditLogger)
	academicYearController := controllers.NewAcademicYearController(academicYearService)

	subjectService := services.NewSubjectService(db, auditLogger)
	subjectController := controllers.NewSubjectController(subjectService)

	classService := services.NewClassService(db, auditLogger)
	classController := controllers.NewClassController(classService)

	teacherService := services.NewTeacherService(db, auditLogger)
	teacherController := controllers.NewTeacherController(teacherService)

	studentService := services.NewStudentService(db, auditLogger)
	studentController := controllers.NewStudentController(studentService)

	// Epic 4 Service & Controller
	scheduleService := services.NewScheduleService(db, auditLogger)
	scheduleController := controllers.NewScheduleController(scheduleService)

	// Endpoint terautentikasi (/api/v1). Permission dicek langsung ke DB
	// (user_roles -> role_permissions -> permissions) dengan prinsip default deny.
	apiV1 := r.Group("/api/v1")
	apiV1.Use(middlewares.AuthRequired(db))
	{
		// Epic 2: endpoint read-only RBAC.
		apiV1.GET("/roles",
			middlewares.RequirePermission(db, models.PermissionRoleAssign),
			rbacController.ListRoles,
		)
		apiV1.GET("/permissions/matrix",
			middlewares.RequirePermission(db, models.PermissionPermissionView),
			rbacController.PermissionMatrix,
		)

		// Epic 2: CRUD user (khusus Admin + permission granular).
		users := apiV1.Group("/users")
		users.Use(middlewares.RequireRoles(models.RoleAdmin))
		{
			users.GET("",
				middlewares.RequirePermission(db, models.PermissionAccountView),
				userController.List,
			)
			users.GET("/:id",
				middlewares.RequirePermission(db, models.PermissionAccountView),
				userController.Get,
			)
			users.POST("",
				middlewares.RequirePermission(db, models.PermissionAccountCreate),
				userController.Create,
			)
			users.PATCH("/:id/status",
				middlewares.RequirePermission(db, models.PermissionAccountSetStatus),
				userController.SetStatus,
			)
			users.PUT("/:id/roles",
				middlewares.RequirePermission(db, models.PermissionRoleAssign),
				userController.ReplaceRoles,
			)
			users.PATCH("/:id",
				middlewares.RequirePermission(db, models.PermissionAccountUpdate),
				userController.Update,
			)
			users.DELETE("/:id",
				middlewares.RequirePermission(db, models.PermissionAccountDelete),
				userController.Delete,
			)
		}

		// Epic 3 F3.5: Tahun Ajaran (POST & DELETE khusus Admin; GET & PATCH Admin + Waka Kurikulum).
		academicYears := apiV1.Group("/academic-years")
		{
			academicYears.GET("",
				middlewares.RequirePermission(db, models.PermissionAcademicYearView),
				academicYearController.List,
			)
			academicYears.GET("/:id",
				middlewares.RequirePermission(db, models.PermissionAcademicYearView),
				academicYearController.Get,
			)
			academicYears.POST("",
				middlewares.RequirePermission(db, models.PermissionAcademicYearCreate),
				academicYearController.Create,
			)
			academicYears.PATCH("/:id",
				middlewares.RequirePermission(db, models.PermissionAcademicYearUpdate),
				academicYearController.Update,
			)
			academicYears.DELETE("/:id",
				middlewares.RequirePermission(db, models.PermissionAcademicYearDelete),
				academicYearController.Delete,
			)
		}

		// Epic 3 F3.4: Mata Pelajaran (CRUD Admin & Waka Kurikulum; GET juga Guru Mapel).
		subjects := apiV1.Group("/subjects")
		{
			subjects.GET("",
				middlewares.RequirePermission(db, models.PermissionSubjectView),
				subjectController.List,
			)
			subjects.GET("/:id",
				middlewares.RequirePermission(db, models.PermissionSubjectView),
				subjectController.Get,
			)
			subjects.POST("",
				middlewares.RequirePermission(db, models.PermissionSubjectCreate),
				subjectController.Create,
			)
			subjects.PATCH("/:id",
				middlewares.RequirePermission(db, models.PermissionSubjectUpdate),
				subjectController.Update,
			)
			subjects.DELETE("/:id",
				middlewares.RequirePermission(db, models.PermissionSubjectDelete),
				subjectController.Delete,
			)
		}

		// Epic 3 F3.3: Kelas (CRUD Admin & Waka Kurikulum; GET seluruh role).
		classes := apiV1.Group("/classes")
		{
			classes.GET("",
				middlewares.RequirePermission(db, models.PermissionClassView),
				classController.List,
			)
			classes.GET("/:id",
				middlewares.RequirePermission(db, models.PermissionClassView),
				classController.Get,
			)
			classes.POST("",
				middlewares.RequirePermission(db, models.PermissionClassCreate),
				classController.Create,
			)
			classes.PATCH("/:id",
				middlewares.RequirePermission(db, models.PermissionClassUpdate),
				classController.Update,
			)
			classes.DELETE("/:id",
				middlewares.RequirePermission(db, models.PermissionClassDelete),
				classController.Delete,
			)
		}

		// Epic 3 F3.2: Data Guru (GET, POST, PATCH untuk Admin & Waka Kurikulum; TANPA DELETE).
		teachers := apiV1.Group("/teachers")
		{
			teachers.GET("/candidates",
				middlewares.RequirePermission(db, models.PermissionTeacherView),
				teacherController.Candidates,
			)
			teachers.GET("/available-users",
				middlewares.RequirePermission(db, models.PermissionTeacherView),
				teacherController.Candidates,
			)
			teachers.GET("",
				middlewares.RequirePermission(db, models.PermissionTeacherView),
				teacherController.List,
			)
			teachers.GET("/:id",
				middlewares.RequirePermission(db, models.PermissionTeacherView),
				teacherController.Get,
			)
			teachers.POST("",
				middlewares.RequirePermission(db, models.PermissionTeacherCreate),
				teacherController.Create,
			)
			teachers.PATCH("/:id",
				middlewares.RequirePermission(db, models.PermissionTeacherUpdate),
				teacherController.Update,
			)
		}

		// Epic 3 F3.1: Data Siswa (GET & PATCH; TANPA POST & TANPA DELETE).
		students := apiV1.Group("/students")
		{
			students.GET("",
				middlewares.RequirePermission(db, models.PermissionStudentView),
				studentController.List,
			)
			students.GET("/:id",
				middlewares.RequirePermission(db, models.PermissionStudentView),
				studentController.Get,
			)
			students.PATCH("/:id",
				middlewares.RequirePermission(db, models.PermissionStudentUpdate),
				studentController.Update,
			)
		}

		// Epic 4 F4.1-F4.6: Manajemen Jadwal (TANPA POST tambah manual).
		schedules := apiV1.Group("/schedules")
		{
			schedules.GET("/template",
				middlewares.RequirePermission(db, models.PermissionScheduleDownloadTemplate),
				scheduleController.DownloadTemplate,
			)
			schedules.POST("/import/preview",
				middlewares.RequirePermission(db, models.PermissionScheduleImport),
				scheduleController.PreviewImport,
			)
			schedules.POST("/import/confirm",
				middlewares.RequirePermission(db, models.PermissionScheduleImport),
				scheduleController.ConfirmImport,
			)
			schedules.GET("",
				middlewares.RequirePermission(db, models.PermissionScheduleView),
				scheduleController.List,
			)
			schedules.GET("/:id",
				middlewares.RequirePermission(db, models.PermissionScheduleView),
				scheduleController.Get,
			)
			schedules.PATCH("/:id",
				middlewares.RequirePermission(db, models.PermissionScheduleUpdate),
				scheduleController.Update,
			)
			schedules.PATCH("/:id/convert-to-activity",
				middlewares.RequirePermission(db, models.PermissionScheduleConvertActivity),
				scheduleController.ConvertToActivity,
			)
			schedules.PATCH("/:id/revert-to-lesson",
				middlewares.RequirePermission(db, models.PermissionScheduleConvertActivity),
				scheduleController.RevertToLesson,
			)
			schedules.DELETE("/:id",
				middlewares.RequirePermission(db, models.PermissionScheduleDelete),
				scheduleController.Delete,
			)
		}
	}

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
		v1.POST("/forgot-password/verify-otp", otpController.VerifyResetPasswordOTP)
		v1.POST("/forgot-password/reset", otpController.ResetPassword)

		authed := v1.Group("")
		authed.Use(middlewares.AuthRequired(db))
		{
			authed.POST("/logout", authController.Logout)
			authed.POST("/logout-all", authController.LogoutAll)

			// Ganti password (butuh login) - F1.4 & F1.6.
			authed.POST("/change-password/request", otpController.RequestChangePasswordOTP)
			authed.POST("/change-password/resend", otpController.RequestChangePasswordOTP)
			authed.POST("/change-password/verify-otp", otpController.VerifyChangePasswordOTP)
			authed.POST("/change-password/confirm", otpController.ConfirmChangePassword)
		}
	}
}
