# syntax=docker/dockerfile:1
FROM golang:1.23-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /processor ./cmd/main.go

# Imagem final mínima (distroless)
FROM gcr.io/distroless/static-debian12

COPY --from=builder /processor /processor

ENTRYPOINT ["/processor"]
