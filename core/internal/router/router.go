package router

import (
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/luketaylor45/atlas/core/internal/handlers"
	"github.com/luketaylor45/atlas/core/internal/middleware"
)

func Setup(r *gin.Engine) {
	// CORS Setup
	r.Use(cors.New(cors.Config{
		AllowAllOrigins: true,
		AllowMethods:    []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:    []string{"Origin", "Content-Type", "Accept", "Authorization"},
		ExposeHeaders:   []string{"Content-Length"},
		// Sessions use a bearer header, never cookies, so credentials are not allowed cross-origin.
		AllowCredentials: false,
		MaxAge:           12 * time.Hour,
	}))

	api := r.Group("/api/v1")
	{
		// Credential endpoints are limited per client address to slow down
		// password guessing.
		credentialLimit := middleware.NewRateLimiter(10, 5*time.Minute).Middleware()

		auth := api.Group("/auth")
		{
			auth.POST("/login", credentialLimit, handlers.Login)
			auth.POST("/register", credentialLimit, handlers.Register)
			auth.POST("/setup", credentialLimit, handlers.InitialSetup)
			auth.GET("/setup-status", handlers.GetSetupStatus)
			auth.GET("/me", middleware.AuthMiddleware(), handlers.GetCurrentUser)
			auth.POST("/password", middleware.AuthMiddleware(), credentialLimit, handlers.ChangeOwnPassword)
		}

		// Daemon-facing routes: every one requires a valid node token.
		internal := api.Group("/internal")
		internal.Use(middleware.NodeAuthMiddleware())
		{
			internal.POST("/heartbeat", handlers.HandleHeartbeat)
			internal.POST("/services/:uuid/status", handlers.HandleServerStatusUpdate)
			internal.POST("/backups/:backup/status", handlers.HandleBackupStatusUpdate)
			internal.POST("/sftp/validate", handlers.ValidateSFTPCredentials)
		}

		// Admin Routes (Protected)
		admin := api.Group("/admin")
		admin.Use(middleware.AuthMiddleware(), middleware.AdminMiddleware())
		{
			admin.GET("/overview", handlers.GetAdminOverview)
			admin.GET("/nodes", handlers.GetNodes)
			admin.POST("/nodes", handlers.CreateNode)
			admin.PUT("/nodes/:id", handlers.UpdateNode)
			admin.DELETE("/nodes/:id", handlers.DeleteNode)
			admin.POST("/nodes/:id/token", handlers.RegenerateNodeToken)
			admin.GET("/users", handlers.GetUsers)
			admin.POST("/users", handlers.CreateUser)
			admin.PUT("/users/:id", handlers.UpdateUser)
			admin.DELETE("/users/:id", handlers.DeleteUser)

			admin.GET("/eggs", handlers.GetEggs)
			admin.PUT("/eggs/:id", handlers.UpdateEgg)
			admin.DELETE("/eggs/:id", handlers.DeleteEgg)
			admin.GET("/nests", handlers.GetNests)
			admin.POST("/nests", handlers.CreateNest)
			admin.PUT("/nests/:id", handlers.UpdateNest)
			admin.DELETE("/nests/:id", handlers.DeleteNest)
			admin.POST("/eggs/import", handlers.ImportEgg)
			admin.GET("/services", handlers.GetServices)
			admin.POST("/services", handlers.CreateService)
			admin.PUT("/services/:id", handlers.UpdateService)
			admin.DELETE("/services/:id", handlers.DeleteService)

			// News Management (Admin)
			admin.POST("/news", handlers.CreateNews)
			admin.PUT("/news/:id", handlers.UpdateNews)
			admin.DELETE("/news/:id", handlers.DeleteNews)
		}

		// The console websocket authenticates with a single-use style ticket
		// instead of the session header, which browsers cannot send on a socket.
		api.GET("/services/:uuid/console/ws", handlers.ServiceConsoleSocket)
		// Backup downloads are followed by the browser itself and carry a
		// ticket for the same reason.
		api.GET("/services/:uuid/backups/:backup/download", handlers.DownloadBackup)

		// User Service Routes
		services := api.Group("/services")
		services.Use(middleware.AuthMiddleware())
		{
			services.GET("/overview", handlers.GetUserOverview)
			services.GET("", handlers.GetUserServices)
			services.GET("/:uuid", handlers.GetServiceDetails)
			services.POST("/:uuid/power", handlers.ServicePowerAction)
			services.POST("/:uuid/command", handlers.ServiceSendCommand)
			services.POST("/:uuid/reinstall", handlers.ServiceReinstall)
			services.POST("/:uuid/rebuild", handlers.ServiceRebuild)
			services.POST("/:uuid/install/cancel", handlers.ServiceCancelInstall)
			services.GET("/:uuid/install/log", handlers.ServiceInstallLog)
			services.POST("/:uuid/environment", handlers.UpdateServiceEnvironment)
			services.POST("/:uuid/console/ticket", handlers.CreateConsoleTicket)
			services.GET("/:uuid/stats", handlers.ServiceStats)

			// File Management
			services.GET("/:uuid/files/list", handlers.ServiceListFiles)
			services.GET("/:uuid/files/content", handlers.ServiceGetFileContent)
			services.POST("/:uuid/files/write", handlers.ServiceWriteFile)
			services.POST("/:uuid/files/create-folder", handlers.ServiceCreateFolder)
			services.POST("/:uuid/files/extract", handlers.ServiceExtractArchive)
			services.POST("/:uuid/files/upload", handlers.ServiceUploadFile)
			services.DELETE("/:uuid/files", handlers.ServiceDeleteFile)

			// Sub-user Management (unify with :uuid to avoid Gin conflict)
			services.GET("/:uuid/users", handlers.GetServiceUsers)
			services.POST("/:uuid/users", handlers.AddServiceUser)
			services.PUT("/:uuid/users/:userId", handlers.UpdateServiceUser)
			services.DELETE("/:uuid/users/:userId", handlers.RemoveServiceUser)

			// Backups
			services.GET("/:uuid/backups", handlers.ListBackups)
			services.POST("/:uuid/backups", handlers.CreateBackup)
			services.DELETE("/:uuid/backups/:backup", handlers.DeleteBackup)
			services.POST("/:uuid/backups/:backup/restore", handlers.RestoreBackup)
			services.POST("/:uuid/backups/:backup/download", handlers.CreateBackupDownload)

			// Schedules
			services.GET("/:uuid/schedules", handlers.ListSchedules)
			services.POST("/:uuid/schedules", handlers.CreateSchedule)
			services.PUT("/:uuid/schedules/:schedule", handlers.UpdateSchedule)
			services.DELETE("/:uuid/schedules/:schedule", handlers.DeleteSchedule)
			services.POST("/:uuid/schedules/:schedule/run", handlers.RunScheduleNow)

			// Activity Logs
			services.GET("/:uuid/logs", handlers.GetServiceActivityLogs)
		}

		// Global Routes
		api.GET("/news", handlers.GetNews)
	}
}
