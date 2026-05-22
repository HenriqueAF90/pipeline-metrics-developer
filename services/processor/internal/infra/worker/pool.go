package worker

import (
	"context"
	"sync"

	"github.com/sirupsen/logrus"

	"processor/internal/infra/queue"
	"processor/internal/usecase"
)

// Pool gerencia um conjunto de goroutines que processam mensagens concorrentemente.
type Pool struct {
	size    int
	consumer queue.Consumer
	uc      *usecase.ProcessEventUseCase
	log     *logrus.Logger
}

func NewPool(size int, consumer queue.Consumer, uc *usecase.ProcessEventUseCase, log *logrus.Logger) *Pool {
	return &Pool{size: size, consumer: consumer, uc: uc, log: log}
}

// Run inicia o pool de workers e bloqueia até o contexto ser cancelado.
// Ao cancelar, drena todas as goroutines antes de retornar.
func (p *Pool) Run(ctx context.Context) {
	msgCh := make(chan queue.Message, p.size*2)
	var wg sync.WaitGroup

	// Inicia workers
	for i := 0; i < p.size; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			p.work(ctx, workerID, msgCh)
		}(i)
	}

	// Loop de recebimento de mensagens
	p.receive(ctx, msgCh)

	p.log.Info("draining workers...")
	wg.Wait()
	p.log.Info("all workers drained")
}

// receive consome mensagens da fila e as envia ao canal até o contexto ser cancelado.
func (p *Pool) receive(ctx context.Context, msgCh chan<- queue.Message) {
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
				p.log.WithError(err).Error("failed to receive messages from queue")
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

// work processa mensagens do canal até ele ser fechado.
func (p *Pool) work(ctx context.Context, workerID int, msgCh <-chan queue.Message) {
	for msg := range msgCh {
		logger := p.log.WithFields(map[string]interface{}{
			"worker_id": workerID,
			"msg_id":    msg.ID,
		})

		err := p.uc.Execute(ctx, workerID, msg.Body)
		if err == nil {
			// Sucesso: deletar da fila
			if delErr := p.consumer.Delete(ctx, msg.ReceiptHandle); delErr != nil {
				logger.WithError(delErr).Error("failed to delete processed message")
			}
			continue
		}

		if usecase.IsInvalidEvent(err) {
			// Evento inválido: deletar imediatamente (sem consumir tentativas do SQS)
			// O evento NÃO vai para a DLQ pois é um erro de negócio, não de infraestrutura.
			logger.WithError(err).Warn("invalid event discarded")
			if delErr := p.consumer.Delete(ctx, msg.ReceiptHandle); delErr != nil {
				logger.WithError(delErr).Error("failed to delete invalid message")
			}
			continue
		}

		// Erro transitório (ex: SQS indisponível): NÃO deletar.
		// O SQS vai recolocar a mensagem na fila e, após 3 tentativas, enviá-la à DLQ.
		logger.WithError(err).Error("transient error processing message, message will be retried by SQS")
	}
}
