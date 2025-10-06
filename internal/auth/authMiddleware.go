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
	CurrentRole  Role
	Roles        map[uuid.UUID]Role
	Capabilities Capabilities
}

func (s Session) RoleFor(companyID uuid.UUID) (Role, bool) {
	if s.Roles == nil {
		return RoleUnknown, false
	}
	role, ok := s.Roles[companyID]
	return role, ok
}

// SessionFromContext retrieves the Session stored by JWTMiddleware.
func SessionFromContext(ctx context.Context) (Session, bool) {
	if ctx == nil {
		return Session{}, false
	}
	session, ok := ctx.Value(authContextKey).(Session)
	return session, ok
}

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

			session := Session{
				UserID:       userID,
				CompanyID:    companyID,
				CurrentRole:  currentRole,
				Roles:        roleMap,
				Capabilities: capabilitiesForRole(currentRole),
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
	}

	return caps
}
