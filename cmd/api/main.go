package main

import (
	"context"
	"fmt"
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
		// log.Fatalf = log.Printf + os.Exit(1).
		// Если конфиг невалидный — нет смысла продолжать.
	}

	// 2. Подключаемся к PostgreSQL
	db, err := gorm.Open(postgres.Open(cfg.DatabaseURL), &gorm.Config{})
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	log.Println("✅ Connected to database")
	// ⚠️ В production нужно настроить connection pool:
	//   sqlDB, _ := db.DB()
	//   sqlDB.SetMaxOpenConns(cfg.DBMaxConns)
	//   sqlDB.SetMaxIdleConns(cfg.DBMinConns)
	//   sqlDB.SetConnMaxIdleTime(cfg.DBMaxConnIdle)
	//   sqlDB.SetConnMaxLifetime(cfg.DBMaxConnLifetime)

	// 3. Подключаемся к NATS (для публикации событий)
	nc, err := nats.Connect(cfg.NatsURL)
	if err != nil {
		log.Fatalf("Failed to connect to NATS: %v", err)
	}
	defer nc.Drain()
	// Drain() — graceful закрытие NATS-соединения.
	// Ждёт завершения всех pending-операций.
	//
	// defer — выполнится при выходе из main().
	// Порядок defer'ов: LIFO (последний добавленный — первый выполнится).
	log.Println("✅ Connected to NATS")

	// 4. Dependency Injection
	// Создаём объекты "снизу вверх": models → repository → service → handler.
	// Это называется "Composition Root" — единственное место,
	// где все зависимости связаны.
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
		Addr:    fmt.Sprintf(":%d", cfg.Port),
		Handler: router,
		// ⚠️ В production нужно добавить таймауты:
		//   ReadTimeout:  5 * time.Second,
		//   WriteTimeout: 10 * time.Second,
		//   IdleTimeout:  120 * time.Second,
		// Без таймаутов медленный клиент может держать соединение вечно
		// (Slowloris attack).
	}

	// 7. Запускаем сервер в горутине
	go func() {
		log.Printf("🚀 API Gateway starting on port %d", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()
	// go func() { ... }() — запускаем сервер в отдельной горутине.
	//
	// ЗАЧЕМ?
	// ListenAndServe() БЛОКИРУЕТ выполнение.
	// Если запустить в main() — код ниже никогда не выполнится.
	// А нам нужно дойти до graceful shutdown.
	//
	// err != http.ErrServerClosed — нормальное завершение.
	// Shutdown() вызывает ListenAndServe() вернуть ErrServerClosed.
	// Это НЕ ошибка — не логируем как fatal.

	// 8. Graceful shutdown
	quit := make(chan os.Signal, 1)
	// Канал для сигналов ОС.
	// Буфер 1 — чтобы не пропустить сигнал, если мы ещё не в select.
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	// SIGINT — Ctrl+C (пользователь хочет остановить)
	// SIGTERM — docker stop, kubernetes delete (система хочет остановить)
	//
	// SIGKILL (kill -9) НЕЛЬЗЯ перехватить.
	// Поэтому всегда используем SIGTERM для graceful shutdown.
	<-quit
	// БЛОКИРУЕМСЯ здесь, пока не придёт сигнал.
	// Main-горутина "спит", HTTP-сервер работает в своей горутине.
	log.Println("🛑 Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	// Даём серверу 5 секунд на завершение активных запросов.
	// Если за 5 секунд не успел — принудительно закрываем.

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
		// Shutdown():
		// 1. Закрывает listener (не принимает новые запросы)
		// 2. Ждёт завершения активных запросов (до timeout)
		// 3. Закрывает idle-соединения
		// 4. Возвращает nil или error
	}

	log.Println("✅ Server exited gracefully")
}
