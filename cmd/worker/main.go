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
		// Subscribe создаёт подписку на топик.
		// Callback вызывается для КАЖДОГО сообщения в топике.
		//
		// ⚠️ NATS по умолчанию обрабатывает сообщения ПОСЛЕДОВАТЕЛЬНО
		// (одна горутина на подписку). Для параллелизма:
		// - ChanSubscribe + worker pool
		// - Queue Groups (несколько инстансов Worker'а)

		// ─── Десериализация ───
		var event events.BotCreatedEvent
		if err := json.Unmarshal(msg.Data, &event); err != nil {
			log.Printf("❌ Failed to unmarshal message: %v", err)
			return
			// Если сообщение невалидное — логируем и пропускаем.
			// НЕ возвращаем в очередь: повторная обработка не поможет.
			//
			// ⚠️ В production: отправить в Dead Letter Queue (DLQ).
		}

		// ─── Обработка ───
		// ЗДЕСЬ БУДЕТ ЛОГИКА ЗАПУСКА TELEGRAM БОТА (Фаза 3):
		// 1. Создать telebot.Bot с event.Token
		// 2. Зарегистрировать хендлеры (/start, /help)
		// 3. Запустить bot.Start() в горутине
		// 4. Обновить статус в БД: status = "running"

		log.Printf("🤖 [WORKER] Received bot.created: ID=%d, Name=%s", event.BotID, event.BotName)

		log.Printf("✅ [WORKER] Bot %s initialized (mock)", event.BotName)

		// Подтверждаем обработку
		if err := msg.Ack(); err != nil {
			log.Printf("⚠️ Failed to ack message: %v", err)
		}
		// Ack() — подтверждаем обработку.
		// Без Ack (в JetStream) сообщение будет redelivered.
		//
		// ⚠️ IDEMPOTENCY:
		// Если Worker обработал сообщение, но упал ДО Ack —
		// сообщение будет доставлено повторно.
		// Обработка должна быть идемпотентной!
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
	// defer nc.Drain() выполнится здесь.
	// Drain() подождёт завершения обработки текущих сообщений.
}
