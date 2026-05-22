package domain

import "time"

// ProcessedEvent representa o evento recebido da fila processed-events.
type ProcessedEvent struct {
	EventID     string    `json:"event_id" dynamodbav:"event_id"`
	DeveloperID string    `json:"developer_id" dynamodbav:"developer_id"`
	MetricType  string    `json:"metric_type" dynamodbav:"metric_type"`
	Value       int       `json:"value" dynamodbav:"value"`
	Repository  string    `json:"repository" dynamodbav:"repository"`
	Timestamp   time.Time `json:"timestamp" dynamodbav:"timestamp"`
	ProcessedAt time.Time `json:"processed_at" dynamodbav:"processed_at"`
	ProcessorID string    `json:"processor_id" dynamodbav:"processor_id"`
}

// DeveloperSummary representa as métricas agregadas de um desenvolvedor.
type DeveloperSummary struct {
	DeveloperID          string  `json:"developer_id" dynamodbav:"developer_id"`
	TotalCommits         int     `json:"total_commits" dynamodbav:"total_commits"`
	TotalPullRequests    int     `json:"total_pull_requests" dynamodbav:"total_pull_requests"`
	AvgReviewTimeMinutes float64 `json:"avg_review_time_minutes" dynamodbav:"avg_review_time_minutes"`
	EventsProcessed      int     `json:"events_processed" dynamodbav:"events_processed"`
	ReviewTimeSum        int     `json:"review_time_sum" dynamodbav:"review_time_sum"`
	ReviewTimeCount      int     `json:"review_time_count" dynamodbav:"review_time_count"`
	LastActivity         string  `json:"last_activity" dynamodbav:"last_activity"`
}

// Apply atualiza o summary com base em um novo evento. Toda lógica de
// agregação vive aqui, no domínio, tornando-a testável sem dependências externas.
func (s *DeveloperSummary) Apply(event ProcessedEvent) {
	switch event.MetricType {
	case "commits":
		s.TotalCommits += event.Value
	case "pull_requests":
		s.TotalPullRequests += event.Value
	case "review_time_minutes":
		s.ReviewTimeSum += event.Value
		s.ReviewTimeCount++
		s.AvgReviewTimeMinutes = float64(s.ReviewTimeSum) / float64(s.ReviewTimeCount)
	}

	s.EventsProcessed++

	ts := event.Timestamp.UTC().Format(time.RFC3339)
	if ts > s.LastActivity {
		s.LastActivity = ts
	}
}
