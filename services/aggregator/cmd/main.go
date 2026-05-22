import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jcristsv/aggregator/internal/domain"
	"github.com/jcristsv/aggregator/internal/infra/api"
	"github.com/jcristsv/aggregator/internal/infra/config"
	"github.com/jcristsv/aggregator/internal/infra/queue"
	"github.com/jcristsv/aggregator/internal/infra/repository"
	"github.com/jcristsv/aggregator/internal/usecase"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	cfg := config.Load()

	logger.Info("starting aggregator",
		"processed_queue", cfg.ProcessedQueue,
		"events_table", cfg.EventsTable,
		"summary_table", cfg.SummaryTable,
		"api_port", cfg.APIPort,
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
}
ctx, cancel := context.WithCancel(context.Background())
defer cancel()

// Wait for LocalStack to be ready
var sqsConsumer *queue.SQSConsumer
var err error
for i := 0; i < 30; i++ {
    sqsConsumer, err = queue.NewSQSConsumer(ctx, cfg.AWSEndpoint, cfg.AWSRegion, cfg.ProcessedQueue)
    if err == nil {
        break
    }

    logger.Warn("waiting for SQS to be ready", "attempt", i+1, "error", err.Error())
    time.Sleep(2 * time.Second)
}

if err != nil {
    logger.Error("failed to connect to processed-events queue", "error", err.Error())
    os.Exit(1)
}

var eventRepo *repository.DynamoDBEventRepository
for i := 0; i < 30; i++ {
    eventRepo, err = repository.NewDynamoDBEventRepository(ctx, cfg.AWSEndpoint, cfg.AWSRegion, cfg.EventsTable)
    if err == nil {
        break
    }

    logger.Warn("waiting for DynamoDB to be ready", "attempt", i+1, "error", err.Error())
    time.Sleep(2 * time.Second)
}

if err != nil {
    logger.Error("failed to connect to events table", "error", err.Error())
    os.Exit(1)
}
var summaryRepo *repository.DynamoDBSummaryRepository
for i := 0; i < 30; i++ {
    summaryRepo, err = repository.NewDynamoDBSummaryRepository(
        ctx,
        cfg.AwsEndpoint,
        cfg.AwsRegion,
        cfg.SummaryTable,
    )
    if err == nil {
        break
    }

    logger.Warn(
        "waiting for DynamoDB summary table",
        "attempt", i+1,
        "error", err.Error(),
    )

    time.Sleep(2 * time.Second)
}

if err != nil {
    logger.Error(
        "failed to connect to summary table",
        "error", err.Error(),
    )
    os.Exit(1)
}

aggregateUC := usecase.NewAggregateEventUseCase(
    eventRepo,
    summaryRepo,
    logger,
)

// Setup API
gin.SetMode(gin.ReleaseMode)
router := gin.New()
router.Use(gin.Recovery())

handler := api.NewHandler(
    eventRepo,
    summaryRepo,
    sqsConsumer.Client(),
    eventRepo.Client(),
    sqsConsumer.QueueURL(),
)

handler.RegisterRoutes(router)

server := &http.Server{
    Addr:    ":" + cfg.APIPort,
    Handler: router,
}

// Start API server
go func() {
    logger.Info("API server starting", "port", cfg.APIPort)
    if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
        logger.Error("API server error", "error", err.Error())
    }
}()

// Graceful shutdown
sigCh := make(chan os.Signal, 1)
signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

go func() {
    <-sigCh
    logger.Info("shutdown signal received")
    cancel()

    shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer shutdownCancel()

    if err := server.Shutdown(shutdownCtx); err != nil {
        logger.Error("server shutdown error", "error", err.Error())
    }
}()
// Main consumer loop
for {
    select {
    case <-ctx.Done():
        logger.Info("aggregator shutdown complete")
        return

    default:
        messages, err := sqsConsumer.ReceiveMessages(ctx)
        if err != nil {
            if ctx.Err() != nil {
                continue
            }

            logger.Error("failed to receive messages", "error", err.Error())
            time.Sleep(1 * time.Second)
            continue
        }

        for _, msg := range messages {
            var event domain.ProcessedEvent

            if err := json.Unmarshal([]byte(*msg.Body), &event); err != nil {
                logger.Error("failed to unmarshal message", "error", err.Error())
                continue
            }

            if err := aggregateUC.Execute(ctx, event); err != nil {
                logger.Error("failed to aggregate event",
                    "event_id", event.EventID,
                    "error", err.Error(),
                )
                continue
            }

            if err := sqsConsumer.DeleteMessage(ctx, msg.ReceiptHandle); err != nil {
                logger.Error("failed to delete message", "error", err.Error())
            }
        }
    }
}
if err := sqsConsumer.DeleteMessage(ctx, msg.ReceiptHandle); err != nil {
    logger.Error("failed to delete message", "error", err.Error())
}
    }
}
}