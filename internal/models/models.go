package models

import (
	"time" // Для работы с временными метками
)

// User представляет пользователя системы
// GORM автоматически создаст таблицу "users" на основе этой структуры
type User struct {
	// ID пользователя (первичный ключ), auto-increment
	// GORM автоматически добавит это поле
	ID uint `gorm:"primaryKey" json:"id"`

	// Email должен быть уникальным
	// Добавляем индекс для быстрого поиска по email
	Email string `gorm:"uniqueIndex; not null" json:"email"`

	// Пароль хранится в виде хэша, поэтому не сохраняем его в JSON (никогда не храним в открытом виде!)
	PasswordHash string `gorm:"not null" json:"-"` // json:"-" означает, что поле не будет сериализоваться в JSON

	// CreatedAt и UpdatedAt GORM заполняет автоматически
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	// HasMany означает, что у одного пользователя может быть много ботов
	// GORM автоматически создаст связь через поле user_id в таблице bots
	Bots []Bot `gorm:"foreignKey:UserID" json:"bots,omitempty"`
}

// Bot представляет Telegram-бота
type Bot struct {
	// ID бота (первичный ключ), auto-increment
	ID uint `gorm:"primaryKey" json:"id"`

	// Название бота (для удобства пользователя)
	Name string `gorm:"not null" json:"name"`

	// Токен Telegram Bot API (получается у @BotFather)
	// Храним в зашифрованном виде в продакшене, но для MVP оставим так
	Token string `gorm:"not null" json:"token"`

	// UserID — внешний ключ на таблицу users
	// Это поле GORM создаст автоматически из-за тега foreignKey
	UserID uint `gorm:"not null" json:"user_id"`

	// IsActive — флаг, активен ли бот (можно включать/выключать)
	IsActive bool `gorm:"default:true" json:"is_active"`

	// CreatedAt и UpdatedAt GORM заполняет автоматически
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// TableName переопределяет имя таблицы
// По умолчанию GORM использует множественное число (users, bots)
// Но если хочешь кастомное имя — используй этот метод
func (User) TableName() string {
	return "users"
}

func (Bot) TableName() string {
	return "bots"
}
