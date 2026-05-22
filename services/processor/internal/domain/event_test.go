package domain_test

import (
	"testing"
	"time"

	"processor/internal/domain"
)

func validEvent() domain.RawEvent {
	return domain.RawEvent{
		EventID:     "550e8400-e29b-41d4-a716-446655440000",
		DeveloperID: "dev-1",
		MetricType:  domain.MetricCommits,
		Value:       10,
		Repository:  "org/repo",
		Timestamp:   time.Now().Add(-1 * time.Hour),
	}
}

func TestValidate_ValidCommitsEvent(t *testing.T) {
	e := validEvent()
	if err := e.Validate(); err != nil {
		t.Errorf("expected valid event, got: %v", err)
	}
}

func TestValidate_ValidPullRequestsEvent(t *testing.T) {
	e := validEvent()
	e.EventID = "550e8400-e29b-41d4-a716-446655440001"
	e.MetricType = domain.MetricPullRequests
	e.Value = 5
	if err := e.Validate(); err != nil {
		t.Errorf("expected valid event, got: %v", err)
	}
}

func TestValidate_ValidReviewTimeEvent(t *testing.T) {
	e := validEvent()
	e.EventID = "550e8400-e29b-41d4-a716-446655440002"
	e.MetricType = domain.MetricReviewTimeMinutes
	e.Value = 60
	if err := e.Validate(); err != nil {
		t.Errorf("expected valid event, got: %v", err)
	}
}

func TestValidate_EmptyEventID(t *testing.T) {
	e := validEvent()
	e.EventID = ""
	if err := e.Validate(); err == nil {
		t.Error("expected error for empty event_id")
	}
}

func TestValidate_NonUUIDEventID(t *testing.T) {
	e := validEvent()
	e.EventID = "not-a-uuid"
	if err := e.Validate(); err == nil {
		t.Error("expected error for non-UUID event_id")
	}
}

func TestValidate_EmptyDeveloperID(t *testing.T) {
	e := validEvent()
	e.DeveloperID = ""
	if err := e.Validate(); err == nil {
		t.Error("expected error for empty developer_id")
	}
}

func TestValidate_InvalidMetricType(t *testing.T) {
	e := validEvent()
	e.MetricType = "invalid_type"
	if err := e.Validate(); err == nil {
		t.Error("expected error for invalid metric_type")
	}
}

func TestValidate_NegativeValue(t *testing.T) {
	e := validEvent()
	e.Value = -1
	if err := e.Validate(); err == nil {
		t.Error("expected error for negative value")
	}
}

func TestValidate_ZeroValueIsAllowed(t *testing.T) {
	e := validEvent()
	e.Value = 0
	if err := e.Validate(); err != nil {
		t.Errorf("expected zero value to be valid, got: %v", err)
	}
}

func TestValidate_ReviewTimeExceeds1440(t *testing.T) {
	e := validEvent()
	e.MetricType = domain.MetricReviewTimeMinutes
	e.Value = 1441
	if err := e.Validate(); err == nil {
		t.Error("expected error for review_time > 1440")
	}
}

func TestValidate_ReviewTimeExactly1440(t *testing.T) {
	e := validEvent()
	e.MetricType = domain.MetricReviewTimeMinutes
	e.Value = 1440
	if err := e.Validate(); err != nil {
		t.Errorf("expected 1440 to be valid, got: %v", err)
	}
}

func TestValidate_FutureTimestamp(t *testing.T) {
	e := validEvent()
	e.Timestamp = time.Now().Add(24 * time.Hour)
	if err := e.Validate(); err == nil {
		t.Error("expected error for future timestamp")
	}
}

func TestValidate_ZeroTimestamp(t *testing.T) {
	e := validEvent()
	e.Timestamp = time.Time{}
	if err := e.Validate(); err == nil {
		t.Error("expected error for zero timestamp")
	}
}
