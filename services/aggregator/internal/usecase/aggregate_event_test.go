package usecase_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/sirupsen/logrus"

	"aggregator/internal/domain"
	"aggregator/internal/usecase"
)

// --- mocks ---

type mockEventRepo struct {
	existing map[string]bool
	saved    []domain.ProcessedEvent
}

func (m *mockEventRepo) Exists(_ context.Context, id string) (bool, error) {
	return m.existing[id], nil
}

func (m *mockEventRepo) Save(_ context.Context, e domain.ProcessedEvent) error {
	m.saved = append(m.saved, e)
	if m.existing == nil {
		m.existing = map[string]bool{}
	}
	m.existing[e.EventID] = true
	return nil
}

func (m *mockEventRepo) FindByDeveloper(_ context.Context, _ string) ([]domain.ProcessedEvent, error) {
	return m.saved, nil
}

type mockSummaryRepo struct {
	summaries map[string]domain.DeveloperSummary
}

func (m *mockSummaryRepo) Get(_ context.Context, devID string) (domain.DeveloperSummary, error) {
	if s, ok := m.summaries[devID]; ok {
		return s, nil
	}
	return domain.DeveloperSummary{DeveloperID: devID}, nil
}

func (m *mockSummaryRepo) Save(_ context.Context, s domain.DeveloperSummary) error {
	if m.summaries == nil {
		m.summaries = map[string]domain.DeveloperSummary{}
	}
	m.summaries[s.DeveloperID] = s
	return nil
}

type mockEventRepoWithSaveError struct {
	mockEventRepo
}

func (m *mockEventRepoWithSaveError) Save(_ context.Context, event domain.ProcessedEvent) error {
	return errors.New("event save failed")
}

type mockSummaryRepoWithGetError struct {
	mockSummaryRepo
}

func (m *mockSummaryRepoWithGetError) Get(_ context.Context, devID string) (domain.DeveloperSummary, error) {
	return domain.DeveloperSummary{DeveloperID: devID}, errors.New("summary get failed")
}

type mockSummaryRepoWithSaveError struct {
	mockSummaryRepo
}

func (m *mockSummaryRepoWithSaveError) Save(_ context.Context, summary domain.DeveloperSummary) error {
	return errors.New("summary save failed")
}

// --- helpers ---

func newLogger() *logrus.Logger {
	l := logrus.New()
	l.SetLevel(logrus.ErrorLevel)
	return l
}

func makeBody(eventID, devID, metricType string, value int) string {
	e := domain.ProcessedEvent{
		EventID:     eventID,
		DeveloperID: devID,
		MetricType:  metricType,
		Value:       value,
		Timestamp:   time.Now().Add(-1 * time.Hour),
		ProcessedAt: time.Now(),
	}
	b, _ := json.Marshal(e)
	return string(b)
}

// --- testes ---

func TestExecute_ValidEvent_SavesAndAggregates(t *testing.T) {
	eventRepo := &mockEventRepo{existing: map[string]bool{}}
	summaryRepo := &mockSummaryRepo{}
	uc := usecase.NewAggregateEventUseCase(eventRepo, summaryRepo, newLogger())

	err := uc.Execute(context.Background(), makeBody("evt-1", "dev-1", "commits", 10))
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(eventRepo.saved) != 1 {
		t.Fatalf("expected 1 saved event, got %d", len(eventRepo.saved))
	}
	s := summaryRepo.summaries["dev-1"]
	if s.TotalCommits != 10 {
		t.Errorf("expected TotalCommits=10, got %d", s.TotalCommits)
	}
}

func TestExecute_DuplicateEvent_Skipped(t *testing.T) {
	eventRepo := &mockEventRepo{existing: map[string]bool{"evt-dup": true}}
	summaryRepo := &mockSummaryRepo{}
	uc := usecase.NewAggregateEventUseCase(eventRepo, summaryRepo, newLogger())

	err := uc.Execute(context.Background(), makeBody("evt-dup", "dev-1", "commits", 99))
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	// Não deve salvar nem atualizar summary
	if len(eventRepo.saved) != 0 {
		t.Error("duplicate event should not be saved")
	}
	if _, ok := summaryRepo.summaries["dev-1"]; ok {
		t.Error("summary should not be updated for duplicate")
	}
}

func TestExecute_InvalidJSON_ReturnsError(t *testing.T) {
	uc := usecase.NewAggregateEventUseCase(&mockEventRepo{}, &mockSummaryRepo{}, newLogger())
	err := uc.Execute(context.Background(), "not-json")
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestExecute_AccumulatesMultipleEvents(t *testing.T) {
	eventRepo := &mockEventRepo{existing: map[string]bool{}}
	summaryRepo := &mockSummaryRepo{}
	uc := usecase.NewAggregateEventUseCase(eventRepo, summaryRepo, newLogger())

	_ = uc.Execute(context.Background(), makeBody("e1", "dev-1", "commits", 5))
	_ = uc.Execute(context.Background(), makeBody("e2", "dev-1", "commits", 3))
	_ = uc.Execute(context.Background(), makeBody("e3", "dev-1", "pull_requests", 2))

	s := summaryRepo.summaries["dev-1"]
	if s.TotalCommits != 8 {
		t.Errorf("expected TotalCommits=8, got %d", s.TotalCommits)
	}
	if s.TotalPullRequests != 2 {
		t.Errorf("expected TotalPullRequests=2, got %d", s.TotalPullRequests)
	}
	if s.EventsProcessed != 3 {
		t.Errorf("expected EventsProcessed=3, got %d", s.EventsProcessed)
	}
}

func TestExecute_SummaryRepoGetError_ContinuesWithFreshSummary(t *testing.T) {
	eventRepo := &mockEventRepo{existing: map[string]bool{}}
	summaryRepo := &mockSummaryRepoWithGetError{}
	uc := usecase.NewAggregateEventUseCase(eventRepo, summaryRepo, newLogger())

	err := uc.Execute(context.Background(), makeBody("e4", "dev-1", "commits", 7))
	if err != nil {
		t.Fatalf("expected no error even when summary get fails, got: %v", err)
	}
	if len(eventRepo.saved) != 1 {
		t.Fatalf("expected event saved, got %d", len(eventRepo.saved))
	}
}

func TestExecute_SaveEventError_ReturnsError(t *testing.T) {
	eventRepo := &mockEventRepoWithSaveError{}
	summaryRepo := &mockSummaryRepo{}
	uc := usecase.NewAggregateEventUseCase(eventRepo, summaryRepo, newLogger())

	err := uc.Execute(context.Background(), makeBody("e5", "dev-1", "commits", 4))
	if err == nil {
		t.Fatal("expected error when event save fails")
	}
}

func TestExecute_SaveSummaryError_ReturnsError(t *testing.T) {
	eventRepo := &mockEventRepo{existing: map[string]bool{}}
	summaryRepo := &mockSummaryRepoWithSaveError{}
	uc := usecase.NewAggregateEventUseCase(eventRepo, summaryRepo, newLogger())

	err := uc.Execute(context.Background(), makeBody("e6", "dev-1", "commits", 4))
	if err == nil {
		t.Fatal("expected error when summary save fails")
	}
}
