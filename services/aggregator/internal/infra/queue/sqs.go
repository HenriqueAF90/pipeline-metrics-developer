package queue

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	sqstypes "github.com/aws/aws-sdk-go-v2/service/sqs/types"
	"github.com/sirupsen/logrus"
)

// Message representa uma mensagem SQS recebida.
type Message struct {
	ID            string
	Body          string
	ReceiptHandle string
}

// SQSConsumer consome mensagens da fila processed-events.
type SQSConsumer struct {
	client   *sqs.Client
	queueURL string
	log      *logrus.Logger
}

func NewSQSConsumer(client *sqs.Client, queueURL string, log *logrus.Logger) *SQSConsumer {
	return &SQSConsumer{client: client, queueURL: queueURL, log: log}
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

// WaitForQueue aguarda a fila estar acessível.
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
