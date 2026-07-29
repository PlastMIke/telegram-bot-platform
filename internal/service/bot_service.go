package service

import (
	"context"
	"fmt"

	"gorm.io/gorm"

	"github.com/PlastMIke/telegram-bot-platform/internal/events"
	"github.com/PlastMIke/telegram-bot-platform/internal/models"
	"github.com/PlastMIke/telegram-bot-platform/internal/repository"
)

type BotService struct {
	db         *gorm.DB
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
// Это и есть Outbox Pattern: бизнес-операция и событие атомарны.
func (s *BotService) Create(ctx context.Context, userID uint, name, token string) (*models.Bot, error) {
	bot := &models.Bot{
		Name:     name,
		Token:    token,
		UserID:   userID,
		IsActive: true,
		Status:   "pending",
	}

	event := events.BotCreatedEvent{
		UserID:  userID,
		BotName: name,
		Token:   token,
	}

	// АТОМАРНАЯ ТРАНЗАКЦИЯ: бот + событие в outbox
	err := s.db.Transaction(func(tx *gorm.DB) error {
		// 1. Создаём бота
		if err := tx.WithContext(ctx).Create(bot).Error; err != nil {
			return fmt.Errorf("failed to create bot: %w", err)
		}

		// 2. После Create у bot.ID уже заполнен (GORM делает это автоматически)
		event.BotID = bot.ID

		// 3. Записываем событие в outbox в той же транзакции
		if err := s.outboxRepo.CreateEventInTx(ctx, tx, events.TopicBotCreated, event); err != nil {
			return fmt.Errorf("failed to create outbox event: %w", err)
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return bot, nil
}

// GetByUserID возвращает всех ботов пользователя.
func (s *BotService) GetByUserID(ctx context.Context, userID uint) ([]models.Bot, error) {
	return s.botRepo.FindByUserID(ctx, userID)
}

// Delete удаляет бота и записывает событие в outbox.
func (s *BotService) Delete(ctx context.Context, botID, userID uint) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		// 1. Проверяем, что бот существует и принадлежит пользователю
		var bot models.Bot
		if err := tx.WithContext(ctx).Where("id = ? AND user_id = ?", botID, userID).First(&bot).Error; err != nil {
			return fmt.Errorf("bot not found or access denied")
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
