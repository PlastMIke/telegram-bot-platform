-- Outbox Pattern: техническая таблица для гарантированной доставки событий
CREATE TABLE IF NOT EXISTS outbox_events (
    id SERIAL PRIMARY KEY,
    event_type VARCHAR(100) NOT NULL,
    payload JSONB NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    processed BOOLEAN NOT NULL DEFAULT FALSE
);

-- Индексы для быстрого поиска необработанных событий
CREATE INDEX idx_outbox_events_processed_created ON outbox_events(processed, created_at);
CREATE INDEX idx_outbox_events_event_type ON outbox_events(event_type);

-- Добавляем поле status в таблицу bots
ALTER TABLE bots ADD COLUMN IF NOT EXISTS status VARCHAR(50) DEFAULT 'pending';