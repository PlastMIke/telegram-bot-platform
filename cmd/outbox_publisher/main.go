package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/nats-io/nats.go"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/PlastMIke/telegram-bot-platform/internal/config"
	"github.com/PlastMIke/telegram-bot-platform/internal/outbox"
	"github.com/PlastMIke/telegram-bot-platform/internal/repository"
)

func main() {
	// 1. Загружаем конфигурацию
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// 2. Подключаемся к БД
	db, err := gorm.Open(postgres.Open(cfg.DatabaseURL), &gorm.Config{})
	if err != nil {
		log.Fatalf("Failed to connect to DB: %v", err)
	}
	log.Println("✅ Outbox Publisher connected to Database")

	// 3. Подключаемся к NATS
	nc, err := nats.Connect(cfg.NatsURL)
	if err != nil {
		log.Fatalf("Failed to connect to NATS: %v", err)
	}
	defer nc.Drain()
	log.Println("✅ Outbox Publisher connected to NATS")

	// 4. Dependency Injection
	outboxRepo := repository.NewOutboxRepository(db)
	publisher := outbox.NewPublisher(outboxRepo, nc, cfg)

	// 5. Graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-quit
		log.Println("🛑 Shutting down outbox publisher...")
		cancel()
	}()

	// 6. Запуск
	publisher.Run(ctx)
}
