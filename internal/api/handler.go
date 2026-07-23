package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt" // Для хеширования паролей
	"gorm.io/gorm"               // GORM для работы с БД

	"github.com/PlastMIke/telegram-bot-platform/internal/config"
	"github.com/PlastMIke/telegram-bot-platform/internal/models"
	"github.com/PlastMIke/telegram-bot-platform/pkg/jwt"
)

// Handler содержит зависимости для HTTP хендлеров
// Это паттерн Dependency Injection — мы передаём зависимости через структуру, а не создаём их внутри хендл
type Handler struct {
	DB     *gorm.DB       // Подключение к БД
	Config *config.Config // Конфигурация приложения
}

// RegisterRequest — структура для запроса регистрации
type RegisterRequest struct {
	Email    string `json:"email" binding:"required,email"`    // binding - валидация gin
	Password string `json:"password" binding:"required,min=6"` // min - минимальная длина пароля
}

// LoginRequest — структура для запроса логина
type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

// BotRequest — структура для создания бота
type BotRequest struct {
	Name  string `json:"name" binding:"required"`  // Имя бота
	Token string `json:"token" binding:"required"` // Токен бота
}

// Register обрабатывает POST /register запрос
func (h *Handler) Register(c *gin.Context) {
	var req RegisterRequest

	// BindJSON автоматически парсит JSON и валидирует поля
	// Если есть ошибки валидации — вернёт 400
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Хешируем пароль с помощью bcrypt
	// bcrypt.GenerateFromPassword возвращает ([]byte, error)
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "faild to hash password"})
		return
	}

	// Создаём пользователя с паролем
	user := models.User{
		Email:        req.Email,
		PasswordHash: string(hashedPassword), // Преобразуем []byte в string
	}

	// Сохраняем в БД
	// Create возвращает ошибку, если пользователь с таким email уже существует
	if err := h.DB.Create(&user).Error; err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "email already exists"})
		return
	}

	// Возвращаем ID созданного пользователя
	c.JSON(http.StatusCreated, gin.H{"user_id": user.ID})
}

// Login обрабатывает POST /login
func (h *Handler) Login(c *gin.Context) {
	var req LoginRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Ищем пользователя по email
	var user models.User
	if err := h.DB.Where("email = ?", req.Email).First(&user).Error; err != nil {
		// Если пользователь не найден — возвращаем общую ошибку (не говорим, что email неверный)
		// Это защита от enumeration attacks (перебора email)
		c.JSON(http.StatusUnauthorized, gin.H{"error:": "invalid login"})
		return
	}

	// Проверяем пароль
	// bcrypt.CompareHashAndPassword сравнивает хеш с паролем
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error:": "invalid password"})
		return
	}

	// Генерируем JWT токен
	token, err := jwt.GenerateToken(h.Config.JWTSecret, user.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "faild to generate token"})
		return
	}

	// Возвращаем токен клиенту
	c.JSON(http.StatusOK, gin.H{"token": token})
}

// CreateBot обрабатывает POST /bots (требует аутентификации)
func (h *Handler) CreateBot(c *gin.Context) {
	// Получаем userID из контекста (установлен в AuthMiddleware)
	userID, exists := c.Get("UserID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid user"})
		return
	}

	var req BotRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Создаём бота
	bot := models.Bot{
		Name:   req.Name,
		Token:  req.Token,
		UserID: userID.(uint), // Преобразуем interface{} в uint
	}

	if err := h.DB.Create(&bot).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create bot"})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"bot_id": bot.ID})
}

// GetBots обрабатывает GET /bots (требует аутентификации)
func (h *Handler) GetBots(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user not authorized"})
		return
	}

	// Получаем всех ботов пользователя
	var bots []models.Bot
	if err := h.DB.Where("user_id = ?", userID).Find(&bots).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get bots"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"bots": bots})
}

// DeleteBot обрабатывает DELETE /bots/:id (требует аутентификации)
func (h *Handler) DeleteBot(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user not authorized"})
		return
	}

	// Получаем ID бота из URL параметра
	botID := c.Param("id")

	// Удаляем бота, но только если он принадлежит текущему пользователю
	// Это защита от IDOR (Insecure Direct Object Reference)
	result := h.DB.Where("id = ? AND user_id = ?", botID, userID.(uint)).Delete(&models.Bot{})
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete bot"})
		return
	}

	// Если ничего не удалилось — значит бот не найден или не принадлежит пользователю
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "bot not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "bot deleted successfully"})
}
