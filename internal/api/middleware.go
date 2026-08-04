package api

import (
	"log"
	"net/http" // Для HTTP статусов
	"strings"  // Для работы со строками (разделение "Bearer <token>")

	"github.com/PlastMIke/telegram-bot-platform/internal/config"
	"github.com/PlastMIke/telegram-bot-platform/pkg/jwt"
	"github.com/gin-gonic/gin" // Gin фреймворк для работы с запросами и ответами
)

// AuthMiddleware — проверяет JWT-токен в заголовке Authorization.
//
// ЧТО ТАКОЕ MIDDLEWARE?
// Это функция, которая выполняется ДО хендлера.
// Цепочка: Request → Middleware1 → Middleware2 → Handler → Response
//
// Gin-специфика: middleware — это gin.HandlerFunc,
// которая может вызвать c.Next() (продолжить) или c.Abort() (остановить).
func AuthMiddleware(cfg *config.Config) gin.HandlerFunc {
	// Возвращаем gin.HandlerFunc (замыкание).
	// cfg "захватывается" замыканием — не нужно передавать в каждый запрос.

	return func(c *gin.Context) {
		// c *gin.Context — контекст запроса.
		// Содержит: Request, ResponseWriter, параметры, значения.

		// ─── ШАГ 1: Получаем заголовок ───
		authHeader := c.GetHeader("Authorization")
		// Ожидаемый формат: "Bearer eyJhbGciOiJIUzI1NiIs..."
		//
		// ПОЧЕМУ "Bearer"?
		// RFC 6750 (OAuth 2.0 Bearer Token Usage).
		// "Bearer" означает: "тот, кто предъявит этот токен,
		// имеет доступ". Как билет в кино: у кого билет — тот и зритель.
		// Если заголовок пустой — возвращаем 401 Unauthorized
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "(401 Unauthorized)authorization header is required"})
			c.Abort()
			// Abort() прерывает цепочку middleware.
			// Без Abort() выполнение продолжилось бы до хендлера!
			return
		}

		// ─── ШАГ 2: Разбираем формат ───
		parts := strings.Split(authHeader, " ")

		// "Bearer eyJ..." → ["Bearer", "eyJ..."]
		//
		// ⚠️ Edge cases:
		// "Bearer" (без токена) → ["Bearer"]
		// "Bearer token extra" → ["Bearer", "token", "extra"]
		// "bearer eyJ..." (нижний регистр) → ["bearer", "eyJ..."]

		if len(parts) != 2 || parts[0] != "Bearer" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid authorization header format"})
			c.Abort() // Прерываем выполнение цепочки middleware
			return
		}

		// Проверяем: ровно 2 части И первая = "Bearer".
		//
		// ⚠️ В production можно быть более лояльным:
		// strings.EqualFold(parts[0], "Bearer") — без учёта регистра.
		// RFC 6750 говорит, что "Bearer" case-insensitive.

		tokenString := parts[1]

		// ─── ШАГ 3: Валидируем токен ───
		claims, err := jwt.ValidateToken(cfg.JWTSecret, tokenString)
		if err != nil {
			// Логируем ошибку, чтобы видеть, если ключи не совпадают
			log.Printf("❌ JWT Validation Error: %v | Secret length used: %d", err, len(cfg.JWTSecret))
			// Но НЕ возвращаем детали клиенту (security)!
			// Клиент видит только "invalid or expired token".
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired token"})
			c.Abort()
			return
		}

		// ─── ШАГ 4: Сохраняем userID в контекст ───
		c.Set("userID", claims.UserID)
		// Теперь в хендлере можно получить:
		//   userID := c.GetUint("userID")
		//
		// ⚠️ ВАЖНО: ключ "userID" (с маленькой 'u').
		// В хендлере тоже должно быть "userID", не "UserID".

		// ─── ШАГ 5: Продолжаем цепочку ───

		c.Next()
		// Без Next() хендлер НЕ БУДЕТ вызван!
		// Next() передаёт управление следующему middleware или хендлеру.
	}
}
