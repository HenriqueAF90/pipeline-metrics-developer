package usecase

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/sirupsen/logrus"

	"processor/internal/domain"
	"processor/internal/infra/queue"
)

// ProcessEventUseCase orquestra o pipeline de processamento de um único evento.
type ProcessEventUseCase struct {
	publisher   queue.Publisher
	processorID string
	log         *logrus.Logger
}

func NewProcessEventUseCase(publisher queue.Publisher, processorID string, log *logrus.Logger) *ProcessEventUseCase {
	return &ProcessEventUseCase{
		publisher:   publisher,
		processorID: processorID,
		log:         log,
	}
}

// Execute recebe o corpo de uma mensagem SQS e executa validação → enriquecimento → publicação.
// Retorna ErrInvalidEvent se o evento não passar nas validações (mensagem vai para DLQ após 3 tentativas).
// Outros erros são transitórios e devem ser tratados com retry pelo caller.
func (uc *ProcessEventUseCase) Execute(ctx context.Context, workerID int, msgBody string) error {
	var raw domain.RawEvent
	if err := json.Unmarshal([]byte(msgBody), &raw); err != nil {
		return &ErrInvalidEvent{Reason: fmt.Sprintf("unmarshal failed: %v", err)}
	}

	logger := uc.log.WithFields(logrus.Fields{
		"event_id":  raw.EventID,
		"worker_id": workerID,
	})

	if err := raw.Validate(); err != nil {
		logger.WithError(err).Warn("event failed validation")
		return &ErrInvalidEvent{Reason: err.Error()}
	}

	processed := domain.ProcessedEvent{
		EventID:     raw.EventID,
		DeveloperID: raw.DeveloperID,
		MetricType:  raw.MetricType,
		Value:       raw.Value,
		Repository:  raw.Repository,
		Timestamp:   raw.Timestamp,
		ProcessedAt: time.Now().UTC(),
		ProcessorID: fmt.Sprintf("%s-worker-%d", uc.processorID, workerID),
	}

	if err := uc.publisher.Publish(ctx, processed); err != nil {
		return fmt.Errorf("publish processed event: %w", err)
	}

	logger.Info("event processed and published")
	return nil
}

// ErrInvalidEvent indica que o evento violou regras de negócio.
// O caller deve deletar a mensagem sem novo processamento (deixando o SQS
// contabilizar as tentativas antes de enviar à DLQ).
type ErrInvalidEvent struct {
	Reason string
}

func (e *ErrInvalidEvent) Error() string {
	return fmt.Sprintf("invalid event: %s", e.Reason)
}

// IsInvalidEvent retorna true se o erro for do tipo ErrInvalidEvent.
func IsInvalidEvent(err error) bool {
	_, ok := err.(*ErrInvalidEvent)
	return ok
}
