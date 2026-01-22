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
		AllowOrigins:     []string{"http://localhost:5173", "http://127.0.0.1:5173", "http://localhost:5174", "http://127.0.0.1:5174"}, // Allow Vite Frontend
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	api := r.Group("/api/v1")
	{
		auth := api.Group("/auth")
		{
			auth.POST("/login", handlers.Login)
			auth.POST("/setup", handlers.InitialSetup)
			auth.GET("/setup-status", handlers.GetSetupStatus)
		}

		internal := api.Group("/internal")
		{
			internal.POST("/heartbeat", handlers.HandleHeartbeat)
			internal.POST("/services/:uuid/status", handlers.HandleServerStatusUpdate)
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
			admin.GET("/users", handlers.GetUsers)
			admin.POST("/users", handlers.CreateUser)
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
		}

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
			services.POST("/:uuid/environment", handlers.UpdateServiceEnvironment)

			// File Management
			services.GET("/:uuid/files/list", handlers.ServiceListFiles)
			services.GET("/:uuid/files/content", handlers.ServiceGetFileContent)
			services.POST("/:uuid/files/write", handlers.ServiceWriteFile)
			services.POST("/:uuid/files/create-folder", handlers.ServiceCreateFolder)
			services.POST("/:uuid/files/upload", handlers.ServiceUploadFile)
			services.DELETE("/:uuid/files", handlers.ServiceDeleteFile)
		}
	}
}
