output "primary_endpoint_address" {
  description = "Primary endpoint for the Redis replication group."
  value       = aws_elasticache_replication_group.this.primary_endpoint_address
}

output "reader_endpoint_address" {
  description = "Reader endpoint for the Redis replication group."
  value       = aws_elasticache_replication_group.this.reader_endpoint_address
}

output "port" {
  description = "Port Redis listens on."
  value       = aws_elasticache_replication_group.this.port
}

output "replication_group_id" {
  description = "Replication group identifier."
  value       = aws_elasticache_replication_group.this.id
}

output "subnet_group_name" {
  description = "ElastiCache subnet group name."
  value       = aws_elasticache_subnet_group.this.name
}
