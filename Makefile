.PHONY: up down build test lint seed logs clean help

## up: sobe todos os containers (LocalStack + Processor + Aggregator)
up:
	docker compose up --build -d
	@echo "Aguardando serviços subirem..."
	@sleep 5
	@echo "Pronto! API disponível em http://localhost:8080"

## down: derruba todos os containers
down:
	docker compose down

## build: compila as imagens Docker sem subir
build:
	docker compose build

## test: executa todos os testes unitários
test:
	@echo "=== Processor tests ==="
	cd services/processor && go test ./... -v -count=1
	@echo ""
	@echo "=== Aggregator tests ==="
	cd services/aggregator && go test ./... -v -count=1

## lint: executa o linter (requer golangci-lint instalado)
lint:
	cd services/processor && golangci-lint run ./...
	cd services/aggregator && golangci-lint run ./...

## seed: publica mensagens de teste na fila raw-events
seed:
	@bash scripts/seed.sh

## logs: exibe logs de todos os serviços em tempo real
logs:
	docker compose logs -f

## logs-processor: logs apenas do processor
logs-processor:
	docker compose logs -f processor

## logs-aggregator: logs apenas do aggregator
logs-aggregator:
	docker compose logs -f aggregator

## clean: remove containers, volumes e imagens builadas
clean:
	docker compose down -v --rmi local

## health: verifica o health check da API
health:
	@curl -s http://localhost:8080/health | jq .

## help: exibe esta ajuda
help:
	@grep -E '^##' Makefile | sed 's/## //'
