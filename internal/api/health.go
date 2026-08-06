package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// HealthHandler возвращает статус сервиса для healthcheck.
// Используется Docker, Kubernetes, load balancer'ами.
func HealthHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status": "healthy",
	})
}
