.PHONY: build test lint run stop seed clean logs

# Build all services
build:
	docker compose build

# Run all services
run:
	docker compose up -d

# Run with logs visible
run-logs:
	docker compose up

# Stop all services
stop:
	docker compose down

# Run seed script
seed:
	chmod +x scripts/seed.sh
	@bash scripts/seed.sh

# Run unit tests for processor
test-processor:
	cd services/processor && go test ./... -v -count=1

# Run unit tests for aggregator
test-aggregator:
	cd services/aggregator && go test ./... -v -count=1

# Run all tests
test:
	$(MAKE) test-processor
	$(MAKE) test-aggregator

	# Run all tests
test: test-processor test-aggregator

# Lint (requires golangci-lint)
lint:
	cd services/processor && golangci-lint run ./...
	cd services/aggregator && golangci-lint run ./...

# View logs
logs:
	docker compose logs -f

# View processor logs
logs-processor:
	docker compose logs -f processor

# View aggregator logs
logs-aggregator:
	docker compose logs -f aggregator

# Clean up
clean:
	docker compose down -v --remove-orphans
	docker system prune -f

# Check API health
health:
	@curl -s http://localhost:8080/health | python3 -m json.tool

# Get developer summary
summary:
	@echo "Usage: make summary DEV=dev-123"
	@curl -s http://localhost:8080/metrics/$(DEV)/summary | python3 -m json.tool

# Get developer events
events:
	@echo "Usage: make events DEV=dev-123"

	# Get developer events
events:
	@echo "Usage: make events DEV=dev-123"
	@curl -s http://localhost:8080/metrics/$(DEV) | python3 -m json.tool

# Full integration test: build, run, seed, check
integration: build run
	@echo "Waiting for services to start..."
	@sleep 15
	@make seed
	@echo "Waiting for processing..."
	@sleep 10
	@echo "Checking health..."
	@make health
	@echo ""
	@echo "Checking summary for dev-123..."
	@curl -s http://localhost:8080/metrics/dev-123/summary | python3 -m json.tool
	