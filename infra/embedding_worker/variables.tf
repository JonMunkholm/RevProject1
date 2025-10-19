variable "cluster_arn" {
  description = "ECS cluster ARN where the worker service should run."
  type        = string
}

variable "service_name" {
  description = "Name for the ECS service."
  type        = string
}

variable "container_image" {
  description = "Docker image for the embedding worker."
  type        = string
}

variable "cpu" {
  description = "CPU units for the task (e.g., 256 = 0.25 vCPU)."
  type        = number
  default     = 512
}

variable "memory" {
  description = "Memory (MB) for the task."
  type        = number
  default     = 1024
}

variable "desired_count" {
  description = "Initial desired count for the service."
  type        = number
  default     = 2
}

variable "queue_url" {
  description = "Embedding jobs queue URL for the worker."
  type        = string
}

variable "dlq_url" {
  description = "Dead-letter queue URL (for metrics)."
  type        = string
}

variable "subnet_ids" {
  description = "Private subnet IDs for the service."
  type        = list(string)
}

variable "security_group_ids" {
  description = "Security group IDs for the tasks."
  type        = list(string)
}

variable "secrets_manager_arns" {
  description = "List of Secrets Manager ARNs the task may read."
  type        = list(string)
  default     = []
}

variable "log_group_name" {
  description = "CloudWatch log group for container logs."
  type        = string
}

variable "tags" {
  description = "Tags to apply to all resources."
  type        = map(string)
  default     = {}
}

variable "autoscale_max_count" {
  description = "Maximum number of tasks for autoscaling."
  type        = number
  default     = 10
}

variable "scale_up_queue_messages" {
  description = "Approximate number of messages where scale up should trigger."
  type        = number
  default     = 100
}

variable "scale_down_queue_messages" {
  description = "Approximate number of messages to scale down."
  type        = number
  default     = 10
}
