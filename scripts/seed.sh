#!/bin/bash

ENDPOINT="http://localhost:4566"
QUEUE_URL="$ENDPOINT/000000000000/raw-events"

echo "=== Sending seed messages to raw-events queue ==="
echo ""

send_message() {
    local body="$1"
    local desc="$2"

    aws --endpoint-url=$ENDPOINT sqs send-message \
        --queue-url $QUEUE_URL \
        --message-body "$body" \
        --region us-east-1 > /dev/null 2>&1

    echo "  ✓ $desc"
}

# --- Valid messages (various developers and metric types) ---
echo "📦 Sending valid messages..."

send_message '{
    "event_id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
    "developer_id": "dev-123",
    "metric_type": "commits",
    "value": 15,
    "repository": "org/backend-api",
    "timestamp": "2026-04-15T10:30:00Z"
}' "dev-123: 15 commits"

send_message '{
    "event_id": "b1c2d3e4-f5g6-8901-bcde-f12345678901",
    "developer_id": "dev-123",
    "metric_type": "pull_requests",
    "value": 3,
    "repository": "org/backend-api",
    "timestamp": "2026-04-15T11:00:00Z"
}' "dev-123: 3 pull_requests"

send_message() {

send_message '{
  "event_id": "c304e5f6-a7b8-9012-cdef-123456789012",
  "developer_id": "dev-123",
  "metric_type": "review_time_minutes",
  "value": 45,
  "repository": "org/backend-api",
  "timestamp": "2026-04-15T12:00:00Z"
}' "dev-123: 45 review_time_minutes"

send_message '{
  "event_id": "d4c5f6a7-b8c9-0123-defa-234567890123",
  "developer_id": "dev-456",
  "metric_type": "commits",
  "value": 8,
  "repository": "org/frontend-app",
  "timestamp": "2026-04-15T09:00:00Z"
}' "dev-456: 8 commits"

send_message '{
  "event_id": "e5f6a7b8-c9d0-1234-efab-345678901234",
  "developer_id": "dev-456",
  "metric_type": "pull_requests",
  "value": 2,
  "repository": "org/frontend-app",
  "timestamp": "2026-04-15T10:00:00Z"
}' "dev-456: 2 pull_requests"

send_message '{
  "event_id": "f6a7b8c9-d0e1-2345-fabc-456789012345",
  "developer_id": "dev-456",
  "metric_type": "review_time_minutes",
  "value": 120,
  "repository": "org/frontend-app",
  "timestamp": "2026-04-15T14:00:00Z"
}' "dev-456: 120 review_time_minutes"

}