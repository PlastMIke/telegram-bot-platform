package main

import (
	"context"   // Для graceful shutdown
	"log"       // Для логирования
	"net/http"  // Для HTTP сервера
	"os"        // Для сигналов
	"os/signal" // Для обработки сигналов ОС
	"syscall"   // Для сигналов (SIGINT, SIGTERM)
	"time"      // Для таймаутов

	"github.com/joho/godotenv" // Для загрузки переменных окружения
	"gorm.io/driver/postgres"  // Драйвер PostgreSQL для GORM
	"gorm.io/gorm"

	"github.com/PlastMIke/telegram-bot-platform/internal/api"
	"github.com/PlastMIke/telegram-bot-platform/internal/config"
)

func main() {
	// Загружаем .env файл
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using system environment variables")
	}
	// Загружаем конфигурацию из переменных окружения
	config := config.LoadConfig()

	// Подключаемся к PostgreSQL
	// gorm.Open возвращает (*gorm.DB, error)
	db, err := gorm.Open(postgres.Open(config.DatabaseURL), &gorm.Config{})
	if err != nil {
		// log.Fatalf логирует ошибку и завершает программу с кодом 1
		log.Fatalf("failed to connect to the database: %v", err)
	}
	log.Println("✅ Connected to database")

	// Настраиваем роутер
	router := api.SetupRouter(db, config)

	// Создаём HTTP сервер
	srv := &http.Server{
		Addr:    ":" + config.Port,
		Handler: router,
	}

	// Запускаем сервер в горутине (чтобы не блокировать основной поток)
	go func() {
		log.Printf("🚀 Server is running on %s", config.Port)
		// ListenAndServe блокирует выполнение, пока сервер работает
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("failed to start server: %v", err)
		}
	}()

	// Graceful shutdown — ждём сигнала от ОС (Ctrl+C или docker stop)
	quit := make(chan os.Signal, 1)
	// Notify начинает принимать сигналы SIGINT (Ctrl+C) и SIGTERM (docker stop)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit // Блокирует выполнение, пока не получим сигнал
	log.Println("🛑 Server is shutting down...")

	// Даём серверу 5 секунд на завершение активных запросов
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel() // Освобождаем ресурсы контекста

	// Shutdown плавно завершает сервер (ждёт завершения всех запросов)
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("server forced to shutdown: %v", err)
	}

	log.Println("✅ Server exited successfully")
}
