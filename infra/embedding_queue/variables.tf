variable "queue_name" {
  description = "Name of the primary embedding jobs queue."
  type        = string
}

variable "dlq_name" {
  description = "Name of the dead-letter queue. Defaults to <queue_name>-dlq."
  type        = string
  default     = ""
}

variable "visibility_timeout_seconds" {
  description = "Visibility timeout for the worker to complete a job (seconds)."
  type        = number
  default     = 30
}

variable "message_retention_seconds" {
  description = "How long SQS retains messages that are not deleted (seconds)."
  type        = number
  default     = 345600 # 4 days
}

variable "max_receive_count" {
  description = "How many times a message can be received before being moved to the DLQ."
  type        = number
  default     = 3
}

variable "wait_time_seconds" {
  description = "Long polling wait time for ReceiveMessage calls."
  type        = number
  default     = 0
}

variable "kms_master_key_id" {
  description = "Optional CMK ARN for SQS SSE. When empty the AWS managed SQS key is used."
  type        = string
  default     = ""
}

variable "tags" {
  description = "Tags to apply to all resources."
  type        = map(string)
  default     = {}
}
