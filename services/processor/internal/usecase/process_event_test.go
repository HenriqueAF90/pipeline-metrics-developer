package usecase_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/sirupsen/logrus"

	"processor/internal/domain"
	"processor/internal/usecase"
)

// mockPublisher implementa queue.Publisher para testes.
type mockPublisher struct {
	published []domain.ProcessedEvent
	err       error
}

func (m *mockPublisher) Publish(_ context.Context, event domain.ProcessedEvent) error {
	if m.err != nil {
		return m.err
	}
	m.published = append(m.published, event)
	return nil
}

func newLogger() *logrus.Logger {
	l := logrus.New()
	l.SetLevel(logrus.ErrorLevel) // silencioso nos testes
	return l
}

func validBody() string {
	e := domain.RawEvent{
		EventID:     "550e8400-e29b-41d4-a716-446655440000",
		DeveloperID: "dev-1",
		MetricType:  domain.MetricCommits,
		Value:       10,
		Repository:  "org/repo",
		Timestamp:   time.Now().Add(-1 * time.Hour),
	}
	b, _ := json.Marshal(e)
	return string(b)
}

func TestExecute_ValidEvent_PublishesAndReturnsNil(t *testing.T) {
	pub := &mockPublisher{}
	uc := usecase.NewProcessEventUseCase(pub, "proc-1", newLogger())

	err := uc.Execute(context.Background(), 0, validBody())
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(pub.published) != 1 {
		t.Fatalf("expected 1 published event, got %d", len(pub.published))
	}
	if pub.published[0].ProcessorID != "proc-1-worker-0" {
		t.Errorf("unexpected ProcessorID: %s", pub.published[0].ProcessorID)
	}
}

func TestExecute_InvalidJSON_ReturnsErrInvalidEvent(t *testing.T) {
	pub := &mockPublisher{}
	uc := usecase.NewProcessEventUseCase(pub, "proc-1", newLogger())

	err := uc.Execute(context.Background(), 0, "not-json")
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
	if !usecase.IsInvalidEvent(err) {
		t.Errorf("expected ErrInvalidEvent, got: %T", err)
	}
}

func TestExecute_InvalidEvent_ReturnsErrInvalidEvent(t *testing.T) {
	pub := &mockPublisher{}
	uc := usecase.NewProcessEventUseCase(pub, "proc-1", newLogger())

	// event_id vazio → inválido
	body := `{"event_id":"","developer_id":"dev-1","metric_type":"commits","value":5,"timestamp":"2026-04-15T10:00:00Z"}`
	err := uc.Execute(context.Background(), 0, body)
	if !usecase.IsInvalidEvent(err) {
		t.Errorf("expected ErrInvalidEvent, got: %v", err)
	}
	if len(pub.published) != 0 {
		t.Error("should not publish invalid event")
	}
}

func TestExecute_InvalidMetricType_ReturnsErrInvalidEvent(t *testing.T) {
	pub := &mockPublisher{}
	uc := usecase.NewProcessEventUseCase(pub, "proc-1", newLogger())

	body := `{"event_id":"550e8400-e29b-41d4-a716-446655440000","developer_id":"dev-1","metric_type":"invalid_type","value":5,"timestamp":"2026-04-15T10:00:00Z"}`
	err := uc.Execute(context.Background(), 0, body)
	if !usecase.IsInvalidEvent(err) {
		t.Errorf("expected ErrInvalidEvent for invalid metric_type, got: %v", err)
	}
	if len(pub.published) != 0 {
		t.Error("should not publish invalid event")
	}
}

func TestExecute_NegativeValue_ReturnsErrInvalidEvent(t *testing.T) {
	pub := &mockPublisher{}
	uc := usecase.NewProcessEventUseCase(pub, "proc-1", newLogger())

	body := `{"event_id":"550e8400-e29b-41d4-a716-446655440001","developer_id":"dev-1","metric_type":"commits","value":-1,"timestamp":"2026-04-15T10:00:00Z"}`
	err := uc.Execute(context.Background(), 0, body)
	if !usecase.IsInvalidEvent(err) {
		t.Errorf("expected ErrInvalidEvent for negative value, got: %v", err)
	}
	if len(pub.published) != 0 {
		t.Error("should not publish invalid event")
	}
}

func TestExecute_FutureTimestamp_ReturnsErrInvalidEvent(t *testing.T) {
	pub := &mockPublisher{}
	uc := usecase.NewProcessEventUseCase(pub, "proc-1", newLogger())

	body := `{"event_id":"550e8400-e29b-41d4-a716-446655440002","developer_id":"dev-1","metric_type":"commits","value":5,"timestamp":"2099-01-01T00:00:00Z"}`
	err := uc.Execute(context.Background(), 0, body)
	if !usecase.IsInvalidEvent(err) {
		t.Errorf("expected ErrInvalidEvent for future timestamp, got: %v", err)
	}
	if len(pub.published) != 0 {
		t.Error("should not publish invalid event")
	}
}

func TestExecute_PublisherError_ReturnsTransientError(t *testing.T) {
	pub := &mockPublisher{err: errors.New("connection refused")}
	uc := usecase.NewProcessEventUseCase(pub, "proc-1", newLogger())

	err := uc.Execute(context.Background(), 0, validBody())
	if err == nil {
		t.Fatal("expected error from publisher")
	}
	if usecase.IsInvalidEvent(err) {
		t.Error("publisher error should NOT be ErrInvalidEvent")
	}
}

func TestExecute_EnrichesProcessedAt(t *testing.T) {
	pub := &mockPublisher{}
	uc := usecase.NewProcessEventUseCase(pub, "proc-1", newLogger())

	before := time.Now()
	_ = uc.Execute(context.Background(), 0, validBody())
	after := time.Now()

	if len(pub.published) == 0 {
		t.Fatal("no event published")
	}
	pt := pub.published[0].ProcessedAt
	if pt.Before(before) || pt.After(after) {
		t.Errorf("ProcessedAt %v not in expected range [%v, %v]", pt, before, after)
	}
}
