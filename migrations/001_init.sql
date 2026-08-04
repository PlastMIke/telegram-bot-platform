-- Создаём таблицу пользователей
CREATE TABLE IF NOT EXISTS users (
    id SERIAL PRIMARY KEY,
    -- SERIAL = auto-increment integer.
    -- При INSERT без указания id PostgreSQL сам присвоит следующий.
    -- 
    -- АЛЬТЕРНАТИВА: UUID (gen_random_uuid()).
    -- UUID лучше для распределённых систем (не нужна координация),
    -- но занимает 16 байт вместо 4 и хуже для индексов.
    -- Для MVP SERIAL — ок.
    email VARCHAR(255) UNIQUE NOT NULL,
    -- VARCHAR(255) — стандартная длина для email (RFC 5321).
    -- UNIQUE — создаёт уникальный индекс автоматически.
    -- NOT NULL — email не может быть пустым.
    password VARCHAR(255) NOT NULL,
    -- VARCHAR(255) — bcrypt hash всегда 60 символов.
    -- 255 — с запасом на будущее (argon2 hash длиннее).
    -- 
    -- ⚠️ НИКОГДА не храним пароль в открытом виде!
    -- Только bcrypt/argon2 hash.
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
    -- DEFAULT NOW() — PostgreSQL сам подставит текущее время.
    -- 
    -- ⚠️ TIMESTAMP vs TIMESTAMPTZ:
    -- TIMESTAMP — без таймзоны (хранит "как есть")
    -- TIMESTAMPTZ — с таймзоной (хранит UTC, конвертирует при выводе)
    -- В production ВСЕГДА используем TIMESTAMPTZ.
);

-- Создаём таблицу ботов
CREATE TABLE IF NOT EXISTS bots (
    id SERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    token VARCHAR(255) NOT NULL,
    -- ⚠️ В production: UNIQUE на token.
    -- ALTER TABLE bots ADD CONSTRAINT uq_token UNIQUE (token);
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    -- REFERENCES users(id) — FOREIGN KEY(Внешний ключ).
    -- Нельзя создать бота с несуществующим user_id.
    -- 
    -- ON DELETE CASCADE — при удалении пользователя
    -- автоматически удаляются все его боты.
    -- 
    -- АЛЬТЕРНАТИВЫ:
    -- ON DELETE RESTRICT — запретить удаление, если есть боты
    -- ON DELETE SET NULL — обнулить user_id (нужен nullable user_id)
    is_active BOOLEAN DEFAULT TRUE, -- Флаг активности бота
    status VARCHAR(50) DEFAULT 'pending',
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

-- Создаём индекс для быстрого поиска ботов по user_id
-- Это ускорит запросы типа "SELECT * FROM bots WHERE user_id = ?"
CREATE INDEX idx_bots_user_id ON bots(user_id);
-- Индекс для запроса "все боты пользователя".
-- Без индекса: O(n) full table scan.
-- С индексом: O(log n) B-tree lookup.
-- 
-- ⚠️ FOREIGN KEY НЕ СОЗДАЁТ ИНДЕКС АВТОМАТИЧЕСКИ в PostgreSQL!
-- (В MySQL — создаёт.) Поэтому создаём явно.

CREATE INDEX idx_users_email ON users(email);
-- Явно указываем для читаемости.
-- UNIQUE уже создал индекс, но это неочевидно при чтении кода.
