#!/bin/bash
set -e

echo "Creating AWS resources on LocalStack..."

# Create DLQ queues first
awslocal sqs create-queue --queue-name raw-events-dlq
awslocal sqs create-queue --queue-name processed-events-dlq

# Create main queues with RedrivePolicy pointing to DLQs
awslocal sqs create-queue --queue-name raw-events \
  --attributes '{"RedrivePolicy":"{\"deadLetterTargetArn\":\"arn:aws:sqs:us-east-1:000000000000:raw-events-dlq\",\"maxReceiveCount\":\"3\"}"}'

awslocal sqs create-queue --queue-name processed-events \
  --attributes '{"RedrivePolicy":"{\"deadLetterTargetArn\":\"arn:aws:sqs:us-east-1:000000000000:processed-events-dlq\",\"maxReceiveCount\":\"3\"}"}'

# Create DynamoDB tables
awslocal dynamodb create-table \
  --table-name events \
  --attribute-definitions AttributeName=event_id,AttributeType=S \
  --key-schema AttributeName=event_id,KeyType=HASH \
  --billing-mode PAY_PER_REQUEST

awslocal dynamodb create-table \
  --table-name developer_summary \
  --attribute-definitions AttributeName=developer_id,AttributeType=S \
  --key-schema AttributeName=developer_id,KeyType=HASH \
  --billing-mode PAY_PER_REQUEST

echo "All AWS resources created successfully!"
echo "Queues: raw-events, raw-events-dlq, processed-events, processed-events-dlq"
echo "Tables: events, developer_summary"