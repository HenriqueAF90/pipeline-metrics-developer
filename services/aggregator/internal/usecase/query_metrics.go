package usecase

import (
	"context"
	"fmt"

	"aggregator/internal/domain"
	"aggregator/internal/infra/repository"
)

// QueryMetricsUseCase lida com consultas à API REST.
type QueryMetricsUseCase struct {
	eventRepo   repository.EventRepository
	summaryRepo repository.SummaryRepository
}

func NewQueryMetricsUseCase(eventRepo repository.EventRepository, summaryRepo repository.SummaryRepository) *QueryMetricsUseCase {
	return &QueryMetricsUseCase{eventRepo: eventRepo, summaryRepo: summaryRepo}
}

func (uc *QueryMetricsUseCase) GetEvents(ctx context.Context, developerID string) ([]domain.ProcessedEvent, error) {
	events, err := uc.eventRepo.FindByDeveloper(ctx, developerID)
	if err != nil {
		return nil, fmt.Errorf("get events: %w", err)
	}
	return events, nil
}

func (uc *QueryMetricsUseCase) GetSummary(ctx context.Context, developerID string) (domain.DeveloperSummary, error) {
	summary, err := uc.summaryRepo.Get(ctx, developerID)
	if err != nil {
		return domain.DeveloperSummary{}, fmt.Errorf("get summary: %w", err)
	}
	return summary, nil
}
