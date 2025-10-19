# Redis ElastiCache Module

Terraform module provisioning an AWS ElastiCache for Redis replication group for
embedding/query caching. Supports single-node (dev) and cluster-enabled (prod) setups.

## Usage

```hcl
module "redis" {
  source = "../redis"

  replication_group_id = "rev-embedding-redis"
  node_type            = "cache.t3.small"
  number_cache_clusters = 1  # dev: single node
  subnet_ids           = var.private_subnet_ids
  security_group_ids   = [aws_security_group.cache.id]
  tags = {
    Project = "revproject1"
    Env     = var.environment
  }
}
```

For production with replicas:

```hcl
module "redis" {
  source = "../redis"

  replication_group_id      = "rev-embedding-redis"
  cluster_mode_enabled      = true
  num_node_groups           = 1
  replicas_per_node_group   = 1
  automatic_failover_enabled = true
  multi_az_enabled          = true
  subnet_ids                = var.private_subnet_ids
  security_group_ids        = [aws_security_group.cache.id]
  tags                      = var.tags
}
```

## Outputs

| Output                    | Description                              |
|---------------------------|------------------------------------------|
| `primary_endpoint_address`| Primary (write) endpoint DNS name.       |
| `reader_endpoint_address` | Read-only endpoint (if replicas enabled).|
| `port`                    | Redis port (default 6379).               |
| `replication_group_id`    | Replication group ID.                    |
| `subnet_group_name`       | Subnet group used for the cluster.       |

See `variables.tf` for additional options such as maintenance window,
encryption settings, auth token, and snapshot retention.
