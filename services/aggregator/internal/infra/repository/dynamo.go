package repository

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	dynamotypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/sirupsen/logrus"

	"aggregator/internal/domain"
)

// EventRepository define as operações de persistência de eventos.
type EventRepository interface {
	Exists(ctx context.Context, eventID string) (bool, error)
	Save(ctx context.Context, event domain.ProcessedEvent) error
	FindByDeveloper(ctx context.Context, developerID string) ([]domain.ProcessedEvent, error)
}

// SummaryRepository define as operações de persistência do summary agregado.
type SummaryRepository interface {
	Get(ctx context.Context, developerID string) (domain.DeveloperSummary, error)
	Save(ctx context.Context, summary domain.DeveloperSummary) error
}

// DynamoEventRepository implementa EventRepository usando DynamoDB.
type DynamoEventRepository struct {
	client    *dynamodb.Client
	tableName string
	log       *logrus.Logger
}

// DynamoSummaryRepository implementa SummaryRepository usando DynamoDB.
type DynamoSummaryRepository struct {
	client    *dynamodb.Client
	tableName string
	log       *logrus.Logger
}

func NewDynamoEventRepository(client *dynamodb.Client, tableName string, log *logrus.Logger) *DynamoEventRepository {
	return &DynamoEventRepository{client: client, tableName: tableName, log: log}
}

func NewDynamoSummaryRepository(client *dynamodb.Client, tableName string, log *logrus.Logger) *DynamoSummaryRepository {
	return &DynamoSummaryRepository{client: client, tableName: tableName, log: log}
}

func (r *DynamoEventRepository) Exists(ctx context.Context, eventID string) (bool, error) {
	result, err := r.client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(r.tableName),
		Key: map[string]dynamotypes.AttributeValue{
			"event_id": &dynamotypes.AttributeValueMemberS{Value: eventID},
		},
		ProjectionExpression: aws.String("event_id"),
	})
	if err != nil {
		return false, fmt.Errorf("dynamodb get item: %w", err)
	}
	return result.Item != nil, nil
}

func (r *DynamoEventRepository) Save(ctx context.Context, event domain.ProcessedEvent) error {
	item, err := attributevalue.MarshalMap(event)
	if err != nil {
		return fmt.Errorf("marshal event: %w", err)
	}
	_, err = r.client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(r.tableName),
		Item:      item,
	})
	if err != nil {
		return fmt.Errorf("dynamodb put item: %w", err)
	}
	return nil
}

func (r *DynamoEventRepository) FindByDeveloper(ctx context.Context, developerID string) ([]domain.ProcessedEvent, error) {
	result, err := r.client.Scan(ctx, &dynamodb.ScanInput{
		TableName:        aws.String(r.tableName),
		FilterExpression: aws.String("developer_id = :devId"),
		ExpressionAttributeValues: map[string]dynamotypes.AttributeValue{
			":devId": &dynamotypes.AttributeValueMemberS{Value: developerID},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("dynamodb scan: %w", err)
	}

	var events []domain.ProcessedEvent
	if err := attributevalue.UnmarshalListOfMaps(result.Items, &events); err != nil {
		return nil, fmt.Errorf("unmarshal events: %w", err)
	}
	return events, nil
}

func (r *DynamoSummaryRepository) Get(ctx context.Context, developerID string) (domain.DeveloperSummary, error) {
	result, err := r.client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(r.tableName),
		Key: map[string]dynamotypes.AttributeValue{
			"developer_id": &dynamotypes.AttributeValueMemberS{Value: developerID},
		},
	})
	if err != nil {
		return domain.DeveloperSummary{DeveloperID: developerID}, fmt.Errorf("dynamodb get summary: %w", err)
	}
	if result.Item == nil {
		return domain.DeveloperSummary{DeveloperID: developerID}, nil
	}

	var summary domain.DeveloperSummary
	if err := attributevalue.UnmarshalMap(result.Item, &summary); err != nil {
		return domain.DeveloperSummary{DeveloperID: developerID}, fmt.Errorf("unmarshal summary: %w", err)
	}
	return summary, nil
}

func (r *DynamoSummaryRepository) Save(ctx context.Context, summary domain.DeveloperSummary) error {
	item, err := attributevalue.MarshalMap(summary)
	if err != nil {
		return fmt.Errorf("marshal summary: %w", err)
	}
	_, err = r.client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(r.tableName),
		Item:      item,
	})
	if err != nil {
		return fmt.Errorf("dynamodb put summary: %w", err)
	}
	return nil
}
