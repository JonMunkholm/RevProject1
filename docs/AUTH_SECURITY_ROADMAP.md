# Auth Security Roadmap

Future security enhancements to consider for the authentication system. These build upon the current implementation which includes:

- Per-IP rate limiting on auth endpoints
- Security event logging for monitoring/alerting
- Secure cookie handling with proxy awareness

---

## 1. Distributed Rate Limiting (Redis-backed)

**Problem:** In-memory rate limiting doesn't work across multiple server instances.

**Solution:** Use Redis to store rate limit counters.

**Implementation:**
- Add Redis client (`github.com/redis/go-redis/v9`)
- Store counters with TTL: `INCR rate:{ip}:{endpoint}` with `EXPIRE`
- Use sliding window or token bucket algorithm

```go
func (rl *RedisRateLimiter) Allow(ip, endpoint string) bool {
    key := fmt.Sprintf("rate:%s:%s", ip, endpoint)
    count, _ := rl.client.Incr(ctx, key).Result()
    if count == 1 {
        rl.client.Expire(ctx, key, time.Minute)
    }
    return count <= rl.limit
}
```

**When to implement:** When scaling to multiple app servers without sticky sessions.

---

## 2. Per-Account Failed Attempt Tracking

**Problem:** Distributed attacks use many IPs to target one account, bypassing per-IP limits.

**Solution:** Track failed login attempts per email/username.

**Implementation:**
- DB table: `login_attempts (email, ip, timestamp, success)`
- Or Redis: `INCR login_fail:{email}` with TTL
- After N failures for same email, add delay or require verification

**Thresholds:**
- 5 failures: Add 30-second artificial delay
- 10 failures: Require CAPTCHA
- 20 failures: Temporary account lock (15 min)

```sql
CREATE TABLE login_attempts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email TEXT NOT NULL,
    ip INET NOT NULL,
    success BOOLEAN NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_login_attempts_email_recent
    ON login_attempts (email, created_at DESC);
```

**When to implement:** When targeted attacks become a concern.

---

## 3. Account Lockout

**Problem:** Persistent attackers can eventually guess passwords given enough time.

**Solution:** Temporarily disable login after N consecutive failures.

**Implementation:**
- DB column: `users.locked_until TIMESTAMP`
- After 10 consecutive failures: lock for 15 minutes
- After 20 failures: lock for 1 hour
- After 50 failures: lock until admin review

**Risks:**
- DOS vector: Attacker can lock anyone's account
- Mitigation: Only count from same IP range, or require CAPTCHA instead of lock

```go
func (l *Login) checkAccountLock(email string) error {
    user, err := l.DB.GetUserByEmail(ctx, email)
    if err != nil {
        return nil // Don't reveal account existence
    }
    if user.LockedUntil.Valid && time.Now().Before(user.LockedUntil.Time) {
        return fmt.Errorf("account locked until %s", user.LockedUntil.Time)
    }
    return nil
}
```

**When to implement:** After account recovery flow is in place.

---

## 4. Account Recovery Flow

**Problem:** Users get locked out (forgotten password, account lockout).

**Solution:** Password reset via email with secure token.

**Implementation:**
- DB table: `password_reset_tokens (user_id, token_hash, expires_at, used_at)`
- Generate 32-byte token, send link via email
- Token valid for 1 hour, single use
- Rate limit: 3 reset requests per hour per email

```sql
CREATE TABLE password_reset_tokens (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash TEXT NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    used_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

**Security considerations:**
- Don't reveal if email exists ("If account exists, email sent")
- Invalidate all sessions on password change
- Log password reset events

**When to implement:** Before enabling account lockout.

---

## 5. CAPTCHA Integration

**Problem:** Bots can automate login/registration attempts.

**Solution:** Require human verification after suspicious activity.

**Options:**
- reCAPTCHA v3 (invisible, score-based)
- hCaptcha (privacy-focused)
- Cloudflare Turnstile (free, privacy-focused)

**Trigger conditions:**
- After 3 failed login attempts from same IP
- After 5 failed attempts for same email
- All registration attempts (optional)

```go
type CaptchaVerifier interface {
    Verify(token string) (bool, error)
}

func (l *Login) SignIn(w http.ResponseWriter, r *http.Request) {
    // Check if CAPTCHA required for this IP/email
    if l.requiresCaptcha(r, email) {
        token := r.FormValue("captcha_token")
        if valid, _ := l.captcha.Verify(token); !valid {
            RespondWithError(w, http.StatusBadRequest, "CAPTCHA verification failed", nil)
            return
        }
    }
    // Continue with normal login...
}
```

**When to implement:** When bot traffic becomes significant.

---

## 6. Device Fingerprinting

**Problem:** Attackers rotate IPs using botnets/proxies.

**Solution:** Track browser/device characteristics.

**Fingerprint components:**
- User-Agent
- Accept-Language
- Screen resolution (via JS)
- Timezone
- Canvas fingerprint

**Use cases:**
- Detect same device across IPs
- Flag new device login (email notification)
- Require 2FA for unrecognized devices

```go
type DeviceFingerprint struct {
    UserAgent   string
    Language    string
    Timezone    string
    Screen      string
    CanvasHash  string
}

func (f *DeviceFingerprint) Hash() string {
    data := fmt.Sprintf("%s|%s|%s|%s|%s",
        f.UserAgent, f.Language, f.Timezone, f.Screen, f.CanvasHash)
    hash := sha256.Sum256([]byte(data))
    return hex.EncodeToString(hash[:])
}
```

**When to implement:** For high-security applications.

---

## 7. Geographic Anomaly Detection

**Problem:** Stolen credentials used from unexpected locations.

**Solution:** Flag logins from unusual geographic locations.

**Implementation:**
- GeoIP database (MaxMind GeoLite2)
- Track user's typical login locations
- Alert on login from new country
- Optionally block or require verification

```go
type GeoDetector struct {
    db *geoip2.Reader
}

func (g *GeoDetector) DetectAnomaly(userID uuid.UUID, ip string) bool {
    country, _ := g.db.Country(net.ParseIP(ip))
    knownCountries := g.getUserCountries(userID)
    return !contains(knownCountries, country.Country.IsoCode)
}
```

**When to implement:** When handling sensitive data or financial transactions.

---

## 8. Two-Factor Authentication (2FA)

**Problem:** Passwords alone are insufficient for high-value accounts.

**Solution:** Require second factor for authentication.

**Options:**
- TOTP (Google Authenticator, Authy)
- SMS codes (less secure, but familiar)
- Email codes
- Hardware keys (WebAuthn/FIDO2)

**Implementation:**
- DB: `users.totp_secret`, `users.totp_enabled`
- Recovery codes for account recovery
- Remember device for 30 days (optional)

```sql
ALTER TABLE users ADD COLUMN totp_secret TEXT;
ALTER TABLE users ADD COLUMN totp_enabled BOOLEAN NOT NULL DEFAULT FALSE;

CREATE TABLE totp_recovery_codes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    code_hash TEXT NOT NULL,
    used_at TIMESTAMPTZ
);
```

```go
import "github.com/pquerna/otp/totp"

func (l *Login) verifyTOTP(user database.User, code string) bool {
    if !user.TotpEnabled {
        return true // 2FA not enabled
    }
    return totp.Validate(code, user.TotpSecret)
}
```

**When to implement:** For admin accounts or sensitive operations.

---

## 9. Session Management Improvements

**Problem:** Limited visibility and control over active sessions.

**Features to add:**
- List all active sessions (devices, IPs, last activity)
- "Logout everywhere" button
- Session limit (max 5 concurrent sessions)
- Automatic logout after inactivity

**Implementation:**
- Extend `refresh_tokens` table with device info
- UI for session management in settings
- Background job to expire inactive sessions

```sql
ALTER TABLE refresh_tokens
    ADD COLUMN device_name TEXT,
    ADD COLUMN last_activity_at TIMESTAMPTZ;

-- Query for session list
SELECT device_name, issued_ip, user_agent, last_activity_at
FROM refresh_tokens
WHERE user_id = $1 AND revoked_at IS NULL AND expires_at > NOW()
ORDER BY last_activity_at DESC;
```

**When to implement:** When users request session visibility.

---

## 10. Security Event Alerting

**Problem:** Security events logged but not monitored.

**Solution:** Real-time alerts for suspicious activity.

**Alert triggers:**
- 10+ rate limit hits from same IP in 5 minutes
- 5+ failed logins for same account
- Login from new country
- Password reset requested

**Integration options:**
- Slack webhook
- PagerDuty
- Email digest
- CloudWatch Alarms

```go
type SecurityAlerter interface {
    Alert(event SecurityEvent) error
}

type SlackAlerter struct {
    webhookURL string
}

func (s *SlackAlerter) Alert(event SecurityEvent) error {
    if event.EventType == EventRateLimitHit {
        // Aggregate and alert if threshold exceeded
        if s.countRecent(event.IP, 5*time.Minute) > 10 {
            return s.sendSlackMessage(fmt.Sprintf(
                "Rate limit abuse detected from IP %s", event.IP))
        }
    }
    return nil
}
```

**When to implement:** For production monitoring.

---

## 11. Prometheus Metrics

**Problem:** No visibility into auth system health and abuse patterns.

**Metrics to expose:**
- `auth_login_total{status="success|failure"}`
- `auth_rate_limit_hits_total{endpoint="/auth/login"}`
- `auth_active_sessions`
- `auth_password_reset_requests_total`

**Implementation:**
- Add Prometheus client library
- Expose `/metrics` endpoint
- Create Grafana dashboard

```go
import "github.com/prometheus/client_golang/prometheus"

var loginCounter = prometheus.NewCounterVec(
    prometheus.CounterOpts{
        Name: "auth_login_total",
        Help: "Total login attempts",
    },
    []string{"status"},
)

func init() {
    prometheus.MustRegister(loginCounter)
}

// In SignIn handler
loginCounter.WithLabelValues("success").Inc()
```

**When to implement:** When setting up observability infrastructure.

---

## Priority Order

| Priority | Enhancement | Prerequisite |
|----------|-------------|--------------|
| 1 | Account Recovery | None |
| 2 | Per-Account Tracking | None |
| 3 | Account Lockout | Account Recovery |
| 4 | 2FA | Account Recovery |
| 5 | Alerting | None |
| 6 | Redis Rate Limiting | Horizontal scaling |
| 7 | CAPTCHA | Bot traffic increase |
| 8 | Session Management UI | User request |
| 9 | Geographic Detection | Sensitive data handling |
| 10 | Device Fingerprinting | High-security requirements |
| 11 | Prometheus Metrics | Observability infrastructure |

---

## Current Implementation Status

### Completed
- [x] Per-IP rate limiting (token bucket algorithm)
- [x] Security event logging
- [x] Secure cookie handling with proxy detection
- [x] JWT-based access tokens (HS256)
- [x] Refresh token rotation
- [x] bcrypt password hashing

### Configuration
```bash
# Rate limiting (defaults shown)
RATE_LIMIT_ENABLED=true
RATE_LIMIT_LOGIN=5      # per minute
RATE_LIMIT_REGISTER=3   # per minute
RATE_LIMIT_REFRESH=10   # per minute

# Cookie security
FORCE_SECURE_COOKIES=true  # Set for TLS-terminating proxies
```

### Log Format
Security events are logged in a structured format for parsing:
```
[SECURITY] type=LOGIN_FAILURE ip=192.168.1.100 endpoint=/auth/login email=test@example.com reason=invalid password ua=Mozilla/5.0...
```
