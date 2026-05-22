# Pipeline Metrics Developer

Pipeline de métricas de produtividade de desenvolvedores com dois serviços Go que se comunicam exclusivamente via filas SQS (LocalStack).

```
[SQS: raw-events] → [Processor] → [SQS: processed-events] → [Aggregator] → [DynamoDB]
                                                                                  ↓
                                                                         [API REST :8080]
```

---

## Requisitos

- Docker
- Docker Compose (v2)

---

## Como rodar

```bash
docker compose up --build
```

Após subir, aguarde os serviços conectarem às filas. O Aggregator expõe a API em `http://localhost:8080`.

---

## Como testar

### 1. Seed de dados

Publique mensagens de teste na fila (inclui válidas, inválidas e duplicadas):

```bash
bash scripts/seed.sh
```

### 2. Verificar health check

```bash
curl http://localhost:8080/health
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
curl http://localhost:8080/metrics/dev-1
```

### 4. Consultar resumo agregado de um desenvolvedor

```bash
curl http://localhost:8080/metrics/dev-1/summary
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

## Ambientes

### Desenvolvimento

Use o ambiente local com LocalStack e Docker Compose:

```bash
docker compose up --build
```

- LocalStack cria SQS e DynamoDB locais.
- O serviço `processor` consome mensagens de `raw-events` e publica em `processed-events`.
- O serviço `aggregator` lê de `processed-events`, persiste em DynamoDB e expõe a API REST.
- A API fica disponível em `http://localhost:8080`.

### Produção

Para produção, use recursos AWS reais em vez de LocalStack.

- Construa as imagens dos serviços com os `Dockerfile` em `services/processor` e `services/aggregator`.
- Configure os endpoints AWS corretos em variáveis de ambiente.
- Exemplo de variáveis de produção:
  - `AWS_REGION=us-east-1`
  - `SQS_ENDPOINT=https://sqs.us-east-1.amazonaws.com`
  - `DYNAMODB_ENDPOINT=https://dynamodb.us-east-1.amazonaws.com`
  - `RAW_QUEUE_URL=https://sqs.us-east-1.amazonaws.com/000000000000/raw-events`
  - `PROCESSED_QUEUE_URL=https://sqs.us-east-1.amazonaws.com/000000000000/processed-events`
  - `EVENTS_TABLE=events`
  - `SUMMARY_TABLE=developer_summary`

> Observação: a configuração `docker-compose.yml` atual é destinada ao ambiente local com LocalStack. Em produção, você deve implantar os containers em um orquestrador ou serviço de containers com as variáveis de ambiente adequadas.

---

## Testes unitários

```bash
cd services/processor && go test ./... -v
cd services/aggregator && go test ./... -v
```

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
