package usecase

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/sirupsen/logrus"

	"aggregator/internal/domain"
	"aggregator/internal/infra/repository"
)

// AggregateEventUseCase orquestra: idempotência → persistência do evento → atualização do summary.
type AggregateEventUseCase struct {
	eventRepo   repository.EventRepository
	summaryRepo repository.SummaryRepository
	log         *logrus.Logger
}

func NewAggregateEventUseCase(
	eventRepo repository.EventRepository,
	summaryRepo repository.SummaryRepository,
	log *logrus.Logger,
) *AggregateEventUseCase {
	return &AggregateEventUseCase{
		eventRepo:   eventRepo,
		summaryRepo: summaryRepo,
		log:         log,
	}
}

// Execute processa uma mensagem: verifica duplicidade, persiste evento e atualiza o summary.
func (uc *AggregateEventUseCase) Execute(ctx context.Context, msgBody string) error {
	var event domain.ProcessedEvent
	if err := json.Unmarshal([]byte(msgBody), &event); err != nil {
		return fmt.Errorf("unmarshal event: %w", err)
	}

	logger := uc.log.WithField("event_id", event.EventID)

	// Garantia de idempotência: ignorar eventos já processados
	exists, err := uc.eventRepo.Exists(ctx, event.EventID)
	if err != nil {
		return fmt.Errorf("check idempotency: %w", err)
	}
	if exists {
		logger.Warn("duplicate event_id detected, skipping")
		return nil
	}

	if err := uc.eventRepo.Save(ctx, event); err != nil {
		return fmt.Errorf("save event: %w", err)
	}

	summary, err := uc.summaryRepo.Get(ctx, event.DeveloperID)
	if err != nil {
		// Erro não-fatal: log e continua com summary vazio
		logger.WithError(err).Warn("failed to load summary, starting fresh")
		summary = domain.DeveloperSummary{DeveloperID: event.DeveloperID}
	}

	summary.Apply(event)

	if err := uc.summaryRepo.Save(ctx, summary); err != nil {
		return fmt.Errorf("save summary: %w", err)
	}

	logger.WithField("developer_id", event.DeveloperID).Info("event aggregated")
	return nil
}
