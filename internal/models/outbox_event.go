package models

import "time"

// OutboxEvent — техническая таблица для Outbox Pattern.
// Каждое событие, которое нужно отправить в NATS, сначала пишется сюда
// в той же транзакции, что и бизнес-операция.
// Outbox Publisher читает необработанные события и публикует их в NATS.
type OutboxEvent struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	EventType string    `gorm:"not null;index" json:"event_type"`   // "bot.created"
	Payload   string    `gorm:"type:jsonb;not null" json:"payload"` // JSON-сериализованное событие
	CreatedAt time.Time `gorm:"not null;index" json:"created_at"`
	Processed bool      `gorm:"default:false;index" json:"processed"` // Обработано ли Outbox Publisher'ом
}

func (OutboxEvent) TableName() string {
	return "outbox_events"
}
