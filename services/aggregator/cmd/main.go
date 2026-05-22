package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/sirupsen/logrus"

	"aggregator/internal/infra/api"
	"aggregator/internal/infra/config"
	infraqueue "aggregator/internal/infra/queue"
	"aggregator/internal/infra/repository"
	"aggregator/internal/usecase"
)

func main() {
	log := logrus.New()
	log.SetFormatter(&logrus.JSONFormatter{})
	log.SetOutput(os.Stdout)

	cfg := config.Load()

	log.WithFields(logrus.Fields{
		"worker_count": cfg.WorkerCount,
		"queue":        cfg.ProcessedQueueURL,
		"port":         cfg.APIPort,
	}).Info("aggregator starting")

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	awsCfg, err := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithRegion(cfg.AWSRegion),
		awsconfig.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider("test", "test", ""),
		),
	)
	if err != nil {
		log.WithError(err).Fatal("failed to load AWS config")
	}

	sqsClient := sqs.NewFromConfig(awsCfg, func(o *sqs.Options) {
		o.BaseEndpoint = aws.String(cfg.SQSEndpoint)
	})
	dbClient := dynamodb.NewFromConfig(awsCfg, func(o *dynamodb.Options) {
		o.BaseEndpoint = aws.String(cfg.DynamoDBEndpoint)
	})

	infraqueue.WaitForQueue(ctx, sqsClient, cfg.ProcessedQueueURL, log)

	// Repositórios
	eventRepo := repository.NewDynamoEventRepository(dbClient, cfg.EventsTable, log)
	summaryRepo := repository.NewDynamoSummaryRepository(dbClient, cfg.SummaryTable, log)

	// Use cases
	aggregateUC := usecase.NewAggregateEventUseCase(eventRepo, summaryRepo, log)
	queryUC := usecase.NewQueryMetricsUseCase(eventRepo, summaryRepo)

	// Worker pool
	consumer := infraqueue.NewSQSConsumer(sqsClient, cfg.ProcessedQueueURL, log)
	pool := infraqueue.NewPool(cfg.WorkerCount, consumer, aggregateUC, log)
	go pool.Run(ctx)

	// API HTTP
	mux := http.NewServeMux()
	h := api.NewHandler(queryUC, sqsClient, dbClient, cfg.ProcessedQueueURL, cfg.EventsTable, log)
	h.Register(mux)

	server := &http.Server{
		Addr:    ":" + cfg.APIPort,
		Handler: mux,
	}

	go func() {
		log.WithField("port", cfg.APIPort).Info("API server listening")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.WithError(err).Fatal("server error")
		}
	}()

	<-ctx.Done()
	log.Info("shutting down aggregator...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.WithError(err).Error("graceful server shutdown failed")
	}

	log.Info("aggregator stopped")
}
