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

// CreateEventInTx — КЛЮЧЕВОЙ метод Outbox Pattern.
//
// ОБРАТИ ВНИМАНИЕ НА СИГНАТУРУ:
//
//	CreateEventInTx(ctx, tx *gorm.DB, ...)
//	                       ^^^^^^^^^^^
//
// Мы принимаем ТРАНЗАКЦИЮ (tx), а не r.db!
//
// ПОЧЕМУ?
// Outbox Pattern работает ТОЛЬКО если бизнес-операция и событие
// в ОДНОЙ транзакции:
//
//	db.Transaction(func(tx *gorm.DB) error {
//	    tx.Create(bot)                              // ┐
//	    outboxRepo.CreateEventInTx(ctx, tx, event)  // ├─ ОДНА транзакция
//	    return nil                                  // ┘
//	})
//
// Если бы CreateEventInTx использовал r.db вместо tx:
//
//	db.Transaction(func(tx *gorm.DB) error {
//	    tx.Create(bot)                    // Транзакция A
//	    outboxRepo.CreateEvent(r.db, ...) // Транзакция B (отдельная!)
//	    return nil
//	})
//
// Тогда при сбое между A и B:
// - Бот создан (A закоммитилась)
// - Событие НЕ создано (B не закоммитилась)
// → Outbox Pattern сломан!
func (r *OutboxRepository) CreateEventInTx(
	ctx context.Context,
	tx *gorm.DB, // ← ТРАНЗАКЦИЯ, не r.db!
	eventType string,
	payload interface{},
) error {
	jsonPayload, err := json.Marshal(payload)
	// Сериализуем payload в JSON.
	//
	// payload interface{} — принимаем ЛЮБУЮ структуру.
	// Это делает метод универсальным: можно передать BotCreatedEvent,
	// BotDeletedEvent, или любую другую структуру.
	//
	// ⚠️ ALTERNATIVE: использовать []byte вместо interface{}.
	// Тогда сериализация — ответственность вызывающего.
	// Это более явно, но менее удобно.
	if err != nil {
		return err
		// Ошибка маршалинга практически невозможна для простых структур,
		// но может возникнуть при cyclic references или unsupported types.
	}

	event := models.OutboxEvent{
		EventType: eventType,
		Payload:   string(jsonPayload),
		// Конвертируем []byte → string для сохранения в JSONB.
		// PostgreSQL сам распарсит JSON при вставке.
	}

	return tx.WithContext(ctx).Create(&event).Error
	// Используем tx, а не r.db!
	// CreatedAt заполнится автоматически (GORM).
	// Processed = false (default из БД).
}

// GetUnprocessedEvents возвращает партию необработанных событий.
// LIMIT нужен, чтобы не выгрести всю таблицу, если накопилось много событий.
func (r *OutboxRepository) GetUnprocessedEvents(ctx context.Context, limit int) ([]models.OutboxEvent, error) {
	var events []models.OutboxEvent
	err := r.db.WithContext(ctx).
		Where("processed = ?", false).
		Order("created_at ASC").
		Limit(limit).
		Find(&events).Error
	// Where("processed = ?", false) — только необработанные.
	//
	// Order("created_at ASC") — FIFO (First In, First Out).
	// События обрабатываются в порядке создания.
	//
	// Limit(limit) — НЕ выгребаем всю таблицу.
	// Если накопилось 10000 событий — обрабатываем по 100 за раз.
	// Это предотвращает:
	// 1. Memory exhaustion (10000 событий × 1KB = 10MB в памяти)
	// 2. Долгую блокировку (SELECT ... FOR UPDATE)
	// 3. Timeout (слишком долгий запрос)
	//
	// Find(&events) — в отличие от First(), не возвращает ошибку,
	// если записей нет. Возвращает пустой слайс.
	return events, err
}

// MarkAsProcessed помечает событие как обработанное.
func (r *OutboxRepository) MarkAsProcessed(ctx context.Context, eventID uint) error {
	return r.db.WithContext(ctx).
		Model(&models.OutboxEvent{}).
		Where("id = ?", eventID).
		Update("processed", true).Error
	// Model(&models.OutboxEvent{}) — указывает GORM, какую таблицу обновлять.
	// Без этого GORM не знает, к какой таблице относится Update.
	//
	// ⚠️ IDEMPOTENCY:
	// Если Publisher опубликовал событие, но упал ДО MarkAsProcessed,
	// при следующем запуске событие будет опубликовано ПОВТОРНО.
	//
	// Поэтому consumer (Worker) ДОЛЖЕН БЫТЬ ИДЕМПOTENTНЫМ:
	// обработка одного и того же события дважды не должна ломать систему.
	//
	// Пример: если Worker запускает бота по bot_id,
	// повторный запуск того же bot_id не должен создавать дубликат.
}
