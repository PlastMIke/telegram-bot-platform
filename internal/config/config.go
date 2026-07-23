package config

import (
	"os" // Для чтения переменных окружения
)

// Config хранит все настройки приложения
// Используем структуру, чтобы не передавать 10 параметров в функции
type Config struct {
	// Строка подключения к PostgreSQL
	DatabaseURL string
	// Порт, на котором будет работать API
	Port string
	// Секретный ключ для подписи JWT токенов
	JWTSecret string
}

// LoadConfig загружает конфигурацию из переменных окружения
// Если переменная не задана — используем значение по умолчанию
func LoadConfig() *Config {
	return &Config{
		// Getenv возвращает пустую строку, если переменная не задана
		// Поэтому используем вспомогательную функцию getEnvWithDefault
		DatabaseURL: getEnvWithDefault("DATABASE_URL",
			"postgres://botuser:botpass@localhost:5432/botdb?sslmode=disable"),
		Port:      getEnvWithDefault("PORT", "8080"),
		JWTSecret: getEnvWithDefault("JWT_SECRET", "your_jwt_secret"),
	}
}

// getEnvWithDefault читает переменную окружения или возвращает дефолтное значение
// Это вспомогательная функция, чтобы не дублировать код
func getEnvWithDefault(key, defaultValue string) string {
	// os.LookupEnv возвращает (значение, существует ли переменная)
	// В отличие от os.Getenv, который всегда возвращает строку (даже если переменная не задана)
	value, exists := os.LookupEnv(key)

	// Если переменная не задана — возвращаем дефолтное значение
	if !exists {
		return defaultValue
	}

	return value
}
