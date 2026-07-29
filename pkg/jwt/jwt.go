package jwt

import (
	"errors" // Для создания кастомных ошибок
	"fmt"
	"strings" // Для очистки токена от пробелов
	"time"    // Для работы с временем (expiration токена)

	"github.com/golang-jwt/jwt/v5" // Библиотека для работы с JWT
)

// Claims — это данные, которые мы храним внутри JWT токена
// jwt.RegisteredClaims содержит стандартные поля (exp, iat, iss)
type Claims struct {
	UserID               uint `json:"user_id"` // ID пользователя, который владеет токеном
	jwt.RegisteredClaims      // Включаем стандартные поля из пакета jwt
}

// GenerateToken создаёт новый JWT токен для пользователя
// secretKey — секретный ключ для подписи (из конфигурации)
// userID — ID пользователя, для которого создаём токен
func GenerateToken(secretKey string, userID uint) (string, error) {
	// Токен будет действителен 24 часа
	expirationTime := time.Now().Add(24 * time.Hour)

	// Создаём claims с данными пользователя
	claims := &Claims{
		UserID: userID,
		RegisteredClaims: jwt.RegisteredClaims{
			// ExpiresAt — когда токен перестанет быть действительным
			ExpiresAt: jwt.NewNumericDate(expirationTime),
			// IssuedAt — когда токен был создан
			IssuedAt: jwt.NewNumericDate(time.Now()),
			// Issuer — кто выпустил токен (название сервиса)
			Issuer: "telegram-bot-platform",
		},
	}

	// Создаём токен с методом подписи HS256 (HMAC + SHA256)
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	// Подписываем токен секретным ключом и получаем строку
	tokenString, err := token.SignedString([]byte(secretKey))
	if err != nil {
		// Если не удалось подписать — возвращаем ошибку
		return "", err
	}

	return tokenString, nil
}

// ValidateToken проверяет валидность JWT токена и возвращает данные из него
// Возвращает Claims (с userID) или ошибку
func ValidateToken(secretKey, tokenString string) (*Claims, error) {
	// Защита от случайных пробелов и переносов строк при копировании из терминала
	tokenString = strings.TrimSpace(tokenString)

	claims := &Claims{}

	// Парсим токен с функцией проверки подписи
	token, err := jwt.ParseWithClaims(
		tokenString,
		claims,
		// Функция, которая возвращает ключ для проверки подписи
		func(token *jwt.Token) (interface{}, error) {
			// Проверяем, что метод подписи — HS256 (защита от атак)
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
			}
			// Возвращаем секретный ключ для проверки подписи
			return []byte(secretKey), nil
		},
	)
	// Если библиотека jwt нашла проблему (например, не совпадает подпись)
	if err != nil {
		// Оборачиваем ошибку, чтобы точно знать причину (signature is invalid, expired и т.д.)
		return nil, fmt.Errorf("jwt parse error: %w", err)
	}

	// Извлекаем claims из токена
	if !token.Valid {
		return nil, errors.New("token parsed but marked as invalid")
	}

	return claims, nil
}
