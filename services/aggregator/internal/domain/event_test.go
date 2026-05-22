package domain_test

import (
	"testing"
	"time"

	"aggregator/internal/domain"
)

func makeEvent(metricType string, value int, ts string) domain.ProcessedEvent {
	t, _ := time.Parse(time.RFC3339, ts)
	return domain.ProcessedEvent{
		EventID:     "evt-1",
		DeveloperID: "dev-1",
		MetricType:  metricType,
		Value:       value,
		Timestamp:   t,
	}
}

func TestApply_Commits(t *testing.T) {
	s := domain.DeveloperSummary{DeveloperID: "dev-1"}
	s.Apply(makeEvent("commits", 10, "2026-04-15T10:00:00Z"))

	if s.TotalCommits != 10 {
		t.Errorf("expected TotalCommits=10, got %d", s.TotalCommits)
	}
	if s.EventsProcessed != 1 {
		t.Errorf("expected EventsProcessed=1, got %d", s.EventsProcessed)
	}
}

func TestApply_PullRequests(t *testing.T) {
	s := domain.DeveloperSummary{DeveloperID: "dev-1"}
	s.Apply(makeEvent("pull_requests", 3, "2026-04-15T10:00:00Z"))

	if s.TotalPullRequests != 3 {
		t.Errorf("expected TotalPullRequests=3, got %d", s.TotalPullRequests)
	}
}

func TestApply_ReviewTimeAverage(t *testing.T) {
	s := domain.DeveloperSummary{DeveloperID: "dev-1"}
	s.Apply(makeEvent("review_time_minutes", 60, "2026-04-15T10:00:00Z"))
	s.Apply(makeEvent("review_time_minutes", 120, "2026-04-15T11:00:00Z"))
	s.Apply(makeEvent("review_time_minutes", 90, "2026-04-15T12:00:00Z"))

	expected := float64(60+120+90) / 3
	if s.AvgReviewTimeMinutes != expected {
		t.Errorf("expected AvgReviewTimeMinutes=%.2f, got %.2f", expected, s.AvgReviewTimeMinutes)
	}
	if s.ReviewTimeCount != 3 {
		t.Errorf("expected ReviewTimeCount=3, got %d", s.ReviewTimeCount)
	}
}

func TestApply_LastActivity(t *testing.T) {
	s := domain.DeveloperSummary{DeveloperID: "dev-1"}
	s.Apply(makeEvent("commits", 5, "2026-04-15T10:00:00Z"))
	s.Apply(makeEvent("commits", 3, "2026-04-16T10:00:00Z"))
	s.Apply(makeEvent("commits", 7, "2026-04-14T10:00:00Z"))

	if s.LastActivity != "2026-04-16T10:00:00Z" {
		t.Errorf("expected LastActivity=2026-04-16T10:00:00Z, got %s", s.LastActivity)
	}
}

func TestApply_MixedMetrics(t *testing.T) {
	s := domain.DeveloperSummary{DeveloperID: "dev-1"}
	s.Apply(makeEvent("commits", 10, "2026-04-15T10:00:00Z"))
	s.Apply(makeEvent("pull_requests", 2, "2026-04-15T11:00:00Z"))
	s.Apply(makeEvent("review_time_minutes", 30, "2026-04-15T12:00:00Z"))
	s.Apply(makeEvent("commits", 5, "2026-04-15T13:00:00Z"))

	if s.TotalCommits != 15 {
		t.Errorf("expected TotalCommits=15, got %d", s.TotalCommits)
	}
	if s.TotalPullRequests != 2 {
		t.Errorf("expected TotalPullRequests=2, got %d", s.TotalPullRequests)
	}
	if s.AvgReviewTimeMinutes != 30 {
		t.Errorf("expected AvgReviewTimeMinutes=30, got %.2f", s.AvgReviewTimeMinutes)
	}
	if s.EventsProcessed != 4 {
		t.Errorf("expected EventsProcessed=4, got %d", s.EventsProcessed)
	}
}
