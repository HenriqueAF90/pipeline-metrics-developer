# Pipeline Metrics Developer

Pipeline de métricas de produtividade de desenvolvedores construído com dois serviços Go que se comunicam exclusivamente via filas SQS (LocalStack).

```
[SQS: raw-events] → [Processor] → [SQS: processed-events] → [Aggregator] → [DynamoDB]
                                                                                  ↓
                                                                         [API REST :8080]
```

---

## Requisitos

- Docker e Docker Compose (v2)
- `make` (opcional, mas recomendado)
- `jq` (opcional, para formatar respostas da API)

---

## Como rodar

```bash
# Sobe tudo com um único comando
docker compose up --build

# Ou com make:
make up
```

Após subir, aguarde os serviços conectarem às filas. O Aggregator expõe a API em `http://localhost:8080`.

---

## Como testar

### 1. Seed de dados

Publique mensagens de teste na fila (inclui válidas, inválidas e duplicadas):

```bash
make seed
# ou diretamente:
bash scripts/seed.sh
```

### 2. Verificar health check

```bash
curl http://localhost:8080/health | jq .
```

Resposta esperada:
```json
{
  "status": "ok",
  "details": {
    "dynamodb": "connected",
    "sqs": "connected"
  }
}
```

### 3. Consultar eventos de um desenvolvedor

```bash
curl http://localhost:8080/metrics/dev-1 | jq .
```

### 4. Consultar resumo agregado de um desenvolvedor

```bash
curl http://localhost:8080/metrics/dev-1/summary | jq .
```

Resposta esperada:
```json
{
  "developer_id": "dev-1",
  "total_commits": 42,
  "total_pull_requests": 10,
  "avg_review_time_minutes": 45.5,
  "events_processed": 15,
  "last_activity": "2026-04-15T10:30:00Z"
}
```

---

## Testes unitários

```bash
make test

# Apenas processor:
cd services/processor && go test ./... -v

# Apenas aggregator:
cd services/aggregator && go test ./... -v
```

---

## Comandos Make disponíveis

| Comando | Descrição |
|---------|-----------|
| `make up` | Sobe todos os containers |
| `make down` | Derruba todos os containers |
| `make build` | Compila imagens sem subir |
| `make test` | Executa todos os testes unitários |
| `make seed` | Publica mensagens de teste |
| `make logs` | Logs de todos os serviços |
| `make logs-processor` | Logs apenas do Processor |
| `make logs-aggregator` | Logs apenas do Aggregator |
| `make health` | Checa o endpoint /health |
| `make clean` | Remove containers, volumes e imagens |

---

## API REST

Base URL: `http://localhost:8080`

| Método | Endpoint | Descrição |
|--------|----------|-----------|
| `GET` | `/health` | Health check (SQS + DynamoDB) |
| `GET` | `/metrics/:developer_id` | Todos os eventos de um developer |
| `GET` | `/metrics/:developer_id/summary` | Resumo agregado de um developer |

---

## Arquitetura

### Estrutura de diretórios

```
.
├── services/
│   ├── processor/
│   │   ├── cmd/main.go                   # Entrypoint
│   │   ├── internal/
│   │   │   ├── domain/                   # Entidades e regras de negócio (Validate)
│   │   │   ├── usecase/                  # Orquestração: validar → enriquecer → publicar
│   │   │   └── infra/
│   │   │       ├── queue/                # Adapter SQS (Consumer + Publisher)
│   │   │       ├── worker/               # Worker pool concorrente
│   │   │       └── config/               # Variáveis de ambiente
│   │   └── Dockerfile
│   └── aggregator/
│       ├── cmd/main.go                   # Entrypoint
│       ├── internal/
│       │   ├── domain/                   # Entidades e lógica de agregação (Apply)
│       │   ├── usecase/                  # Orquestração: idempotência → persistir → agregar
│       │   └── infra/
│       │       ├── queue/                # Adapter SQS + worker pool
│       │       ├── repository/           # Adapter DynamoDB (events + summary)
│       │       ├── api/                  # Handlers HTTP
│       │       └── config/               # Variáveis de ambiente
│       └── Dockerfile
├── infra/localstack/init-aws.sh          # Cria filas e tabelas no startup
├── scripts/seed.sh                       # Publica mensagens de teste
├── docker-compose.yml
├── Makefile
└── README.md
```

### Decisões de design

**Clean Architecture:** cada serviço é dividido em três camadas — `domain` (regras puras, sem dependências externas), `usecase` (orquestra o fluxo usando interfaces), e `infra` (implementações concretas: SQS, DynamoDB, HTTP). Isso garante testabilidade por injeção de dependência e mocks simples.

**Worker pool:** ambos os serviços utilizam um pool de goroutines (`WORKER_COUNT` configurável) com um canal bufferizado para separar o recebimento de mensagens do processamento, evitando gargalos.

**Idempotência:** antes de processar qualquer evento, o Aggregator verifica se o `event_id` já existe na tabela `events` do DynamoDB. Eventos duplicados são descartados sem atualizar o summary.

**DLQ:** eventos com erro **de infraestrutura** (ex: DynamoDB indisponível) NÃO são deletados da fila, forçando o SQS a retentar. Após 3 tentativas, a mensagem vai automaticamente para a DLQ. Eventos com erro de negócio (inválidos) são deletados imediatamente para não consumir tentativas.

**Graceful shutdown:** ao receber `SIGTERM` ou `SIGINT`, ambos os serviços param de receber novas mensagens, drenam os workers em andamento e encerram o servidor HTTP com timeout de 15s.

**Logs estruturados:** todos os logs são emitidos em JSON via `logrus`, com correlação por `event_id` e `worker_id`.

**Dockerfiles multi-stage:** a imagem final usa `gcr.io/distroless/static-debian12`, sem shell ou ferramentas desnecessárias, resultando em imagens mínimas e seguras.

---

## Variáveis de ambiente

### Processor

| Variável | Padrão | Descrição |
|----------|--------|-----------|
| `AWS_REGION` | `us-east-1` | Região AWS |
| `SQS_ENDPOINT` | `http://localstack:4566` | Endpoint SQS |
| `RAW_QUEUE_URL` | `http://localstack:4566/000000000000/raw-events` | URL da fila de entrada |
| `PROCESSED_QUEUE_URL` | `http://localstack:4566/000000000000/processed-events` | URL da fila de saída |
| `WORKER_COUNT` | `5` | Número de workers concorrentes |
| `PROCESSOR_ID` | `processor-1` | Identificador desta instância |

### Aggregator

| Variável | Padrão | Descrição |
|----------|--------|-----------|
| `AWS_REGION` | `us-east-1` | Região AWS |
| `SQS_ENDPOINT` | `http://localstack:4566` | Endpoint SQS |
| `DYNAMODB_ENDPOINT` | `http://localstack:4566` | Endpoint DynamoDB |
| `PROCESSED_QUEUE_URL` | `http://localstack:4566/000000000000/processed-events` | URL da fila de entrada |
| `WORKER_COUNT` | `3` | Número de workers concorrentes |
| `API_PORT` | `8080` | Porta da API REST |
| `EVENTS_TABLE` | `events` | Tabela de eventos no DynamoDB |
| `SUMMARY_TABLE` | `developer_summary` | Tabela de summaries no DynamoDB |
