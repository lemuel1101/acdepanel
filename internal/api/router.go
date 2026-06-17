package api

import (
	"github.com/gin-gonic/gin"
	"github.com/novapanel/novapanel/internal/api/handlers"
	"github.com/novapanel/novapanel/internal/auth"
	"github.com/novapanel/novapanel/internal/config"
)

func SetupRouter(cfg *config.Config) *gin.Engine {
	if cfg.Server.Mode == "release" {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery())
	r.Use(SecurityHeadersMiddleware())
	r.Use(SecurityContext())
	r.Use(CORSMiddleware())

	if cfg.Server.FrontendDir != "" {
		r.Static("/static", cfg.Server.FrontendDir+"/assets")
		r.StaticFile("/", cfg.Server.FrontendDir+"/index.html")
		r.NoRoute(func(c *gin.Context) {
			c.File(cfg.Server.FrontendDir + "/index.html")
		})
	}

	authHandler := handlers.NewAuthHandler(cfg)
	userHandler := handlers.NewUserHandler()
	domainHandler := handlers.NewDomainHandler(cfg)
	fileHandler := handlers.NewFileHandler()
	dbHandler := handlers.NewDatabaseHandler()
	emailHandler := handlers.NewEmailHandler()
	firewallHandler := handlers.NewFirewallHandler()
	backupHandler := handlers.NewBackupHandler(cfg)
	systemHandler := handlers.NewSystemHandler()
	cronHandler := handlers.NewCronHandler()

	api := r.Group("/api")
	{
		api.POST("/login", authHandler.Login)
		api.POST("/refresh", authHandler.RefreshToken)
	}

	api.POST("/csrf-token", func(c *gin.Context) {
		token := "np_" + c.ClientIP()
		c.SetCookie("csrf_token", token, 86400, "/", "", false, true)
		c.JSON(200, gin.H{"csrf_token": token})
	})

	protected := api.Group("")
	protected.Use(AuthMiddleware(&cfg.JWT))
	{
		protected.GET("/me", authHandler.Me)
		protected.POST("/logout", authHandler.Logout)
		protected.POST("/change-password", authHandler.ChangePassword)

		users := protected.Group("/users")
		users.Use(RoleMiddleware(auth.RoleAdmin, auth.RoleReseller))
		{
			users.GET("", userHandler.List)
			users.GET("/:id", userHandler.Get)
			users.POST("", userHandler.Create)
			users.PUT("/:id", userHandler.Update)
			users.DELETE("/:id", userHandler.Delete)
		}

		domains := protected.Group("/domains")
		{
			domains.GET("", domainHandler.List)
			domains.GET("/:id", domainHandler.Get)
			domains.POST("", domainHandler.Create)
			domains.DELETE("/:id", domainHandler.Delete)
			domains.POST("/:id/ssl", domainHandler.EnableSSL)
			domains.PUT("/:id/php", domainHandler.SetPHP)
			domains.POST("/:id/redirect", domainHandler.SetRedirect)
			domains.GET("/:id/logs", domainHandler.GetLogs)
		}

		files := protected.Group("/files")
		{
			files.GET("", fileHandler.List)
			files.GET("/read", fileHandler.Read)
			files.POST("/write", fileHandler.Write)
			files.POST("/delete", fileHandler.Delete)
			files.POST("/upload", fileHandler.Upload)
			files.POST("/mkdir", fileHandler.CreateDir)
			files.POST("/chmod", fileHandler.Chmod)
			files.POST("/zip", fileHandler.Zip)
			files.POST("/unzip", fileHandler.Unzip)
		}

		databases := protected.Group("/databases")
		{
			databases.GET("", dbHandler.List)
			databases.POST("", dbHandler.Create)
			databases.DELETE("/:id", dbHandler.Delete)
			databases.POST("/:id/users", dbHandler.CreateUser)
			databases.DELETE("/:id/users/:userId", dbHandler.DeleteUser)
		}

		email := protected.Group("/email")
		{
			email.GET("/:domain_id", emailHandler.List)
			email.POST("", emailHandler.Create)
			email.DELETE("/:id", emailHandler.Delete)
			email.PUT("/:id/password", emailHandler.UpdatePassword)
		}

		firewall := protected.Group("/firewall")
		firewall.Use(RoleMiddleware(auth.RoleAdmin))
		{
			firewall.GET("", firewallHandler.List)
			firewall.POST("", firewallHandler.Create)
			firewall.DELETE("/:id", firewallHandler.Delete)
			firewall.PUT("/:id/toggle", firewallHandler.Toggle)
			firewall.GET("/status", firewallHandler.Status)
			firewall.POST("/toggle", firewallHandler.ToggleFirewall)
		}

		backups := protected.Group("/backups")
		{
			backups.GET("", backupHandler.List)
			backups.POST("", backupHandler.Create)
			backups.GET("/:id/download", backupHandler.Download)
			backups.DELETE("/:id", backupHandler.Delete)
			backups.PUT("/settings", backupHandler.Settings)
		}

		systemGroup := protected.Group("/system")
		{
			systemGroup.GET("/stats", systemHandler.Stats)
			systemGroup.GET("/services", systemHandler.Services)
			systemGroup.POST("/services/:name/:action", systemHandler.ServiceAction)
			systemGroup.GET("/logs", systemHandler.Logs)
			systemGroup.GET("/processes", systemHandler.Processes)
			systemGroup.GET("/disk-usage", systemHandler.DiskUsage)
			systemGroup.GET("/audit-logs", systemHandler.AuditLogs)
		}

		cron := protected.Group("/cron")
		{
			cron.GET("", cronHandler.List)
			cron.POST("", cronHandler.Create)
			cron.PUT("/:id", cronHandler.Update)
			cron.DELETE("/:id", cronHandler.Delete)
		}

		protected.GET("/ws/stats", systemHandler.Websocket)
	}

	return r
}
