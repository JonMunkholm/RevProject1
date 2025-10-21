package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"time"

	"github.com/JonMunkholm/RevProject1/internal/awsutil"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/joho/godotenv"
)

func main() {
	log.SetFlags(0)
	if err := run(context.Background()); err != nil {
		log.Fatalf("enqueue: %v", err)
	}
}

func run(ctx context.Context) error {
	_ = godotenv.Load()

	opts, err := parseOptions(os.Stdin)
	if err != nil {
		return err
	}

	client, err := awsutil.NewSQSClient(ctx)
	if err != nil {
		return fmt.Errorf("init sqs client: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	if _, err := client.SendMessage(ctx, opts.toInput()); err != nil {
		return fmt.Errorf("send message: %w", err)
	}

	return nil
}

type options struct {
	queueURL string
	body     string
	delay    int32
}

func parseOptions(r io.Reader) (options, error) {
	opts := options{
		queueURL: os.Getenv("EMBED_QUEUE_URL"),
	}
	body := flag.String("body", "", "Message body to send (optional; reads stdin when empty)")
	bodyFile := flag.String("body-file", "", "Read message body from file path")
	flag.StringVar(&opts.queueURL, "queue-url", opts.queueURL, "SQS queue URL (defaults to EMBED_QUEUE_URL env)")
	delay := 0
	flag.IntVar(&delay, "delay-seconds", 0, "SQS delay seconds")
	flag.Parse()

	if strings.TrimSpace(opts.queueURL) == "" {
		return options{}, errors.New("queue url must be provided (set EMBED_QUEUE_URL or use -queue-url)")
	}

	switch {
	case *bodyFile != "":
		data, err := os.ReadFile(*bodyFile)
		if err != nil {
			return options{}, err
		}
		opts.body = string(data)
	case *body != "":
		opts.body = *body
	default:
		buf, err := io.ReadAll(r)
		if err != nil {
			return options{}, err
		}
		opts.body = string(buf)
	}

	opts.body = strings.TrimSpace(opts.body)
	if opts.body == "" {
		return options{}, errors.New("message body cannot be empty")
	}

	if delay < 0 {
		return options{}, errors.New("delay seconds cannot be negative")
	}
	opts.delay = int32(delay)

	return opts, nil
}

func (o options) toInput() *sqs.SendMessageInput {
	input := &sqs.SendMessageInput{
		QueueUrl:    aws.String(o.queueURL),
		MessageBody: aws.String(o.body),
	}
	if o.delay > 0 {
		input.DelaySeconds = o.delay
	}
	return input
}
