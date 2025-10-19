variable "replication_group_id" {
  description = "Identifier for the ElastiCache replication group."
  type        = string
}

variable "description" {
  description = "Description for the replication group."
  type        = string
  default     = "Redis cache for embedding retrieval"
}

variable "node_type" {
  description = "Instance class for cache nodes (e.g., cache.t3.small)."
  type        = string
  default     = "cache.t3.small"
}

variable "engine_version" {
  description = "Redis engine version."
  type        = string
  default     = "7.1"
}

variable "cluster_mode_enabled" {
  description = "If true, enable cluster-mode with shards/replicas."
  type        = bool
  default     = false
}

variable "number_cache_clusters" {
  description = "Number of cache clusters (when cluster mode disabled)."
  type        = number
  default     = 1
}

variable "num_node_groups" {
  description = "Number of node groups (shards) when cluster mode enabled."
  type        = number
  default     = 1
}

variable "replicas_per_node_group" {
  description = "Number of replicas per node group when cluster mode enabled."
  type        = number
  default     = 1
}

variable "automatic_failover_enabled" {
  description = "Enable automatic failover (requires replicas)."
  type        = bool
  default     = false
}

variable "multi_az_enabled" {
  description = "Whether Multi-AZ is enabled."
  type        = bool
  default     = false
}

variable "subnet_ids" {
  description = "Subnet IDs for the cache subnet group."
  type        = list(string)
}

variable "security_group_ids" {
  description = "Security group IDs allowing traffic to Redis."
  type        = list(string)
}

variable "parameter_group_name" {
  description = "Parameter group name to use."
  type        = string
  default     = "default.redis7"
}

variable "maintenance_window" {
  description = "Preferred maintenance window (e.g., sun:05:00-sun:06:00)."
  type        = string
  default     = ""
}

variable "snapshot_retention_limit" {
  description = "How many days to retain automatic snapshots (0 disables)."
  type        = number
  default     = 1
}

variable "auth_token" {
  description = "Optional Redis AUTH token."
  type        = string
  default     = ""
}

variable "at_rest_encryption_enabled" {
  description = "Enable encryption at rest."
  type        = bool
  default     = true
}

variable "transit_encryption_enabled" {
  description = "Enable encryption in transit."
  type        = bool
  default     = true
}

variable "tags" {
  description = "Tags to apply to all resources."
  type        = map(string)
  default     = {}
}
