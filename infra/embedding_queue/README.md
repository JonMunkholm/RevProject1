# Embedding Queue Module

Terraform module that provisions the asynchronous embedding job SQS queue and companion
dead-letter queue, along with convenience IAM policy documents for producers and workers.

## Usage

```hcl
module "embedding_queue" {
  source = "../embedding_queue"

  queue_name                 = "rev-embedding-jobs"
  visibility_timeout_seconds = 60
  message_retention_seconds  = 345600
  max_receive_count          = 3
  tags = {
    Project = "revproject1"
    Env     = var.environment
  }
}

resource "aws_iam_role_policy" "embedding_producer" {
  role   = aws_iam_role.api.name
  policy = module.embedding_queue.producer_policy_json
}
```

## Outputs

| Output                | Description                                      |
|-----------------------|--------------------------------------------------|
| `queue_url`           | SQS URL for publishing messages.                 |
| `queue_arn`           | ARN of the primary queue.                         |
| `dlq_url`             | SQS URL of the dead-letter queue.                 |
| `dlq_arn`             | ARN of the dead-letter queue.                     |
| `producer_policy_json`| IAM policy JSON granting send permissions.       |
| `consumer_policy_json`| IAM policy JSON granting receive/delete + DLQ.   |

See `variables.tf` for tunable parameters (visibility timeout, retention,
custom DLQ name, optional customer-managed KMS key, etc.).
