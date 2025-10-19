output "queue_url" {
  description = "URL of the primary embedding jobs queue."
  value       = aws_sqs_queue.primary.id
}

output "queue_arn" {
  description = "ARN of the primary embedding jobs queue."
  value       = aws_sqs_queue.primary.arn
}

output "dlq_url" {
  description = "URL of the dead-letter queue."
  value       = aws_sqs_queue.dlq.id
}

output "dlq_arn" {
  description = "ARN of the dead-letter queue."
  value       = aws_sqs_queue.dlq.arn
}

output "producer_policy_json" {
  description = "IAM policy JSON granting producer permissions to publish to the queue."
  value       = data.aws_iam_policy_document.producer.json
}

output "consumer_policy_json" {
  description = "IAM policy JSON granting worker permissions to consume and DLQ interactions."
  value       = data.aws_iam_policy_document.consumer.json
}
