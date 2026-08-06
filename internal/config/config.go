package config

import (
	"fmt"
	"time"

	"github.com/spf13/viper"
	// viper — библиотека для управления конфигурацией.
	// Автор: Steve Francia (spf13), он же автор Hugo и Cobra.
	// Поддерживает: env vars, YAML, JSON, TOML, remote config (etcd, Consul).
)

// Config — production-grade конфигурация с валидацией и типизацией.
// Почему Viper, а не os.Getenv?
// 1. Поддержка multiple sources: env vars, files, remote config (etcd, Consul)
// 2. Автоматическая конвертация типов (string -> int, duration, etc.)
// 3. Валидация обязательных полей
// 4. Hot reload (перечитывание конфига без перезапуска)
type Config struct {
	// Server
	Port        int    `mapstructure:"PORT"`
	Environment string `mapstructure:"ENVIRONMENT"` // development | staging | production
	LogLevel    string `mapstructure:"LOG_LEVEL"`   // debug < info < warn < error

	// Database
	DatabaseURL       string        `mapstructure:"DATABASE_URL"`
	DBMaxConns        int           `mapstructure:"DB_MAX_CONNS"`
	DBMinConns        int           `mapstructure:"DB_MIN_CONNS"`
	DBMaxConnIdle     time.Duration `mapstructure:"DB_MAX_CONN_IDLE"`
	DBMaxConnLifetime time.Duration `mapstructure:"DB_MAX_CONN_LIFETIME"`

	// Redis
	RedisURL      string `mapstructure:"REDIS_URL"`
	RedisPoolSize int    `mapstructure:"REDIS_POOL_SIZE"`
	RedisMinIdle  int    `mapstructure:"REDIS_MIN_IDLE"`

	// NATS
	NatsURL            string        `mapstructure:"NATS_URL"`
	NatsMaxReconnects  int           `mapstructure:"NATS_MAX_RECONNECTS"`
	NatsReconnectWait  time.Duration `mapstructure:"NATS_RECONNECT_WAIT"`
	NatsPingInterval   time.Duration `mapstructure:"NATS_PING_INTERVAL"`
	NatsMaxOutstanding int           `mapstructure:"NATS_MAX_OUTSTANDING"`

	// JWT
	JWTSecret     string        `mapstructure:"JWT_SECRET"`
	JWTExpiration time.Duration `mapstructure:"JWT_EXPIRATION"`

	// Worker
	WorkerConcurrency int `mapstructure:"WORKER_CONCURRENCY"`
	WorkerPrefetch    int `mapstructure:"WORKER_PREFETCH"`

	// Rate Limiting
	RateLimitRPS   int `mapstructure:"RATE_LIMIT_RPS"`   // Requests per second
	RateLimitBurst int `mapstructure:"RATE_LIMIT_BURST"` // Burst size

	// Telemetry
	OTLPEndpoint string `mapstructure:"OTEL_EXPORTER_OTLP_ENDPOINT"`
}

// LoadConfig загружает и валидирует конфигурацию.
func LoadConfig() (*Config, error) {
	v := viper.New()

	// Читаем из переменных окружения
	v.AutomaticEnv()

	// Устанавливаем дефолтные значения
	setDefaults(v)

	// Опционально: читаем из файла (для локальной разработки)
	v.SetConfigName("config")
	v.SetConfigType("yaml")
	v.AddConfigPath(".")
	v.AddConfigPath("./config")
	_ = v.ReadInConfig() // Игнорируем ошибку, если файла нет

	// Десериализуем в структуру
	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
		// Unmarshal преобразует map[string]interface{} → Config.
		// Использует теги mapstructure для маппинга.
		//
		// %w в fmt.Errorf — "wrapping" ошибки.
		// Позволяет вызывающему коду проверить: errors.Is(err, specificError)
	}

	// Валидируем обязательные поля
	if err := validate(&cfg); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	return &cfg, nil
	// Валидация ОТДЕЛЬНА от загрузки.
	// Это позволяет протестировать загрузку и валидацию независимо.
}

func setDefaults(v *viper.Viper) {
	// Server
	v.SetDefault("PORT", 8080)
	v.SetDefault("ENVIRONMENT", "development")
	// По умолчанию development. В production ОБЯЗАТЕЛЬНО задавать явно.
	v.SetDefault("LOG_LEVEL", "info")

	// Database
	v.SetDefault("DB_MAX_CONNS", 50)
	v.SetDefault("DB_MIN_CONNS", 10)
	v.SetDefault("DB_MAX_CONN_IDLE", 5*time.Minute)
	v.SetDefault("DB_MAX_CONN_LIFETIME", 1*time.Hour)

	// Redis
	v.SetDefault("REDIS_POOL_SIZE", 100)
	v.SetDefault("REDIS_MIN_IDLE", 10)

	// NATS
	v.SetDefault("NATS_MAX_RECONNECTS", 60)
	v.SetDefault("NATS_RECONNECT_WAIT", 2*time.Second)
	v.SetDefault("NATS_PING_INTERVAL", 2*time.Minute)
	v.SetDefault("NATS_MAX_OUTSTANDING", 1024)

	// JWT
	v.SetDefault("JWT_EXPIRATION", 24*time.Hour)
	// JWT_SECRET НЕ ИМЕЕТ ДЕФОЛТА. Это намеренно.
	// Если его не задать — validate() вернёт ошибку.

	// Worker
	v.SetDefault("WORKER_CONCURRENCY", 10)
	v.SetDefault("WORKER_PREFETCH", 50)

	// Rate Limiting
	v.SetDefault("RATE_LIMIT_RPS", 100)
	v.SetDefault("RATE_LIMIT_BURST", 200)
}

func validate(cfg *Config) error {
	// Валидация обязательных полей
	if cfg.JWTSecret == "" {
		return fmt.Errorf("JWT_SECRET is required")
		// JWT_SECRET без значения = катастрофа.
		// Все токены будут подписаны пустым ключом = любой может подделать токен.
	}

	if cfg.DatabaseURL == "" {
		return fmt.Errorf("DATABASE_URL is required")
	}

	if cfg.RedisURL == "" {
		return fmt.Errorf("REDIS_URL is required")
	}

	if cfg.NatsURL == "" {
		return fmt.Errorf("NATS_URL is required")
	}

	// Валидация значений
	if cfg.Port < 1 || cfg.Port > 65535 {
		return fmt.Errorf("PORT must be between 1 and 65535")
		// TCP-порт — 16-битное число: 0-65535.
		// 0 — специальный (OS выбирает свободный порт).
		// 1-1023 — привилегированные (требуют root).
	}

	if cfg.DBMaxConns < cfg.DBMinConns {
		return fmt.Errorf("DB_MAX_CONNS must be >= DB_MIN_CONNS")
		// Логическая проверка: max не может быть меньше min.
		// Без этой проверки pool будет вести себя непредсказуемо.
	}

	if cfg.WorkerConcurrency < 1 {
		return fmt.Errorf("WORKER_CONCURRENCY must be >= 1")
		// 0 горутин = сообщения не обрабатываются.
	}

	return nil
}
