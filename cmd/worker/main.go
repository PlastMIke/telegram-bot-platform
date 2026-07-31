package main

import (
	"encoding/json"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/nats-io/nats.go"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/PlastMIke/telegram-bot-platform/internal/config"
	"github.com/PlastMIke/telegram-bot-platform/internal/events"
)

func main() {
	// 1. Загружаем конфигурацию
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// 2. Инициализируем БД (подключение для валидации конфигурации и миграций при необходимости)
	db, err := gorm.Open(postgres.Open(cfg.DatabaseURL), &gorm.Config{})
	if err != nil {
		log.Fatalf("Failed to connect to DB: %v", err)
	}
	_ = db // соединение не используется напрямую в текущей реализации
	
	log.Println("✅ Worker connected to Database")

	// 3. Подключаемся к NATS
	nc, err := nats.Connect(cfg.NatsURL)
	if err != nil {
		log.Fatalf("Failed to connect to NATS: %v", err)
	}
	defer nc.Drain()
	log.Println("✅ Worker connected to NATS")

	// 4. Подписка на события
	_, err = nc.Subscribe(events.TopicBotCreated, func(msg *nats.Msg) {
		var event events.BotCreatedEvent
		if err := json.Unmarshal(msg.Data, &event); err != nil {
			log.Printf("❌ Failed to unmarshal message: %v", err)
			return
		}

		log.Printf("🤖 [WORKER] Received bot.created: ID=%d, Name=%s", event.BotID, event.BotName)

		// ЗДЕСЬ БУДЕТ ЛОГИКА ЗАПУСКА TELEGRAM БОТА (Фаза 3)
		log.Printf("✅ [WORKER] Bot %s initialized (mock)", event.BotName)

		// Подтверждаем обработку
		if err := msg.Ack(); err != nil {
			log.Printf("⚠️ Failed to ack message: %v", err)
		}
	})
	if err != nil {
		log.Fatalf("Failed to subscribe: %v", err)
	}

	log.Println("🚀 Worker is running and listening for events...")

	// 5. Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("🛑 Shutting down worker...")
}
