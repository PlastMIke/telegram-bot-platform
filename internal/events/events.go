package events

// ═══════════════════════════════════════════════════════════════
// КОНТРАКТ СОБЫТИЙ
// ═══════════════════════════════════════════════════════════════
// Этот пакет — ЕДИНСТВЕННЫЙ источник правды для DTO событий.
// API Gateway и Bot Worker оба импортируют его, что гарантирует
// идентичность структур. При изменении DTO правим только здесь.
//
// Альтернативы (и почему мы их НЕ используем):
// - Protobuf/Avro: overkill для MVP, но идеально для enterprise
// - Дублирование в каждом сервисе: риск рассинхронизации
// - Shared package через отдельный репо: усложняет CI/CD

// TopicBot* — имена топиков NATS.
// Централизуем, чтобы избежать опечаток типа "bot.creted".
const (
	TopicBotCreated = "bot.created"
	TopicBotDeleted = "bot.deleted"
	TopicBotStopped = "bot.stopped"
)

// BotCreatedEvent публикуется, когда пользователь создаёт бота.
// Worker получает его и запускает Telegram-бота.
type BotCreatedEvent struct {
	BotID   uint   `json:"bot_id"`
	UserID  uint   `json:"user_id"`
	BotName string `json:"bot_name"`
	Token   string `json:"token"`
}

// BotDeletedEvent публикуется при удалении бота.
// Worker должен остановить работающего бота.
type BotDeletedEvent struct {
	BotID  uint `json:"bot_id"`
	UserID uint `json:"user_id"`
}
