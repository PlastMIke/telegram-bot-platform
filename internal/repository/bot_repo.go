package repository

import (
	"context"
	"errors"

	"github.com/PlastMIke/telegram-bot-platform/internal/models"
	"gorm.io/gorm"
)

type BotRepository struct {
	db *gorm.DB
}

func NewBotRepository(db *gorm.DB) *BotRepository {
	return &BotRepository{db: db}
}

// Create создаёт нового бота.
func (r *BotRepository) Create(ctx context.Context, bot *models.Bot) error {
	return r.db.WithContext(ctx).Create(bot).Error
}

// FindByUserID возвращает всех ботов пользователя.
func (r *BotRepository) FindByUserID(ctx context.Context, userID uint) ([]models.Bot, error) {
	var bots []models.Bot
	err := r.db.WithContext(ctx).Where("user_id = ?", userID).Find(&bots).Error
	return bots, err
}

// FindByID возвращает бота по ID.
func (r *BotRepository) FindByID(ctx context.Context, id uint) (*models.Bot, error) {
	var bot models.Bot
	err := r.db.WithContext(ctx).First(&bot, id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &bot, nil
}

// Delete удаляет бота, но только если он принадлежит пользователю.
// Возвращает true, если бот удалён, false — если не найден или не принадлежит.
func (r *BotRepository) Delete(ctx context.Context, botID, userID uint) (bool, error) {
	result := r.db.WithContext(ctx).
		Where("id = ? AND user_id = ?", botID, userID).
		Delete(&models.Bot{})

	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected > 0, nil
}
