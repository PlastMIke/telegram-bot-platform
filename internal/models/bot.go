package models

import "time"

// Bot — модель таблицы bots.
type Bot struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Name      string    `gorm:"not null" json:"name"`
	Token     string    `gorm:"uniqueIndex;not null" json:"token"`
	UserID    uint      `gorm:"not null;index" json:"user_id"`
	IsActive  bool      `gorm:"default:true" json:"is_active"`
	Status    string    `gorm:"default:'pending'" json:"status"` // pending | running | stopped | error
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (Bot) TableName() string {
	return "bots"
}
