package outbox

import (
	"context"
	"log"
	"time"

	"github.com/PlastMIke/telegram-bot-platform/internal/config"
	"github.com/PlastMIke/telegram-bot-platform/internal/repository"
	"github.com/nats-io/nats.go"
)

// Publisher — фоновый процесс, который читает необработанные события из outbox
// и публикует их в NATS. Это отдельный микросервис, который работает параллельно с API.
type Publisher struct {
	outboxRepo *repository.OutboxRepository
	natsConn   *nats.Conn
	cfg        *config.Config
}

func NewPublisher(
	outboxRepo *repository.OutboxRepository,
	natsConn *nats.Conn,
	cfg *config.Config,
) *Publisher {
	return &Publisher{
		outboxRepo: outboxRepo,
		natsConn:   natsConn,
		cfg:        cfg,
	}
}

// Run запускает бесконечный цикл обработки outbox событий.
// Останавливается при отмене контекста (graceful shutdown).
func (p *Publisher) Run(ctx context.Context) {
	// Ticker срабатывает каждую секунду.
	// Почему не чаще? Чтобы не нагружать БД лишними SELECT'ами.
	// Почему не реже? Чтобы события доставлялись быстро.
	// В production можно использовать LISTEN/NOTIFY из PostgreSQL для instant delivery.
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	log.Println("📦 Outbox Publisher started")

	for {
		select {
		case <-ctx.Done():
			log.Println("📦 Outbox Publisher stopped")
			return
		case <-ticker.C:
			p.processBatch(ctx)
		}
	}
}

// processBatch обрабатывает партию необработанных событий.
func (p *Publisher) processBatch(ctx context.Context) {
	events, err := p.outboxRepo.GetUnprocessedEvents(ctx, 100)
	if err != nil {
		log.Printf("❌ Failed to get outbox events: %v", err)
		return
	}

	if len(events) == 0 {
		return // Ничего не делаем, если нет событий
	}

	log.Printf("📦 Processing %d outbox events", len(events))

	for _, event := range events {
		if err := p.publishEvent(ctx, event); err != nil {
			log.Printf("❌ Failed to publish event %d: %v", event.ID, err)
			continue // Переходим к следующему событию
		}

		if err := p.outboxRepo.MarkAsProcessed(ctx, event.ID); err != nil {
			log.Printf("❌ Failed to mark event %d as processed: %v", event.ID, err)
			// Событие опубликовано, но не помечено. Будет опубликовано повторно.
			// Поэтому consumer должен быть идемпотентным!
		}
	}
}

// publishEvent публикует одно событие в NATS.
func (p *Publisher) publishEvent(ctx context.Context, event repository.OutboxEvent) error {
	// В production здесь нужно использовать JetStream для гарантированной доставки:
	// js, _ := p.natsConn.JetStream()
	// js.Publish(event.EventType, []byte(event.Payload))

	_, err := p.natsConn.Publish(event.EventType, []byte(event.Payload))
	return err
}
