// Package config provides centralized configuration for the application.
package config

import (
	"os"
	"strconv"
	"time"
)

// Handler timeouts
var (
	// DefaultHandlerTimeout is the standard timeout for HTTP handlers.
	DefaultHandlerTimeout = 10 * time.Second

	// LongHandlerTimeout is used for operations that may take longer (e.g., AI completions).
	LongHandlerTimeout = 60 * time.Second

	// ShortTimeout is used for quick operations like event recording.
	ShortTimeout = 5 * time.Second
)

// Pagination defaults
var (
	DefaultConversationLimit    int32 = 20
	DefaultJobLimit             int32 = 20
	DefaultCredentialLimit      int32 = 20
	DefaultCredentialEventLimit int32 = 20
)

// UI constants
var (
	ChatPreviewCharacterLimit = 80
)

// Metadata keys
const (
	MetadataKeyCredentialSuffix = "key_suffix"
)

// Auth token TTLs
var (
	DefaultAccessTokenTTL  = 15 * time.Minute
	DefaultRefreshTokenTTL = 60 * 24 * time.Hour // 60 days
	ForceSecureCookies     = false
)

// Rate limiting
var (
	RateLimitLogin    = 5  // requests per minute
	RateLimitRegister = 3  // requests per minute
	RateLimitRefresh  = 10 // requests per minute
	RateLimitEnabled  = true
)

// Worker settings
var (
	SQSWaitTimeSeconds int64 = 20
	SQSMaxMessages     int32 = 5
	VisibilityGrace          = 15 * time.Second
	JobProcessTimeout        = 60 * time.Second
)

// Database pool settings
var (
	DBMaxOpenConns    = 25
	DBMaxIdleConns    = 5
	DBConnMaxLifetime = 5 * time.Minute
)

// AI and retrieval settings
var (
	HTTPClientTimeout       = 15 * time.Second
	DefaultAIProvider       = "openai"
	DefaultAIModel          = "text-embedding-3-large"
	DefaultOpenAIBaseURL    = "https://api.openai.com/v1"
	CatalogCacheTTL         = 5 * time.Minute
	SearchDefaultLimit      = 5
	SearchMaxLimit          = 25
	CompareDefaultMaxResults = 5
)

// Server settings
var (
	DefaultServerPort  = ":8080"
	ShutdownTimeout    = 10 * time.Second
	WorkerMetricsPort  = ":2112"
	CloudWatchNamespace = "EGRA/EmbeddingWorker"
)

// Auth paths
const (
	RefreshCookiePath = "/auth/refresh"
)

// Load reads configuration from environment variables, applying defaults where not set.
func Load() {
	// Handler timeouts
	if v := os.Getenv("HANDLER_TIMEOUT_SECONDS"); v != "" {
		if seconds, err := strconv.Atoi(v); err == nil && seconds > 0 {
			DefaultHandlerTimeout = time.Duration(seconds) * time.Second
		}
	}

	// Pagination limits
	if v := os.Getenv("DEFAULT_PAGE_LIMIT"); v != "" {
		if limit, err := strconv.Atoi(v); err == nil && limit > 0 {
			limit32 := int32(limit)
			DefaultConversationLimit = limit32
			DefaultJobLimit = limit32
			DefaultCredentialLimit = limit32
			DefaultCredentialEventLimit = limit32
		}
	}

	// Auth token TTLs
	if v := os.Getenv("ACCESS_TOKEN_TTL_MINUTES"); v != "" {
		if minutes, err := strconv.Atoi(v); err == nil && minutes > 0 {
			DefaultAccessTokenTTL = time.Duration(minutes) * time.Minute
		}
	}
	if v := os.Getenv("REFRESH_TOKEN_TTL_HOURS"); v != "" {
		if hours, err := strconv.Atoi(v); err == nil && hours > 0 {
			DefaultRefreshTokenTTL = time.Duration(hours) * time.Hour
		}
	}
	if v := os.Getenv("FORCE_SECURE_COOKIES"); v != "" {
		if parsed, err := strconv.ParseBool(v); err == nil {
			ForceSecureCookies = parsed
		}
	}

	// Rate limiting
	if v := os.Getenv("RATE_LIMIT_ENABLED"); v != "" {
		if parsed, err := strconv.ParseBool(v); err == nil {
			RateLimitEnabled = parsed
		}
	}

	// Worker settings
	if v := os.Getenv("SQS_WAIT_TIME_SECONDS"); v != "" {
		if seconds, err := strconv.ParseInt(v, 10, 64); err == nil && seconds > 0 {
			SQSWaitTimeSeconds = seconds
		}
	}
	if v := os.Getenv("SQS_MAX_MESSAGES"); v != "" {
		if max, err := strconv.Atoi(v); err == nil && max > 0 {
			SQSMaxMessages = int32(max)
		}
	}
	if v := os.Getenv("JOB_PROCESS_TIMEOUT_SECONDS"); v != "" {
		if seconds, err := strconv.Atoi(v); err == nil && seconds > 0 {
			JobProcessTimeout = time.Duration(seconds) * time.Second
		}
	}

	// Database pool settings
	if v := os.Getenv("DB_MAX_OPEN_CONNS"); v != "" {
		if conns, err := strconv.Atoi(v); err == nil && conns > 0 {
			DBMaxOpenConns = conns
		}
	}
	if v := os.Getenv("DB_MAX_IDLE_CONNS"); v != "" {
		if conns, err := strconv.Atoi(v); err == nil && conns > 0 {
			DBMaxIdleConns = conns
		}
	}
	if v := os.Getenv("DB_CONN_MAX_LIFETIME_MINUTES"); v != "" {
		if minutes, err := strconv.Atoi(v); err == nil && minutes > 0 {
			DBConnMaxLifetime = time.Duration(minutes) * time.Minute
		}
	}
}
