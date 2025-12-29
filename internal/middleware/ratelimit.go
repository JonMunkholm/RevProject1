package middleware

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// IPRateLimiter tracks rate limits per IP address using token bucket algorithm.
type IPRateLimiter struct {
	mu       sync.Mutex
	limiters map[string]*rateLimiterEntry
	rate     rate.Limit
	burst    int
	cleanup  time.Duration
}

type rateLimiterEntry struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// NewIPRateLimiter creates a new rate limiter that allows requestsPerMinute
// requests on average, with burst capacity for short spikes.
func NewIPRateLimiter(requestsPerMinute int, burst int) *IPRateLimiter {
	rl := &IPRateLimiter{
		limiters: make(map[string]*rateLimiterEntry),
		rate:     rate.Limit(float64(requestsPerMinute) / 60.0),
		burst:    burst,
		cleanup:  5 * time.Minute,
	}
	go rl.cleanupLoop()
	return rl
}

// GetLimiter returns the rate limiter for the given IP, creating one if needed.
func (rl *IPRateLimiter) GetLimiter(ip string) *rate.Limiter {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	entry, exists := rl.limiters[ip]
	if !exists {
		limiter := rate.NewLimiter(rl.rate, rl.burst)
		rl.limiters[ip] = &rateLimiterEntry{limiter: limiter, lastSeen: time.Now()}
		return limiter
	}
	entry.lastSeen = time.Now()
	return entry.limiter
}

// cleanupLoop periodically removes stale IP entries to prevent memory leaks.
func (rl *IPRateLimiter) cleanupLoop() {
	ticker := time.NewTicker(rl.cleanup)
	for range ticker.C {
		rl.mu.Lock()
		cutoff := time.Now().Add(-rl.cleanup)
		for ip, entry := range rl.limiters {
			if entry.lastSeen.Before(cutoff) {
				delete(rl.limiters, ip)
			}
		}
		rl.mu.Unlock()
	}
}

// RateLimit returns middleware that enforces rate limiting per IP.
func RateLimit(limiter *IPRateLimiter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := ExtractIP(r)
			if !limiter.GetLimiter(ip).Allow() {
				LogSecurityEvent(SecurityEvent{
					Timestamp: time.Now(),
					EventType: EventRateLimitHit,
					IP:        ip,
					UserAgent: r.UserAgent(),
					Endpoint:  r.URL.Path,
					Reason:    "rate limit exceeded",
				})
				w.Header().Set("Retry-After", "60")
				http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// ExtractIP extracts the client IP from the request, checking proxy headers first.
func ExtractIP(r *http.Request) string {
	// Check X-Forwarded-For (first IP is client)
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		if idx := strings.Index(xff, ","); idx != -1 {
			xff = xff[:idx]
		}
		if ip := net.ParseIP(strings.TrimSpace(xff)); ip != nil {
			return ip.String()
		}
	}

	// Check X-Real-IP
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		if ip := net.ParseIP(strings.TrimSpace(xri)); ip != nil {
			return ip.String()
		}
	}

	// Fall back to RemoteAddr
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
