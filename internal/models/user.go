package models

import "time"

// User — модель таблицы users.
// GORM автоматически создаст таблицу на основе этой структуры.
type User struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Email     string    `gorm:"uniqueIndex;not null" json:"email"`
	Password  string    `gorm:"not null" json:"-"` // json:"-" — не сериализуем пароль!
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	// HasMany: один пользователь → много ботов.
	// GORM создаст foreign key user_id в таблице bots.
	Bots []Bot `gorm:"foreignKey:UserID" json:"bots,omitempty"`
	// ⚠️ ВНИМАНИЕ: это создаёт N+1 проблему при загрузке.
	// Если загрузить 100 пользователей, GORM сделает 101 запрос
	// (1 на пользователей + 100 на ботов каждого).
	// Решение: Preload("Bots") или явный JOIN.
}

func (User) TableName() string {
	return "users"
}
