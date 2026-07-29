package repository

import (
	"context"
	"encoding/json"

	"github.com/PlastMIke/telegram-bot-platform/internal/models"
	"gorm.io/gorm"
)

type OutboxRepository struct {
	db *gorm.DB
}

func NewOutboxRepository(db *gorm.DB) *OutboxRepository {
	return &OutboxRepository{db: db}
}

// CreateEventInTx создаёт событие в outbox в рамках СУЩЕСТВУЮЩЕЙ транзакции.
// Это КЛЮЧЕВОЙ метод Outbox Pattern: бизнес-операция и событие атомарны.
//
// ВАЖНО: tx — это транзакция из бизнес-логики, а не r.db.
// Если использовать r.db, событие запишется в отдельной транзакции,
// и мы потеряем гарантии атомарности.
func (r *OutboxRepository) CreateEventInTx(
	ctx context.Context,
	tx *gorm.DB,
	eventType string,
	payload interface{},
) error {
	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	event := models.OutboxEvent{
		EventType: eventType,
		Payload:   string(jsonPayload),
	}

	return tx.WithContext(ctx).Create(&event).Error
}

// GetUnprocessedEvents возвращает партию необработанных событий.
// LIMIT нужен, чтобы не выгрести всю таблицу, если накопилось много событий.
func (r *OutboxRepository) GetUnprocessedEvents(ctx context.Context, limit int) ([]models.OutboxEvent, error) {
	var events []models.OutboxEvent
	err := r.db.WithContext(ctx).
		Where("processed = ?", false).
		Order("created_at ASC"). // Обрабатываем в порядке создания (FIFO)
		Limit(limit).
		Find(&events).Error
	return events, err
}

// MarkAsProcessed помечает событие как обработанное.
func (r *OutboxRepository) MarkAsProcessed(ctx context.Context, eventID uint) error {
	return r.db.WithContext(ctx).
		Model(&models.OutboxEvent{}).
		Where("id = ?", eventID).
		Update("processed", true).Error
}
