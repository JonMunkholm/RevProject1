package awsutil

import (
	"context"
	"os"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
)

// CloudWatchPublisher wraps PutMetricData calls for worker metrics.
type CloudWatchPublisher struct {
	client     *cloudwatch.Client
	namespace  string
	dimensions []types.Dimension
	dryRun     bool
}

// NewCloudWatchPublisher creates a CloudWatch client honoring AWS_ENDPOINT_URL_CLOUDWATCH overrides.
func NewCloudWatchPublisher(ctx context.Context, namespace string, dimensions map[string]string) (*CloudWatchPublisher, error) {
	endpoint := os.Getenv("AWS_ENDPOINT_URL_CLOUDWATCH")
	if endpoint == "" {
		endpoint = os.Getenv("AWS_CLOUDWATCH_ENDPOINT")
	}

	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, err
	}

	var client *cloudwatch.Client
	if endpoint == "" {
		client = cloudwatch.NewFromConfig(cfg)
	} else {
		cfg.BaseEndpoint = aws.String(endpoint)
		client = cloudwatch.NewFromConfig(cfg)
	}

	dims := make([]types.Dimension, 0, len(dimensions))
	for k, v := range dimensions {
		dims = append(dims, types.Dimension{
			Name:  aws.String(k),
			Value: aws.String(v),
		})
	}

	return &CloudWatchPublisher{
		client:     client,
		namespace:  namespace,
		dimensions: dims,
	}, nil
}

// Publish emits a metric datum to CloudWatch with optional extra dimensions.
func (p *CloudWatchPublisher) Publish(ctx context.Context, name string, value float64, unit types.StandardUnit, extraDims map[string]string) error {
	if p == nil || p.client == nil || p.dryRun {
		return nil
	}

	datumDims := make([]types.Dimension, len(p.dimensions), len(p.dimensions)+len(extraDims))
	copy(datumDims, p.dimensions)
	for k, v := range extraDims {
		datumDims = append(datumDims, types.Dimension{
			Name:  aws.String(k),
			Value: aws.String(v),
		})
	}

	input := &cloudwatch.PutMetricDataInput{
		Namespace: aws.String(p.namespace),
		MetricData: []types.MetricDatum{
			{
				MetricName: aws.String(name),
				Value:      aws.Float64(value),
				Timestamp:  aws.Time(time.Now().UTC()),
				Unit:       unit,
				Dimensions: datumDims,
			},
		},
	}

	_, err := p.client.PutMetricData(ctx, input)
	return err
}

// WithDryRun returns a shallow copy respecting the dryRun flag (skips PutMetricData).
func (p *CloudWatchPublisher) WithDryRun(dry bool) *CloudWatchPublisher {
	if p == nil {
		return nil
	}
	clone := *p
	clone.dryRun = dry
	return &clone
}
