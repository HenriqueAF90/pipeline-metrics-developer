package config

import (
	"os"
	"strconv"
)

// Config reúne todas as configurações do serviço Processor lidas de variáveis de ambiente.
type Config struct {
	AWSRegion         string
	SQSEndpoint       string
	RawQueueURL       string
	ProcessedQueueURL string
	WorkerCount       int
	ProcessorID       string
}

func Load() Config {
	return Config{
		AWSRegion:         getEnv("AWS_REGION", "us-east-1"),
		SQSEndpoint:       getEnv("SQS_ENDPOINT", "http://localstack:4566"),
		RawQueueURL:       getEnv("RAW_QUEUE_URL", "http://localstack:4566/000000000000/raw-events"),
		ProcessedQueueURL: getEnv("PROCESSED_QUEUE_URL", "http://localstack:4566/000000000000/processed-events"),
		WorkerCount:       getEnvInt("WORKER_COUNT", 5),
		ProcessorID:       getEnv("PROCESSOR_ID", "processor-1"),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return fallback
}
