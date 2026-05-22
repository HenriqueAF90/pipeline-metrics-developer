package config

import (
	"os"
	"strconv"
)

type Config struct {
	AWSRegion         string
	SQSEndpoint       string
	DynamoDBEndpoint  string
	ProcessedQueueURL string
	WorkerCount       int
	APIPort           string
	EventsTable       string
	SummaryTable      string
}

func Load() Config {
	return Config{
		AWSRegion:         getEnv("AWS_REGION", "us-east-1"),
		SQSEndpoint:       getEnv("SQS_ENDPOINT", "http://localstack:4566"),
		DynamoDBEndpoint:  getEnv("DYNAMODB_ENDPOINT", "http://localstack:4566"),
		ProcessedQueueURL: getEnv("PROCESSED_QUEUE_URL", "http://localstack:4566/000000000000/processed-events"),
		WorkerCount:       getEnvInt("WORKER_COUNT", 3),
		APIPort:           getEnv("API_PORT", "8080"),
		EventsTable:       getEnv("EVENTS_TABLE", "events"),
		SummaryTable:      getEnv("SUMMARY_TABLE", "developer_summary"),
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
