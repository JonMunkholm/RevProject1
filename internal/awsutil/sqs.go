package awsutil

import (
	"context"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
)

// NewSQSClient returns an SQS client that honors AWS_ENDPOINT_URL_SQS/AWS_SQS_ENDPOINT overrides.
func NewSQSClient(ctx context.Context) (*sqs.Client, error) {
	endpoint := os.Getenv("AWS_ENDPOINT_URL_SQS")
	if endpoint == "" {
		endpoint = os.Getenv("AWS_SQS_ENDPOINT")
	}

	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, err
	}

	if endpoint == "" {
		return sqs.NewFromConfig(cfg), nil
	}

	cfg.BaseEndpoint = aws.String(endpoint)
	return sqs.NewFromConfig(cfg), nil
}
