package api

import (
	"github.com/PlastMIke/telegram-bot-platform/internal/config"
	"github.com/gin-gonic/gin"
)

// SetupRouter настраивает все маршруты API.
func SetupRouter(handler *Handler, cfg *config.Config) *gin.Engine {
	// gin.Default() включает Logger и Recovery middleware.
	r := gin.Default()

	// Healthcheck для Docker/Kubernetes
	r.GET("/health", HealthHandler)

	// Версионирование API — best practice
	v1 := r.Group("/api/v1")
	{
		// Публичные маршруты
		v1.POST("/register", handler.Register)
		v1.POST("/login", handler.Login)

		// Защищённые маршруты
		protected := v1.Group("")
		protected.Use(AuthMiddleware(cfg))
		{
			protected.POST("/bots", handler.CreateBot)
			protected.GET("/bots", handler.GetBots)
			protected.DELETE("/bots/:id", handler.DeleteBot)
		}
	}

	return r
}
