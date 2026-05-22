package queue

import (
	"context"
	"sync"

	"github.com/sirupsen/logrus"

	"aggregator/internal/usecase"
)

// Pool gerencia workers que consomem e processam mensagens concorrentemente.
type Pool struct {
	size     int
	consumer *SQSConsumer
	uc       *usecase.AggregateEventUseCase
	log      *logrus.Logger
}

func NewPool(size int, consumer *SQSConsumer, uc *usecase.AggregateEventUseCase, log *logrus.Logger) *Pool {
	return &Pool{size: size, consumer: consumer, uc: uc, log: log}
}

// Run inicia os workers e bloqueia até o contexto ser cancelado.
func (p *Pool) Run(ctx context.Context) {
	msgCh := make(chan Message, p.size*2)
	var wg sync.WaitGroup

	for i := 0; i < p.size; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			p.work(ctx, id, msgCh)
		}(i)
	}

	p.receive(ctx, msgCh)

	p.log.Info("draining aggregator workers...")
	wg.Wait()
	p.log.Info("all aggregator workers drained")
}

func (p *Pool) receive(ctx context.Context, msgCh chan<- Message) {
	defer close(msgCh)
	for {
		select {
		case <-ctx.Done():
			return
		default:
			msgs, err := p.consumer.Receive(ctx)
			if err != nil {
				if ctx.Err() != nil {
					return
				}
				p.log.WithError(err).Error("failed to receive messages")
				continue
			}
			for _, msg := range msgs {
				select {
				case <-ctx.Done():
					return
				case msgCh <- msg:
				}
			}
		}
	}
}

func (p *Pool) work(ctx context.Context, id int, msgCh <-chan Message) {
	for msg := range msgCh {
		logger := p.log.WithFields(map[string]interface{}{
			"worker_id": id,
			"msg_id":    msg.ID,
		})

		if err := p.uc.Execute(ctx, msg.Body); err != nil {
			// Erro transitório: não deletar → SQS fará retry e enviará à DLQ após 3 tentativas
			logger.WithError(err).Error("error aggregating event, message will be retried")
			continue
		}

		if err := p.consumer.Delete(ctx, msg.ReceiptHandle); err != nil {
			logger.WithError(err).Error("failed to delete message after aggregation")
		}
	}
}
