package main

import (
	"log"
	"os"

	"github.com/hiiamtrong/go-imap-bot/internal/api/handlers"
	"github.com/hiiamtrong/go-imap-bot/internal/config"
	"github.com/hiiamtrong/go-imap-bot/internal/database"
	authmiddleware "github.com/hiiamtrong/go-imap-bot/internal/middleware"
	"github.com/hiiamtrong/go-imap-bot/internal/repository"
	"github.com/hiiamtrong/go-imap-bot/internal/s3"
	"github.com/hiiamtrong/go-imap-bot/internal/smtp"
	"github.com/hiiamtrong/go-imap-bot/internal/vietqr"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	_ "github.com/mattn/go-sqlite3"
)

func main() {
	// Load configuration
	cfg := config.NewConfig()

	// Setup database
	db, err := database.GetDatabase(cfg.DatabaseConfig)
	if err != nil {
		log.Fatalf("Failed to get database: %v", err)
	}
	defer db.Conn.Close()

	// Setup services
	s3Service, err := s3.NewS3Service(cfg)
	if err != nil {
		log.Fatalf("Failed to get S3 service: %v", err)
	}

	vietQR := vietqr.NewVietQRService(cfg, s3Service)

	smtpService, err := smtp.NewSMTPService(cfg, vietQR)
	if err != nil {
		log.Fatalf("Failed to get SMTP service: %v", err)
	}

	// Setup repositories
	transactionRepo := repository.NewTransactionRepository(db)
	tagRepo := repository.NewTagRepository(db)
	userRepo := repository.NewUserRepository(db)
	transactionSplitRepo := repository.NewTransactionSplitRepository(db)
	splitHashRepo := repository.NewSplitHashRepository(db)

	// Setup handlers
	authHandler := handlers.NewAuthHandler(cfg)
	transactionHandler := handlers.NewTransactionHandler(transactionRepo, tagRepo, transactionSplitRepo, userRepo)
	userHandler := handlers.NewUserHandler(userRepo)
	tagHandler := handlers.NewTagHandler(tagRepo)
	splitHandler := handlers.NewSplitHandler(transactionSplitRepo, transactionRepo, userRepo, smtpService, splitHashRepo)
	statsHandler := handlers.NewStatisticsHandler(db)

	// Setup Echo
	e := echo.New()

	// Middleware
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())

	// CORS middleware
	e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins: []string{"*"},
		AllowMethods: []string{echo.GET, echo.POST, echo.PUT, echo.DELETE, echo.PATCH, echo.OPTIONS},
		AllowHeaders: []string{echo.HeaderOrigin, echo.HeaderContentType, echo.HeaderAccept, echo.HeaderAuthorization},
	}))

	// Health check
	e.GET("/health", func(c echo.Context) error {
		return c.JSON(200, map[string]string{
			"status": "ok",
		})
	})

	// API routes
	api := e.Group("/api")

	// Auth routes (public)
	auth := api.Group("/auth")
	auth.GET("/login", authHandler.Login)
	auth.GET("/callback", authHandler.Callback)

	// Public auth API routes (with auth required for /me and /refresh)
	api.GET("/auth/me", authHandler.Me, authmiddleware.JWTMiddleware(cfg.OAuth))
	api.POST("/auth/refresh", authHandler.Refresh, authmiddleware.JWTMiddleware(cfg.OAuth))

	// Protected API routes - require JWT authentication for all other routes
	protected := api.Group("", authmiddleware.JWTMiddleware(cfg.OAuth))

	// Transaction routes
	protected.GET("/transactions", transactionHandler.GetTransactions)
	protected.GET("/transactions/:id", transactionHandler.GetTransaction)
	protected.POST("/transactions/virtual", transactionHandler.CreateVirtualBill)
	protected.POST("/transactions/:id/complete", transactionHandler.CompleteTransaction)
	protected.DELETE("/transactions/:id", transactionHandler.DeleteTransaction)
	protected.POST("/transactions/:id/tags/:tagId", transactionHandler.AddTagToTransaction)
	protected.DELETE("/transactions/:id/tags/:tagId", transactionHandler.RemoveTagFromTransaction)

	// User routes
	protected.GET("/users", userHandler.GetUsers)
	protected.GET("/users/:id", userHandler.GetUser)
	protected.POST("/users", userHandler.CreateUser)
	protected.PUT("/users/:id", userHandler.UpdateUser)
	protected.DELETE("/users/:id", userHandler.DeleteUser)

	// Tag routes
	protected.GET("/tags", tagHandler.GetTags)
	protected.POST("/tags", tagHandler.CreateTag)

	// Split routes
	protected.GET("/splits/pending", splitHandler.GetPendingSplitsSummary)
	protected.POST("/splits", splitHandler.CreateSplit)
	protected.GET("/transactions/:id/splits", splitHandler.GetSplitsForTransaction)
	protected.POST("/splits/:id/complete", splitHandler.CompleteSplit)
	protected.POST("/splits/:id/complete-single", splitHandler.CompleteSingleSplit)
	protected.PUT("/splits/:id", splitHandler.UpdateSplit)
	protected.DELETE("/splits/:id", splitHandler.DeleteSplit)

	// Reminder routes
	protected.POST("/reminders", splitHandler.SendReminders)

	// Statistics routes
	protected.GET("/statistics", statsHandler.GetStatistics)
	protected.GET("/statistics/monthly", statsHandler.GetMonthlySpending)
	protected.GET("/statistics/tags", statsHandler.GetTagSpending)

	// Get port from environment or use default
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// Start server
	log.Printf("Starting server on port %s", port)
	if err := e.Start(":" + port); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
