package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/sirupsen/logrus"

	"processor/internal/infra/config"
	"processor/internal/infra/queue"
	"processor/internal/infra/worker"
	"processor/internal/usecase"
)

func main() {
	log := logrus.New()
	log.SetFormatter(&logrus.JSONFormatter{})
	log.SetOutput(os.Stdout)

	cfg := config.Load()

	log.WithFields(logrus.Fields{
		"worker_count": cfg.WorkerCount,
		"processor_id": cfg.ProcessorID,
		"queue":        cfg.RawQueueURL,
	}).Info("processor starting")

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

	queue.WaitForQueue(ctx, sqsClient, cfg.RawQueueURL, log)

	consumer := queue.NewSQSConsumer(sqsClient, cfg.RawQueueURL, log)
	publisher := queue.NewSQSPublisher(sqsClient, cfg.ProcessedQueueURL, log)
	uc := usecase.NewProcessEventUseCase(publisher, cfg.ProcessorID, log)
	pool := worker.NewPool(cfg.WorkerCount, consumer, uc, log)

	pool.Run(ctx)

	log.Info("processor stopped")
}
