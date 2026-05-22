#!/bin/bash

set -e  # Exit on error

LOCALSTACK_SERVICE="localstack"
QUEUE_URL="http://localhost:4566/000000000000/raw-events"

get_localstack_container() {
  local container_id
  container_id=$(docker compose ps -q "$LOCALSTACK_SERVICE" 2>/dev/null || true)
  if [ -n "$container_id" ]; then
    echo "$container_id"
    return 0
  fi

  docker ps --filter "name=$LOCALSTACK_SERVICE" --format '{{.ID}}' | head -n 1
}

LOCALSTACK_CONTAINER=$(get_localstack_container)
if [ -z "$LOCALSTACK_CONTAINER" ]; then
  echo "❌ Error: LocalStack container '$LOCALSTACK_SERVICE' is not running"
  echo "   Run: docker compose up -d"
  exit 1
fi

send_message() {
  local body="$1"
  local label="$2"

  if docker exec -i "$LOCALSTACK_CONTAINER" awslocal sqs send-message \
    --queue-url "$QUEUE_URL" \
    --message-body "$body" > /dev/null 2>&1; then
    echo "✓ Sent: $label"
  else
    echo "✗ Failed to send: $label"
    echo "  Body: $body"
    return 1
  fi
}

echo "=== Sending valid messages ==="

DEVELOPERS=("dev-1" "dev-2" "dev-3" "dev-4")
METRIC_TYPES=("commits" "pull_requests" "review_time_minutes")
REPOS=("org/api" "org/frontend" "org/infra" "org/docs")

DUPLICATE_UUID="aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"

for i in {1..20}; do
  UUID=$(cat /proc/sys/kernel/random/uuid)
  DEV=${DEVELOPERS[$((RANDOM % ${#DEVELOPERS[@]}))]}
  METRIC=${METRIC_TYPES[$((RANDOM % ${#METRIC_TYPES[@]}))]}
  REPO=${REPOS[$((RANDOM % ${#REPOS[@]}))]}

  if [ "$METRIC" = "review_time_minutes" ]; then
    VALUE=$((RANDOM % 120 + 1))
  else
    VALUE=$((RANDOM % 50 + 1))
  fi

  if [ "$i" -eq 1 ]; then
    UUID="$DUPLICATE_UUID"
  fi

  BODY="{\"event_id\":\"$UUID\",\"developer_id\":\"$DEV\",\"metric_type\":\"$METRIC\",\"value\":$VALUE,\"repository\":\"$REPO\",\"timestamp\":\"2026-04-15T10:30:00Z\"}"
  send_message "$BODY" "valid #$i ($DEV, $METRIC, value=$VALUE)"
done

echo ""
echo "=== Sending invalid messages ==="

send_message '{"event_id":"","developer_id":"dev-1","metric_type":"commits","value":5,"repository":"org/api","timestamp":"2026-04-15T10:30:00Z"}' \
  "invalid #1 (empty event_id)"

send_message '{"event_id":"not-a-uuid","developer_id":"dev-1","metric_type":"commits","value":5,"repository":"org/api","timestamp":"2026-04-15T10:30:00Z"}' \
  "invalid #2 (non-UUID event_id)"

send_message "{\"event_id\":\"$(cat /proc/sys/kernel/random/uuid)\",\"developer_id\":\"dev-1\",\"metric_type\":\"invalid_type\",\"value\":5,\"repository\":\"org/api\",\"timestamp\":\"2026-04-15T10:30:00Z\"}" \
  "invalid #3 (invalid metric_type)"

send_message "{\"event_id\":\"$(cat /proc/sys/kernel/random/uuid)\",\"developer_id\":\"dev-1\",\"metric_type\":\"commits\",\"value\":5,\"repository\":\"org/api\",\"timestamp\":\"2099-12-31T23:59:59Z\"}" \
  "invalid #4 (future timestamp)"

echo ""
echo "=== Sending duplicate messages ==="

send_message "{\"event_id\":\"$DUPLICATE_UUID\",\"developer_id\":\"dev-1\",\"metric_type\":\"commits\",\"value\":99,\"repository\":\"org/api\",\"timestamp\":\"2026-04-15T10:30:00Z\"}" \
  "duplicate #1 (same event_id as valid #1)"

send_message "{\"event_id\":\"$DUPLICATE_UUID\",\"developer_id\":\"dev-2\",\"metric_type\":\"pull_requests\",\"value\":10,\"repository\":\"org/frontend\",\"timestamp\":\"2026-04-15T10:30:00Z\"}" \
  "duplicate #2 (same event_id as valid #1)"

echo ""
echo "=== Seed complete: 20 valid + 4 invalid + 2 duplicates ===" 
echo ""
echo "✓ All messages sent successfully!"
echo ""
echo "To verify, check the processor logs:"
echo "  docker logs -f developer-metrics-pipeline-processor-1"  