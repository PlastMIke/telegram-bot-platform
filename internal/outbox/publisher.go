package outbox

import (
	"context"
	"log"
	"time"

	"github.com/PlastMIke/telegram-bot-platform/internal/config"
	"github.com/PlastMIke/telegram-bot-platform/internal/models"
	"github.com/PlastMIke/telegram-bot-platform/internal/repository"
	"github.com/nats-io/nats.go"
)

// Publisher — фоновый процесс, который переносит события
// из outbox (PostgreSQL) в NATS.
//
// АРХИТЕКТУРНАЯ РОЛЬ:
// API Gateway  →  [outbox_events в БД]  →  Publisher  →  NATS  →  Worker
//
// Publisher — это "мост" между транзакционным миром (SQL)
// и асинхронным миром (NATS).
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
// ПАТТЕРН: "Polling + Batch Processing"
// Каждую секунду проверяем, есть ли новые события.
// Если есть — обрабатываем партию.
//
// АЛЬТЕРНАТИВЫ:
//  1. PostgreSQL LISTEN/NOTIFY — мгновенное уведомление,
//     но сложнее в настройке и не работает с репликацией.
//  2. Debezium (CDC) — читает WAL PostgreSQL, нулевая задержка,
//     но требует Kafka Connect и сложной инфраструктуры.
//  3. Cron + SELECT — то, что мы делаем. Просто, надёжно,
//     задержка до 1 секунды (приемлемо для MVP).
func (p *Publisher) Run(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Second)
	// Ticker — таймер, который "тикает" каждые 1 секунду.
	//
	// ПОЧЕМУ 1 СЕКУНДА?
	// - Меньше (100мс) → лишняя нагрузка на БД (10 SELECT/сек)
	// - Больше (10с) → события доставляются медленно
	// - 1с → баланс между нагрузкой и задержкой
	//
	// В production можно сделать адаптивный интервал:
	// если есть события → обрабатываем сразу
	// если нет → увеличиваем интервал (backoff)
	defer ticker.Stop()
	// defer — гарантируем, что ticker будет остановлен.
	// Иначе будет утечка ресурсов (ticker держит goroutine).

	log.Println("📦 Outbox Publisher started")

	for {
		select {
		case <-ctx.Done():
			// Контекст отменён (graceful shutdown).
			// Выходим из цикла.
			log.Println("📦 Outbox Publisher stopped")
			return
		case <-ticker.C:
			// Ticker "тикнул" — обрабатываем партию.
			p.processBatch(ctx)
		}
		// select ждёт ПЕРВОЕ сработавшее событие.
		// Если одновременно пришёл сигнал shutdown и тикнул ticker —
		// обработается тот, который select выберет (недетерминировано).
		// Но это ок: в худшем случае обработаем одну лишнюю партию.
	}
}

// processBatch обрабатывает партию необработанных событий.
func (p *Publisher) processBatch(ctx context.Context) {
	// ─── ШАГ 1: Получаем необработанные события ───
	events, err := p.outboxRepo.GetUnprocessedEvents(ctx, 100)
	if err != nil {
		log.Printf("❌ Failed to get outbox events: %v", err)
		return
		// Не паникуем. Логируем и ждём следующего тика.
		// Возможно, БД временно недоступна.
	}

	if len(events) == 0 {
		return
		// Нечего обрабатывать. Выходим без лишнего логирования.
		// (Иначе лог будет заспамлен "Processing 0 events" каждую секунду)
	}

	log.Printf("📦 Processing %d outbox events", len(events))

	// ─── ШАГ 2: Обрабатываем каждое событие ───
	for _, event := range events {
		// Публикуем в NATS
		if err := p.publishEvent(ctx, event); err != nil {
			log.Printf("❌ Failed to publish event %d: %v", event.ID, err)
			continue
			// continue — переходим к следующему событию.
			// Одно проваленное событие не должно блокировать остальные.
			//
			// ⚠️ В production нужен RETRY с exponential backoff.
			// Если NATS упал — все 100 событий провалятся.
			// Лучше: попытаться 3 раза с паузой 1s, 2s, 4s.
		}

		// Помечаем как обработанное
		if err := p.outboxRepo.MarkAsProcessed(ctx, event.ID); err != nil {
			log.Printf("❌ Failed to mark event %d as processed: %v", event.ID, err)
			// Событие опубликовано, но не помечено.
			// При следующем тике будет опубликовано ПОВТОРНО.
			//
			// Это "at-least-once delivery".
			// Consumer (Worker) должен быть идемпотентным!
		}
	}
}

// publishEvent публикует одно событие в NATS.
func (p *Publisher) publishEvent(ctx context.Context, event models.OutboxEvent) error {
	// Публикуем в NATS.
	//
	// event.EventType → топик (например, "bot.created")
	// event.Payload → тело сообщения (JSON)
	//
	// ⚠️ В PRODUCTION используем JetStream:
	//   js, _ := p.natsConn.JetStream()
	//   js.Publish(event.EventType, []byte(event.Payload))
	//
	// JetStream гарантирует, что сообщение не потеряется,
	// даже если все consumer'ы offline.

	err := p.natsConn.Publish(event.EventType, []byte(event.Payload))
	return err
}
