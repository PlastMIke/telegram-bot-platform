package models

import "time"

// Bot — модель таблицы bots.
type Bot struct {
	ID    uint   `gorm:"primaryKey" json:"id"`
	Name  string `gorm:"not null" json:"name"`
	Token string `gorm:"uniqueIndex;not null" json:"token"` // TODO: encrypt token at rest using AES-256-GCM
	// ⚠️ Внимание:
	// В production токен нужно ШИФРОВАТЬ перед сохранением.
	// Если БД утечёт — злоумышленник получит контроль над всеми ботами.
	UserID uint `gorm:"not null;index" json:"user_id"`
	// ⚠️ ПОЧЕМУ НЕ uniqueIndex?
	// Потому что один пользователь может иметь МНОГО ботов.
	// Уникальность была бы на паре (user_id, token), но token
	// уже уникален сам по себе.
	IsActive bool   `gorm:"default:true" json:"is_active"`
	Status   string `gorm:"default:'pending'" json:"status"` // pending | running | stopped | error
	// ⚠️ ПОЧЕМУ STRING, А НЕ ENUM?
	// PostgreSQL имеет тип ENUM, но:
	// 1. Сложно менять (ALTER TYPE ... ADD VALUE)
	// 2. GORM плохо работает с PG ENUM
	// 3. String + валидация в коде — проще и гибче
	//
	// В production можно использовать CHECK constraint:
	// ALTER TABLE bots ADD CONSTRAINT chk_status
	//   CHECK (status IN ('pending','running','stopped','error'));
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (Bot) TableName() string {
	return "bots"
}
