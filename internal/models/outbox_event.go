package models

import "time"

// OutboxEvent — техническая таблица для Outbox Pattern.
// Каждое событие, которое нужно отправить в NATS, сначала пишется сюда
// в той же транзакции, что и бизнес-операция.
// Outbox Publisher читает необработанные события и публикует их в NATS.
type OutboxEvent struct {
	ID        uint   `gorm:"primaryKey" json:"id"`
	EventType string `gorm:"not null;index" json:"event_type"`   // "bot.created"
	Payload   string `gorm:"type:jsonb;not null" json:"payload"` // JSON-сериализованное событие
	// JSONB vs JSON:
	// - JSON хранит текст как есть (медленный поиск)
	// - JSONB хранит бинарное представление (быстрый поиск, индексы)
	// - JSONB занимает чуть больше места, но запросы в разы быстрее
	//
	// В production можно создать GIN-индекс для поиска внутри JSON:
	// CREATE INDEX idx_payload ON outbox_events USING GIN (payload);
	CreatedAt time.Time `gorm:"not null;index" json:"created_at"`
	Processed bool      `gorm:"default:false;index" json:"processed"` // Обработано ли Outbox Publisher'ом
	// ⚠️ В production нужна периодическая ОЧИСТКА обработанных событий.
	// Иначе таблица будет расти бесконечно.
	// Решение: cron-задача или pg_cron:
	// DELETE FROM outbox_events WHERE processed = true AND created_at < NOW() - INTERVAL '7 days';
}

func (OutboxEvent) TableName() string {
	return "outbox_events"
}
