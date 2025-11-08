package main

import (
	"log"
	"os"

	"github.com/hiiamtrong/go-imap-bot/internal/api/handlers"
	"github.com/hiiamtrong/go-imap-bot/internal/config"
	"github.com/hiiamtrong/go-imap-bot/internal/database"
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

	// Setup handlers
	transactionHandler := handlers.NewTransactionHandler(transactionRepo, tagRepo, transactionSplitRepo, userRepo)
	userHandler := handlers.NewUserHandler(userRepo)
	tagHandler := handlers.NewTagHandler(tagRepo)
	splitHandler := handlers.NewSplitHandler(transactionSplitRepo, transactionRepo, userRepo, smtpService)
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

	// Transaction routes
	api.GET("/transactions", transactionHandler.GetTransactions)
	api.GET("/transactions/:id", transactionHandler.GetTransaction)
	api.POST("/transactions/virtual", transactionHandler.CreateVirtualBill)
	api.POST("/transactions/:id/complete", transactionHandler.CompleteTransaction)
	api.DELETE("/transactions/:id", transactionHandler.DeleteTransaction)
	api.POST("/transactions/:id/tags/:tagId", transactionHandler.AddTagToTransaction)
	api.DELETE("/transactions/:id/tags/:tagId", transactionHandler.RemoveTagFromTransaction)

	// User routes
	api.GET("/users", userHandler.GetUsers)
	api.GET("/users/:id", userHandler.GetUser)
	api.POST("/users", userHandler.CreateUser)
	api.PUT("/users/:id", userHandler.UpdateUser)
	api.DELETE("/users/:id", userHandler.DeleteUser)

	// Tag routes
	api.GET("/tags", tagHandler.GetTags)
	api.POST("/tags", tagHandler.CreateTag)

	// Split routes
	api.GET("/splits/pending", splitHandler.GetPendingSplitsSummary)
	api.POST("/splits", splitHandler.CreateSplit)
	api.GET("/transactions/:id/splits", splitHandler.GetSplitsForTransaction)
	api.POST("/splits/:id/complete", splitHandler.CompleteSplit)
	api.DELETE("/splits/:id", splitHandler.DeleteSplit)

	// Reminder routes
	api.POST("/reminders", splitHandler.SendReminders)

	// Statistics routes
	api.GET("/statistics", statsHandler.GetStatistics)
	api.GET("/statistics/monthly", statsHandler.GetMonthlySpending)
	api.GET("/statistics/tags", statsHandler.GetTagSpending)

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
