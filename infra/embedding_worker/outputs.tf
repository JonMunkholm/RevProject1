output "service_name" {
  description = "Name of the ECS service."
  value       = aws_ecs_service.this.name
}

output "task_definition_arn" {
  description = "ARN of the worker task definition."
  value       = aws_ecs_task_definition.this.arn
}

output "task_role_arn" {
  description = "ARN of the task IAM role."
  value       = aws_iam_role.task.arn
}

output "execution_role_arn" {
  description = "ARN of the execution IAM role."
  value       = aws_iam_role.execution.arn
}

output "log_group_name" {
  description = "CloudWatch log group used for worker logs."
  value       = aws_cloudwatch_log_group.this.name
}
