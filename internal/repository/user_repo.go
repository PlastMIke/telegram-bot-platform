package repository

import (
	"context"
	"errors"

	"github.com/PlastMIke/telegram-bot-platform/internal/models"
	"gorm.io/gorm"
)

type UserRepository struct {
	db *gorm.DB
	// ⚠️ ПОЧЕМУ *gorm.DB, А НЕ ИНТЕРФЕЙС?
	// Для MVP это допустимо. Но в production лучше:
	//   type UserRepository struct { db DBTX }
	//   type DBTX interface {
	//       Create(value interface{}) *gorm.DB
	//       First(out interface{}, where ...interface{}) *gorm.DB
	//       ...
	//   }
	// Это позволяет мокать БД в unit-тестах без реальной БД.
	//
	// Альтернатива: sqlmock (github.com/DATA-DOG/go-sqlmock)
	// для мокинга на уровне database/sql.
}

// NewUserRepository — конструктор.
//
// ПОЧЕМУ КОНСТРУКТОР, А НЕ &UserRepository{db: db} НАПРЯМУЮ?
//  1. Если в будущем добавим зависимости (кэш, логгер) —
//     меняем только конструктор, не все места создания.
//  2. Можно добавить валидацию: if db == nil { panic }
//  3. Явность: видно, что это "создание объекта с зависимостями".
func NewUserRepository(db *gorm.DB) *UserRepository {
	return &UserRepository{db: db}
}

// Create создаёт нового пользователя.
//
// context.Context — ПЕРВЫЙ параметр. Это Go-конвенция.
// Context нужен для:
// 1. Timeout: если запрос висит дольше 5 секунд — отменяем
// 2. Cancellation: если HTTP-запрос отменён клиентом — отменяем и SQL
// 3. Tracing: передаём trace-id через все слои
func (r *UserRepository) Create(ctx context.Context, user *models.User) error {
	return r.db.WithContext(ctx).Create(user).Error
	// WithContext(ctx) — привязывает контекст к запросу.
	// Если ctx отменён — GORM отменит SQL-запрос.
	//
	// Create(user) — INSERT INTO users (...) VALUES (...).
	// После успешного INSERT GORM заполняет user.ID (LastInsertId).
	//
	// .Error — GORM возвращает *gorm.DB, а не error.
	// Ошибка хранится в поле Error. Это позволяет цепочкам:
	//   db.Where(...).First(...).Error
	// Но это же — источник багов: легко забыть проверить Error.
}

// FindByEmail ищет пользователя по email.
//
// ВОЗВРАЩАЕТ (*models.User, error):
// - (user, nil) — пользователь найден
// - (nil, nil) — пользователь НЕ найден (это НЕ ошибка)
// - (nil, err) — реальная ошибка БД
//
// ПОЧЕМУ (nil, nil) ВМЕСТО ОШИБКИ "not found"?
// "Пользователь не найден" — это нормальная бизнес-ситуация,
// а не исключение. Ошибка должна быть для неожиданных ситуаций
// (БД упала, таймаут, etc.).
//
// Это упрощает код вызывающего:
//
//	user, err := repo.FindByEmail(ctx, email)
//	if err != nil { return err }  // Реальная ошибка
//	if user == nil { ... }        // Просто не найден
func (r *UserRepository) FindByEmail(ctx context.Context, email string) (*models.User, error) {
	var user models.User
	err := r.db.WithContext(ctx).Where("email = ?", email).First(&user).Error
	// Where("email = ?", email) — параметризованный запрос.
	//
	// ⚠️ ПОЧЕМУ НЕ fmt.Sprintf("email = '%s'", email)?
	// SQL INJECTION! Если email = "'; DROP TABLE users; --"
	// то fmt.Sprintf создаст: email = ''; DROP TABLE users; --'
	//
	// Параметры (?) экранируются драйвером БД. Всегда используй их.
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
			// GORM возвращает ErrRecordNotFound, если First() не нашёл запись.
			// Это НЕ ошибка для бизнес-логики → возвращаем (nil, nil).
		}
		return nil, err
		// Любая другая ошибка (connection refused, timeout) — реальная ошибка.
	}
	return &user, nil
}

// FindByID — аналогично FindByEmail, но по ID.
func (r *UserRepository) FindByID(ctx context.Context, id uint) (*models.User, error) {
	var user models.User
	err := r.db.WithContext(ctx).First(&user, id).Error
	// First(&user, id) — shorthand для First(&user, "id = ?", id).
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &user, nil
}
