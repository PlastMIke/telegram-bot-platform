package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/nats-io/nats.go"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/PlastMIke/telegram-bot-platform/internal/api"
	"github.com/PlastMIke/telegram-bot-platform/internal/config"
	"github.com/PlastMIke/telegram-bot-platform/internal/repository"
	"github.com/PlastMIke/telegram-bot-platform/internal/service"
)

func main() {
	// 1. Загружаем конфигурацию
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// 2. Подключаемся к PostgreSQL
	db, err := gorm.Open(postgres.Open(cfg.DatabaseURL), &gorm.Config{})
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	log.Println("✅ Connected to database")

	// 3. Подключаемся к NATS (для публикации событий)
	nc, err := nats.Connect(cfg.NatsURL)
	if err != nil {
		log.Fatalf("Failed to connect to NATS: %v", err)
	}
	defer nc.Drain()
	log.Println("✅ Connected to NATS")

	// 4. Dependency Injection
	userRepo := repository.NewUserRepository(db)
	botRepo := repository.NewBotRepository(db)
	outboxRepo := repository.NewOutboxRepository(db)

	authService := service.NewAuthService(userRepo, cfg.JWTSecret)
	botService := service.NewBotService(db, botRepo, outboxRepo)

	handler := api.NewHandler(authService, botService)

	// 5. Настраиваем роутер
	router := api.SetupRouter(handler, cfg)

	// 6. Создаём HTTP-сервер
	srv := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: router,
	}

	// 7. Запускаем сервер в горутине
	go func() {
		log.Printf("🚀 API Gateway starting on port %s", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	// 8. Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("🛑 Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("✅ Server exited gracefully")
}
