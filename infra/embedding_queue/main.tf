locals {
  resolved_dlq_name = var.dlq_name != "" ? var.dlq_name : "${var.queue_name}-dlq"
}

resource "aws_sqs_queue" "dlq" {
  name                      = local.resolved_dlq_name
  message_retention_seconds = max(var.message_retention_seconds, 604800) # DLQ keeps messages for at least 7 days
  sqs_managed_sse_enabled   = var.kms_master_key_id == ""
  visibility_timeout_seconds = var.visibility_timeout_seconds
  wait_time_seconds          = var.wait_time_seconds

  dynamic "kms_master_key_id" {
    for_each = var.kms_master_key_id == "" ? [] : [var.kms_master_key_id]
    content  = kms_master_key_id.value
  }

  tags = var.tags
}

resource "aws_sqs_queue" "primary" {
  name                        = var.queue_name
  message_retention_seconds   = var.message_retention_seconds
  visibility_timeout_seconds  = var.visibility_timeout_seconds
  wait_time_seconds           = var.wait_time_seconds
  sqs_managed_sse_enabled     = var.kms_master_key_id == ""

  dynamic "kms_master_key_id" {
    for_each = var.kms_master_key_id == "" ? [] : [var.kms_master_key_id]
    content  = kms_master_key_id.value
  }

  redrive_policy = jsonencode({
    deadLetterTargetArn = aws_sqs_queue.dlq.arn
    maxReceiveCount     = var.max_receive_count
  })

  tags = var.tags
}

data "aws_iam_policy_document" "producer" {
  statement {
    effect = "Allow"
    actions = [
      "sqs:SendMessage",
      "sqs:GetQueueAttributes"
    ]
    resources = [
      aws_sqs_queue.primary.arn
    ]
  }
}

data "aws_iam_policy_document" "consumer" {
  statement {
    effect = "Allow"
    actions = [
      "sqs:ReceiveMessage",
      "sqs:DeleteMessage",
      "sqs:GetQueueAttributes",
      "sqs:ChangeMessageVisibility",
      "sqs:ChangeMessageVisibilityBatch",
      "sqs:DeleteMessageBatch"
    ]
    resources = [
      aws_sqs_queue.primary.arn
    ]
  }

  statement {
    effect = "Allow"
    actions = [
      "sqs:SendMessage",
      "sqs:GetQueueAttributes"
    ]
    resources = [
      aws_sqs_queue.dlq.arn
    ]
  }
}
