package api

import (
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/PlastMIke/telegram-bot-platform/internal/config"
)

// SetupRouter настраивает все маршруты API
func SetupRouter(db *gorm.DB, config *config.Config) *gin.Engine {
	// gin.Default() создаёт движок с middleware Logger и Recovery
	// Logger — логирует все запросы
	// Recovery — восстанавливается после паники (чтобы сервер не падал)
	r := gin.Default()

	// Создаём хендлер с зависимостями
	handler := &Handler{
		DB:     db,
		Config: config,
	}

	// Группа маршрутов /api/v1
	// Это best practice — версионирование API
	v1 := r.Group("/api/v1")
	{
		// Публичные маршруты (не требуют аутентификации)
		v1.POST("/register", handler.Register)
		v1.POST("/login", handler.Login)

		// Защищённые маршруты (требуют JWT токен)
		// AuthMiddleware проверяет токен перед выполнением хендлера
		protected := v1.Group("")
		protected.Use(AuthMiddleware(config))
		{
			protected.POST("/bots", handler.CreateBot)
			protected.GET("/bots", handler.GetBots)
			protected.DELETE("/bots/:id", handler.DeleteBot)
		}
	}
	return r
}
