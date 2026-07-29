package api

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/PlastMIke/telegram-bot-platform/internal/service"
)

// Handler содержит зависимости для HTTP-хендлеров.
// Используем сервисы, а не репозитории напрямую — это правильный слой абстракции.
type Handler struct {
	authService *service.AuthService
	botService  *service.BotService
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
}

type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

type BotRequest struct {
	Name  string `json:"name" binding:"required"`
	Token string `json:"token" binding:"required"`
}

// Register обрабатывает POST /api/v1/register
func (h *Handler) Register(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	user, err := h.authService.Register(c.Request.Context(), req.Email, req.Password)
	if err != nil {
		if err.Error() == "user already exists" {
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to register user"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"user_id": user.ID})
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
	}

	c.JSON(http.StatusOK, gin.H{"token": token})
}

// CreateBot обрабатывает POST /api/v1/bots
func (h *Handler) CreateBot(c *gin.Context) {
	userID := c.GetUint("userID")
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid user"})
		return
	}

	var req BotRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	bot, err := h.botService.Create(c.Request.Context(), userID, req.Name, req.Token)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create bot"})
		return
	}

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
