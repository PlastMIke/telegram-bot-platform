package service

import (
	"context"
	"fmt"

	"gorm.io/gorm"

	"github.com/PlastMIke/telegram-bot-platform/internal/events"
	"github.com/PlastMIke/telegram-bot-platform/internal/models"
	"github.com/PlastMIke/telegram-bot-platform/internal/repository"
)

// BotService — бизнес-логика для ботов.
//
// ЧТО ДОЛЖНО БЫТЬ В SERVICE?
// - Бизнес-правила ("бот можно создать только если у пользователя < 10 ботов")
// - Транзакции (Create бота + Create события в outbox)
// - Оркестрация (вызов нескольких репозиториев)
// - Публикация событий
//
// ЧЕГО НЕ ДОЛЖНО БЫТЬ В SERVICE?
// - HTTP-статусов (это ответственность handler'а)
// - SQL-запросов (это ответственность repository)
// - JSON-сериализации (это ответственность handler'а)
type BotService struct {
	db *gorm.DB
	// db нужен для ТРАНЗАКЦИЙ.
	// Repository не управляет транзакциями — это делает Service.
	//
	// ⚠️ ALTERNATIVE: передавать TransactionRunner интерфейс:
	//   type TransactionRunner interface {
	//       Transaction(fn func(tx *gorm.DB) error) error
	//   }
	// Это лучше для тестирования (можно мокать).
	botRepo    *repository.BotRepository
	outboxRepo *repository.OutboxRepository
}

func NewBotService(
	db *gorm.DB,
	botRepo *repository.BotRepository,
	outboxRepo *repository.OutboxRepository,
) *BotService {
	return &BotService{
		db:         db,
		botRepo:    botRepo,
		outboxRepo: outboxRepo,
	}
}

// Create создаёт бота И записывает событие в outbox в ОДНОЙ транзакции.
// ЭТО САМЫЙ ВАЖНЫЙ МЕТОД В ПРОЕКТЕ.
// Это и есть Outbox Pattern: бизнес-операция и событие атомарны.
func (s *BotService) Create(ctx context.Context, userID uint, name, token string) (*models.Bot, error) {
	// ─── ШАГ 1: Подготавливаем данные ───
	bot := &models.Bot{
		Name:     name,
		Token:    token,
		UserID:   userID,
		IsActive: true,
		Status:   "pending",
		// Status = "pending", потому что бот ещё не запущен Worker'ом.
		// Worker изменит на "running" после успешного запуска.
	}

	event := events.BotCreatedEvent{
		UserID:  userID,
		BotName: name,
		Token:   token,
		// BotID пока НЕ ЗАПОЛНЯЕМ — он будет известен только после INSERT.
		// Заполним его после Create.
	}

	// ─── ШАГ 2: АТОМАРНАЯ ТРАНЗАКЦИЯ ─── бот + событие в outbox
	err := s.db.Transaction(func(tx *gorm.DB) error {
		// db.Transaction — обёртка вокруг BEGIN / COMMIT / ROLLBACK.
		//
		// Как это работает:
		// 1) BEGIN
		// 2) Выполняется fn(tx)
		// 3) Если fn вернула nil → COMMIT
		//    Если fn вернула error → ROLLBACK
		//
		// ⚠️ ВАЖНО: внутри fn используем ТОЛЬКО tx, не s.db!
		// Если использовать s.db — запросы будут ВНЕ транзакции.

		// 1. Создаём бота
		if err := tx.WithContext(ctx).Create(bot).Error; err != nil {
			return fmt.Errorf("failed to create bot: %w", err)
			// Возвращаем error → Transaction сделает ROLLBACK.
			// Ни бот, ни событие не будут созданы.
		}

		// 2. После Create у bot.ID уже заполнен (GORM делает это автоматически LastInsertId)
		event.BotID = bot.ID
		// Теперь мы знаем ID бота и можем включить его в событие.

		// 3. Записываем событие в outbox в той же транзакции
		if err := s.outboxRepo.CreateEventInTx(ctx, tx, // ← ПЕРЕДАЁМ tx, НЕ s.db!
			events.TopicBotCreated, event); err != nil {
			return fmt.Errorf("failed to create outbox event: %w", err)
			// Возвращаем error → ROLLBACK.
			// Бот тоже НЕ будет создан (хотя INSERT уже выполнен).
			// Это атомарность: всё или ничего.
		}

		return nil
		// nil → COMMIT. Оба INSERT'а закоммичены атомарно.
	})

	if err != nil {
		return nil, err
	}

	// ─── ШАГ 3: Возвращаем результат ───
	return bot, nil
	// В этот момент:
	// - Бот создан в БД ✅
	// - Событие в outbox_events ✅
	// - NATS ещё НЕ знает о боте (Publisher обработает позже)
	//
	// API возвращает 201 Created СРАЗУ, не дожидаясь Worker'а.
	// Это и есть асинхронность: пользователь не ждёт запуска бота.
}

// GetByUserID возвращает всех ботов пользователя.
func (s *BotService) GetByUserID(ctx context.Context, userID uint) ([]models.Bot, error) {
	return s.botRepo.FindByUserID(ctx, userID)
	// Просто делегируем в repository.
	//
	// ЗАЧЕМ НУЖЕН SERVICE, ЕСЛИ ОН ПРОСТО ДЕЛЕГИРУЕТ?
	// 1. Единообразие: все операции идут через service
	// 2. В будущем добавим бизнес-логику (фильтрация, пагинация)
	// 3. В будущем добавим кэширование (Redis)
}

// Delete удаляет бота и записывает событие в outbox.
func (s *BotService) Delete(ctx context.Context, botID, userID uint) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		// 1. Проверяем, что бот существует и принадлежит пользователю
		var bot models.Bot
		if err := tx.WithContext(ctx).Where("id = ? AND user_id = ?", botID, userID).First(&bot).Error; err != nil {
			return fmt.Errorf("bot not found or access denied")
			// ⚠️ SECURITY: проверяем user_id в ЗДЕСЬ!
			// Без этого любой пользователь мог бы удалить чужого бота,
			// зная его ID (Insecure Direct Object Reference — IDOR).
			//
			// OWASP Top 10: Broken Object Level Authorization.
		}

		// 2. Удаляем бота
		if err := tx.WithContext(ctx).Delete(&bot).Error; err != nil {
			return fmt.Errorf("failed to delete bot: %w", err)
		}

		// 3. Записываем событие в outbox
		event := events.BotDeletedEvent{
			BotID:  botID,
			UserID: userID,
		}
		if err := s.outboxRepo.CreateEventInTx(ctx, tx, events.TopicBotDeleted, event); err != nil {
			return fmt.Errorf("failed to create outbox event: %w", err)
		}

		return nil
	})
}
