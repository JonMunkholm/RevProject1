package middleware

import (
	"log"
	"time"
)

// SecurityEventType identifies the type of security event.
type SecurityEventType string

const (
	EventRateLimitHit    SecurityEventType = "RATE_LIMIT_HIT"
	EventLoginFailure    SecurityEventType = "LOGIN_FAILURE"
	EventLoginSuccess    SecurityEventType = "LOGIN_SUCCESS"
	EventRegisterFailure SecurityEventType = "REGISTER_FAILURE"
	EventRefreshFailure  SecurityEventType = "REFRESH_FAILURE"
)

// SecurityEvent represents a security-relevant event for logging and monitoring.
type SecurityEvent struct {
	Timestamp time.Time
	EventType SecurityEventType
	IP        string
	UserAgent string
	Endpoint  string
	Email     string // optional, for login/register events
	Reason    string // failure reason
}

// LogSecurityEvent logs a security event in a structured format suitable for
// parsing by log aggregators (CloudWatch, Datadog, ELK, etc.).
func LogSecurityEvent(event SecurityEvent) {
	log.Printf("[SECURITY] type=%s ip=%s endpoint=%s email=%s reason=%s ua=%s",
		event.EventType,
		event.IP,
		event.Endpoint,
		event.Email,
		event.Reason,
		truncateUserAgent(event.UserAgent),
	)
}

// truncateUserAgent limits user agent length to prevent log flooding.
func truncateUserAgent(ua string) string {
	const maxLen = 100
	if len(ua) > maxLen {
		return ua[:maxLen] + "..."
	}
	return ua
}
