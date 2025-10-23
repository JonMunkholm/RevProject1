package auth

import (
	"context"
	"errors"
	"log"
	"net/http"
	"strings"

	"github.com/google/uuid"
)

type contextKey struct{}

var authContextKey = contextKey{}

// Session holds authenticated user metadata extracted from a JWT.
type Session struct {
	UserID       uuid.UUID
	CompanyID    uuid.UUID
	TenantID     uuid.UUID
	CurrentRole  Role
	Roles        map[uuid.UUID]Role
	Scopes       map[string]struct{}
	Capabilities Capabilities
}

func (s Session) RoleFor(companyID uuid.UUID) (Role, bool) {
	if s.Roles == nil {
		return RoleUnknown, false
	}
	role, ok := s.Roles[companyID]
	return role, ok
}

func (s Session) HasScope(scope string) bool {
	if scope == "" || len(s.Scopes) == 0 {
		return false
	}
	normalized := strings.ToLower(strings.TrimSpace(scope))
	if normalized == "" {
		return false
	}
	_, ok := s.Scopes[normalized]
	return ok
}

// SessionFromContext retrieves the Session stored by JWTMiddleware.
func SessionFromContext(ctx context.Context) (Session, bool) {
	if ctx == nil {
		return Session{}, false
	}
	session, ok := ctx.Value(authContextKey).(Session)
	return session, ok
}

const (
	scopeGuardrailsAll          = "guardrails.all"
	scopeGuardrailsSuperseded   = "guardrails.include_superseded"
	scopeGuardrailsInterpretive = "guardrails.include_interpretive"
	scopeGuardrailsInternal     = "guardrails.include_internal"
	scopeGuardrailsAsOf         = "guardrails.use_asof"
)

// JWTMiddleware validates access tokens and enforces authentication for protected routes.
func JWTMiddleware(secret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if secret == "" {
				RespondWithError(w, http.StatusInternalServerError, "authentication not configured", errors.New("jwt secret missing"))
				return
			}

			token, err := tokenFromRequest(r)
			if err != nil {
				handleUnauthorized(w, r, http.StatusUnauthorized, "authentication required", err)
				return
			}

			claims, err := ValidateJWT(token, secret)
			if err != nil {
				handleUnauthorized(w, r, http.StatusUnauthorized, "invalid or expired token", err)
				return
			}

			userID, err := uuid.Parse(claims.Subject)
			if err != nil {
				handleUnauthorized(w, r, http.StatusUnauthorized, "invalid token subject", err)
				return
			}

			companyID, err := uuid.Parse(claims.CompanyID)
			if err != nil {
				handleUnauthorized(w, r, http.StatusUnauthorized, "invalid token company", err)
				return
			}

			tenantID := companyID
			if rawTenant := strings.TrimSpace(claims.TenantID); rawTenant != "" {
				if parsedTenant, parseErr := uuid.Parse(rawTenant); parseErr == nil {
					tenantID = parsedTenant
				} else {
					log.Printf("auth: skipping invalid tenant id %q for user %s", rawTenant, userID)
				}
			}

			roleMap := make(map[uuid.UUID]Role, len(claims.Roles))
			for company, value := range claims.Roles {
				companyUUID, parseErr := uuid.Parse(company)
				if parseErr != nil {
					log.Printf("auth: skipping invalid company id %q in token for user %s", company, userID)
					continue
				}
				roleMap[companyUUID] = ParseRole(value)
			}

			currentRole := ParseRole(claims.CurrentRole)
			if raw := strings.TrimSpace(claims.CurrentRole); raw == "" {
				if role, ok := roleMap[companyID]; ok {
					currentRole = role
				} else {
					currentRole = RoleViewer
				}
			}

			scopeSet := make(map[string]struct{})
			addScope := func(scope string) {
				normalized := strings.ToLower(strings.TrimSpace(scope))
				if normalized == "" {
					return
				}
				scopeSet[normalized] = struct{}{}
			}
			for _, scope := range claims.Scopes {
				addScope(scope)
			}
			for _, scope := range strings.Fields(claims.Scope) {
				addScope(scope)
			}

			session := Session{
				UserID:       userID,
				CompanyID:    companyID,
				TenantID:     tenantID,
				CurrentRole:  currentRole,
				Roles:        roleMap,
				Scopes:       scopeSet,
				Capabilities: capabilitiesForRole(currentRole),
			}

			if _, ok := scopeSet[scopeGuardrailsAll]; ok {
				session.Capabilities.CanIncludeSuperseded = true
				session.Capabilities.CanIncludeInterpretive = true
				session.Capabilities.CanIncludeInternal = true
				session.Capabilities.CanUseAsOf = true
			} else {
				if _, ok := scopeSet[scopeGuardrailsSuperseded]; ok {
					session.Capabilities.CanIncludeSuperseded = true
				}
				if _, ok := scopeSet[scopeGuardrailsInterpretive]; ok {
					session.Capabilities.CanIncludeInterpretive = true
				}
				if _, ok := scopeSet[scopeGuardrailsInternal]; ok {
					session.Capabilities.CanIncludeInternal = true
				}
				if _, ok := scopeSet[scopeGuardrailsAsOf]; ok {
					session.Capabilities.CanUseAsOf = true
				}
			}

			ctx := context.WithValue(r.Context(), authContextKey, session)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func tokenFromRequest(r *http.Request) (string, error) {
	if r == nil {
		return "", errors.New("request missing")
	}

	if cookie, err := r.Cookie("access_token"); err == nil && cookie.Value != "" {
		return cookie.Value, nil
	}

	token, err := GetBearerToken(r.Header)
	if err != nil {
		return "", err
	}
	return token, nil
}

func handleUnauthorized(w http.ResponseWriter, r *http.Request, status int, msg string, err error) {
	if shouldRedirectToLogin(r) {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	RespondWithError(w, status, msg, err)
}

func shouldRedirectToLogin(r *http.Request) bool {
	if r == nil {
		return false
	}

	if strings.EqualFold(r.Header.Get("HX-Request"), "true") {
		return false
	}

	if strings.EqualFold(r.Header.Get("X-Requested-With"), "XMLHttpRequest") {
		return false
	}

	accept := r.Header.Get("Accept")
	if strings.Contains(accept, "text/html") {
		return true
	}

	if accept == "" && r.Method == http.MethodGet {
		return true
	}

	return false
}

func capabilitiesForRole(role Role) Capabilities {
	caps := Capabilities{}

	if role.Meets(RoleViewer) {
		caps.CanViewCompanySettings = true
		caps.CanViewProviderCredentials = true
	}

	if role.Meets(RoleMember) {
		caps.CanManagePersonalCredentials = true
	}

	if role.Meets(RoleAdmin) {
		caps.CanManageCompanyCredentials = true
		caps.CanManagePersonalCredentials = true
		caps.CanUseAsOf = true
		caps.CanIncludeSuperseded = true
		caps.CanIncludeInterpretive = true
		caps.CanIncludeInternal = true
	}

	return caps
}
