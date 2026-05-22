package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	sqstypes "github.com/aws/aws-sdk-go-v2/service/sqs/types"
	"github.com/sirupsen/logrus"

	"processor/internal/domain"
)

// Consumer é a interface para consumir mensagens de uma fila.
type Consumer interface {
	Receive(ctx context.Context) ([]Message, error)
	Delete(ctx context.Context, receiptHandle string) error
}

// Publisher é a interface para publicar mensagens em uma fila.
type Publisher interface {
	Publish(ctx context.Context, event domain.ProcessedEvent) error
}

// Message representa uma mensagem recebida da fila com seu handle de exclusão.
type Message struct {
	ID            string
	Body          string
	ReceiptHandle string
}

// SQSConsumer implementa Consumer usando AWS SQS.
type SQSConsumer struct {
	client   *sqs.Client
	queueURL string
	log      *logrus.Logger
}

// SQSPublisher implementa Publisher usando AWS SQS.
type SQSPublisher struct {
	client   *sqs.Client
	queueURL string
	log      *logrus.Logger
}

func NewSQSConsumer(client *sqs.Client, queueURL string, log *logrus.Logger) *SQSConsumer {
	return &SQSConsumer{client: client, queueURL: queueURL, log: log}
}

func NewSQSPublisher(client *sqs.Client, queueURL string, log *logrus.Logger) *SQSPublisher {
	return &SQSPublisher{client: client, queueURL: queueURL, log: log}
}

func (c *SQSConsumer) Receive(ctx context.Context) ([]Message, error) {
	resp, err := c.client.ReceiveMessage(ctx, &sqs.ReceiveMessageInput{
		QueueUrl:            aws.String(c.queueURL),
		MaxNumberOfMessages: 10,
		WaitTimeSeconds:     5,
	})
	if err != nil {
		return nil, fmt.Errorf("sqs receive: %w", err)
	}

	msgs := make([]Message, 0, len(resp.Messages))
	for _, m := range resp.Messages {
		msgs = append(msgs, Message{
			ID:            aws.ToString(m.MessageId),
			Body:          aws.ToString(m.Body),
			ReceiptHandle: aws.ToString(m.ReceiptHandle),
		})
	}
	return msgs, nil
}

func (c *SQSConsumer) Delete(ctx context.Context, receiptHandle string) error {
	_, err := c.client.DeleteMessage(ctx, &sqs.DeleteMessageInput{
		QueueUrl:      aws.String(c.queueURL),
		ReceiptHandle: aws.String(receiptHandle),
	})
	if err != nil {
		return fmt.Errorf("sqs delete: %w", err)
	}
	return nil
}

func (p *SQSPublisher) Publish(ctx context.Context, event domain.ProcessedEvent) error {
	body, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal processed event: %w", err)
	}

	const maxRetries = 3
	for attempt := 0; attempt <= maxRetries; attempt++ {
		_, err = p.client.SendMessage(ctx, &sqs.SendMessageInput{
			QueueUrl:    aws.String(p.queueURL),
			MessageBody: aws.String(string(body)),
		})
		if err == nil {
			return nil
		}
		if attempt == maxRetries {
			break
		}
		backoff := time.Duration(math.Pow(2, float64(attempt))) * 100 * time.Millisecond
		p.log.WithFields(logrus.Fields{
			"attempt":  attempt + 1,
			"event_id": event.EventID,
		}).WithError(err).Warn("publish failed, retrying with backoff")

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(backoff):
		}
	}
	return fmt.Errorf("publish after %d retries: %w", maxRetries, err)
}

// WaitForQueue aguarda até que a fila SQS esteja acessível.
func WaitForQueue(ctx context.Context, client *sqs.Client, queueURL string, log *logrus.Logger) {
	for {
		_, err := client.GetQueueAttributes(ctx, &sqs.GetQueueAttributesInput{
			QueueUrl:       aws.String(queueURL),
			AttributeNames: []sqstypes.QueueAttributeName{sqstypes.QueueAttributeNameAll},
		})
		if err == nil {
			log.WithField("queue", queueURL).Info("queue is ready")
			return
		}
		log.WithField("queue", queueURL).Info("waiting for queue...")
		select {
		case <-ctx.Done():
			return
		case <-time.After(2 * time.Second):
		}
	}
}
