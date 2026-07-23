-- Создаём таблицу пользователей
CREATE TABLE IF NOT EXISTS users (
    id SERIAL PRIMARY KEY,  -- SERIAL означает auto-increment
    email VARCHAR(255) UNIQUE NOT NULL, -- UNIQUE добавляет уникальный индекс
    password VARCHAR(255) NOT NULL, -- Хранить хеш пароля (bcrypt)
    created_at TIMESTAMP DEFAULT NOW(), -- NOW() — текущее время
    updated_at TIMESTAMP DEFAULT NOW()
);

-- Создаём таблицу ботов
CREATE TABLE IF NOT EXISTS bots (
    id SERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    token VARCHAR(255) NOT NULL,    -- Токен Telegram Bot API
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE, -- Внешний ключ на таблицу users
    is_active BOOLEAN DEFAULT TRUE, -- Флаг активности бота
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

-- Создаём индекс для быстрого поиска ботов по user_id
-- Это ускорит запросы типа "SELECT * FROM bots WHERE user_id = ?"
CREATE INDEX idx_bots_user_id ON bots(user_id);

-- Создаём индекс для email (хотя UNIQUE уже создаёт индекс, явно указываем для читаемости)
CREATE INDEX idx_users_email ON users(email);
