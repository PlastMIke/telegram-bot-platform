package api

import (
	"net/http" // Для HTTP статусов
	"strings"  // Для работы со строками (разделение "Bearer <token>")

	"github.com/PlastMIke/telegram-bot-platform/internal/config"
	"github.com/PlastMIke/telegram-bot-platform/pkg/jwt"
	"github.com/gin-gonic/gin" // Gin фреймворк для работы с запросами и ответами
)

// AuthMiddleware проверяет JWT токен в заголовке Authorization
// Если токен валидный — добавляет userID в контекст запроса
func AuthMiddleware(config *config.Config) gin.HandlerFunc {
	// Возвращаем функцию-обработчик
	return func(c *gin.Context) {
		// Получаем заголовок Authorization
		authHeader := c.GetHeader("Authorization")

		// Если заголовок пустой — возвращаем 401 Unauthorized
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "(401 Unauthorized)authorization header is required"})
			c.Abort() // Прерываем выполнение цепочки middleware
			return
		}

		// Ожидаем формат: "Bearer <token>"
		// Разделяем строку на части
		parts := strings.Split(authHeader, " ")
		// Проверяем, что формат правильный (ровно 2 части) и первая часть начинается с "Bearer"
		if len(parts) != 2 || parts[0] != "Bearer" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid authorization header format"})
			c.Abort() // Прерываем выполнение цепочки middleware
			return
		}
		// Получаем токен из второго элемента
		tokenString := parts[1]

		// Валидируем токен
		claims, err := jwt.ValidateToken(config.JWTSecret, tokenString)
		if err != nil {
			// Если токен невалидный — возвращаем 401
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired token"})
			c.Abort()
			return
		}

		// Сохраняем userID в контексте Gin
		// Теперь в хендлерах можно получить userID через c.GetUint("userID")
		c.Set("userID", claims.UserID)

		// Продолжаем выполнение цепочки middleware/хендлеров
		c.Next()
	}
}
