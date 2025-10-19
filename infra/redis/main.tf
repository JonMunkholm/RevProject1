locals {
  subnet_group_name = "${var.replication_group_id}-subnet-group"
}

resource "aws_elasticache_subnet_group" "this" {
  name       = local.subnet_group_name
  subnet_ids = var.subnet_ids
  tags       = var.tags
}

resource "aws_elasticache_replication_group" "this" {
  replication_group_id          = var.replication_group_id
  replication_group_description = var.description
  engine                         = "redis"
  engine_version                 = var.engine_version
  node_type                      = var.node_type
  parameter_group_name           = var.parameter_group_name
  subnet_group_name              = aws_elasticache_subnet_group.this.name
  security_group_ids             = var.security_group_ids
  at_rest_encryption_enabled     = var.at_rest_encryption_enabled
  transit_encryption_enabled     = var.transit_encryption_enabled
  automatic_failover_enabled     = var.automatic_failover_enabled
  multi_az_enabled               = var.multi_az_enabled
  snapshot_retention_limit       = var.snapshot_retention_limit
  port                           = 6379
  apply_immediately              = true
  tags                           = var.tags

  number_cache_clusters = var.cluster_mode_enabled ? null : var.number_cache_clusters

  dynamic "cluster_mode" {
    for_each = var.cluster_mode_enabled ? [true] : []
    content {
      num_node_groups         = var.num_node_groups
      replicas_per_node_group = var.replicas_per_node_group
    }
  }

  dynamic "auth_token" {
    for_each = var.auth_token == "" ? [] : [var.auth_token]
    content  = auth_token.value
  }

  maintenance_window = var.maintenance_window != "" ? var.maintenance_window : null
}
