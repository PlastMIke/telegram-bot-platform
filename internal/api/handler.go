package api

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/PlastMIke/telegram-bot-platform/internal/service"
)

// Handler содержит зависимости для HTTP-хендлеров.
// Используем сервисы, а не репозитории напрямую — это правильный слой абстракции.
// Handler — HTTP-слой. Знает про HTTP, JSON, статус-коды.
// НЕ знает про SQL, бизнес-правила, NATS.
type Handler struct {
	authService *service.AuthService
	botService  *service.BotService
	// Handler зависит от ИНТЕРФЕЙСОВ сервисов.
	// В production лучше:
	//   type AuthService interface { Register(...); Login(...) }
	// Но для MVP конкретные типы — ок.
}

func NewHandler(authService *service.AuthService, botService *service.BotService) *Handler {
	return &Handler{
		authService: authService,
		botService:  botService,
	}
}

// Request DTOs
type RegisterRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=6"`
	// binding:"..." — валидация Gin (использует go-playground/validator).
	//
	// required — поле обязательно
	// email — должно быть валидным email
	// min=6 — минимум 6 символов
	//
	// Если валидация не пройдёт — ShouldBindJSON вернёт ошибку,
	// и мы вернём 400 Bad Request с описанием.
	//
	// ⚠️ В production нужно больше валидации:
	// - max=255 для email
	// - Проверка сложности пароля (regex)
	// - Rate limiting на /register (защита от ботов)
}

type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

type BotRequest struct {
	Name  string `json:"name" binding:"required"`
	Token string `json:"token" binding:"required,matches=^[0-9]+:.+$"`
	// ⚠️ В production валидируем формат токена Telegram:
	// binding:"required,matches=^[0-9]+:.+$"
}

// Register обрабатывает POST /api/v1/register
func (h *Handler) Register(c *gin.Context) {
	// ─── ШАГ 1: Парсим и валидируем запрос ───
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
		// 400 Bad Request — клиент отправил невалидные данные.
		// Возвращаем описание ошибки, чтобы клиент мог исправить.
		//
		// ⚠️ В production НЕ возвращаем err.Error() напрямую!
		// Он может содержать внутренние детали.
		// Лучше: маппинг ошибок в человекочитаемые сообщения.
	}

	// ─── ШАГ 2: Вызываем сервис ───
	user, err := h.authService.Register(c.Request.Context(), req.Email, req.Password)
	// c.Request.Context() — контекст HTTP-запроса.
	// Если клиент отменит запрос — контекст отменится,
	// и SQL-запрос тоже отменится.

	if err != nil {
		// ─── ШАГ 3: Маппим ошибки в HTTP-статусы ───
		if err.Error() == "user already exists" {
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
			return
			// 409 Conflict — ресурс уже существует.
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to register user"})
		return
		// 500 Internal Server Error — неожиданная ошибка.
		// НЕ возвращаем детали (security).
	}

	// ─── ШАГ 4: Возвращаем результат ───
	c.JSON(http.StatusCreated, gin.H{"user_id": user.ID})
	// 201 Created — ресурс создан.
	//
	// ⚠️ ALTERNATIVE: 201 + Location header:
	// c.Header("Location", fmt.Sprintf("/api/v1/users/%d", user.ID))
	// Это REST best practice, но не всегда нужно.
}

// Login обрабатывает POST /api/v1/login
func (h *Handler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	token, err := h.authService.Login(c.Request.Context(), req.Email, req.Password)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
		// 401 Unauthorized — неверные credentials.
		//
		// ⚠️ SECURITY: НЕ говорим "email не найден" vs "неверный пароль".
		// Всегда говорим: "invalid credentials".
		// Иначе злоумышленник может перебирать email'ы (enumeration attack).
	}

	c.JSON(http.StatusOK, gin.H{"token": token})
	// 200 OK + токен.
	//
	// ⚠️ ALTERNATIVE: HttpOnly cookie вместо JSON.
	// Cookie защищён от XSS (JavaScript не может прочитать).
	// Но усложняет работу с мобильными приложениями.
}

// CreateBot обрабатывает POST /api/v1/bots
func (h *Handler) CreateBot(c *gin.Context) {
	// ─── ШАГ 1: Получаем userID из контекста ───
	userID := c.GetUint("userID")
	// GetUint — типизированный геттер Gin.
	// Возвращает 0, если ключ не найден или тип не uint.

	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid user"})
		return
		// userID = 0 означает, что middleware не установил значение.
		// Это не должно происходить (middleware проверяет токен),
		// но защита от багов.
	}

	// ─── ШАГ 2: Парсим запрос ───
	var req BotRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// ─── ШАГ 3: Вызываем сервис ───
	bot, err := h.botService.Create(c.Request.Context(), userID, req.Name, req.Token)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create bot"})
		return
	}

	// ─── ШАГ 4: Возвращаем результат ───
	c.JSON(http.StatusCreated, gin.H{
		"bot_id":  bot.ID,
		"message": "Bot created. Worker will start it shortly.",
	})
}

// GetBots обрабатывает GET /api/v1/bots
func (h *Handler) GetBots(c *gin.Context) {
	userID := c.GetUint("userID")
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid user"})
		return
	}

	bots, err := h.botService.GetByUserID(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch bots"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"bots": bots})
}

// DeleteBot обрабатывает DELETE /api/v1/bots/:id
func (h *Handler) DeleteBot(c *gin.Context) {
	userID := c.GetUint("userID")
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid user"})
		return
	}

	// Парсим ID из URL
	var req struct {
		ID uint `uri:"id" binding:"required"`
	}
	if err := c.ShouldBindUri(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid bot id"})
		return
	}

	if err := h.botService.Delete(c.Request.Context(), req.ID, userID); err != nil {
		if err.Error() == "bot not found or access denied" {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete bot"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "bot deleted"})
}
