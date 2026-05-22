package domain

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

// MetricType representa os tipos válidos de métricas.
type MetricType string

const (
	MetricCommits           MetricType = "commits"
	MetricPullRequests      MetricType = "pull_requests"
	MetricReviewTimeMinutes MetricType = "review_time_minutes"

	MaxReviewTimeMinutes = 1440
)

// RawEvent representa um evento bruto recebido da fila raw-events.
type RawEvent struct {
	EventID     string     `json:"event_id"`
	DeveloperID string     `json:"developer_id"`
	MetricType  MetricType `json:"metric_type"`
	Value       int        `json:"value"`
	Repository  string     `json:"repository"`
	Timestamp   time.Time  `json:"timestamp"`
}

// ProcessedEvent representa um evento enriquecido pronto para a fila processed-events.
type ProcessedEvent struct {
	EventID     string     `json:"event_id"`
	DeveloperID string     `json:"developer_id"`
	MetricType  MetricType `json:"metric_type"`
	Value       int        `json:"value"`
	Repository  string     `json:"repository"`
	Timestamp   time.Time  `json:"timestamp"`
	ProcessedAt time.Time  `json:"processed_at"`
	ProcessorID string     `json:"processor_id"`
}

// Validate aplica todas as regras de negócio do domínio ao evento bruto.
// Retorna um erro descritivo para cada regra violada.
func (e *RawEvent) Validate() error {
	if e.EventID == "" {
		return errors.New("event_id is required")
	}
	if _, err := uuid.Parse(e.EventID); err != nil {
		return errors.New("event_id must be a valid UUID v4")
	}
	if e.DeveloperID == "" {
		return errors.New("developer_id is required")
	}
	switch e.MetricType {
	case MetricCommits, MetricPullRequests, MetricReviewTimeMinutes:
		// válido
	default:
		return errors.New("metric_type must be one of: commits, pull_requests, review_time_minutes")
	}
	if e.Value < 0 {
		return errors.New("value must be >= 0")
	}
	if e.MetricType == MetricReviewTimeMinutes && e.Value > MaxReviewTimeMinutes {
		return errors.New("review_time_minutes value must be <= 1440 (24h)")
	}
	if e.Timestamp.IsZero() {
		return errors.New("timestamp is required")
	}
	if e.Timestamp.After(time.Now()) {
		return errors.New("timestamp cannot be in the future")
	}
	return nil
}
