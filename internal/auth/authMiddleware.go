package auth

import (
	"context"
	"errors"
	"net/http"

	"github.com/google/uuid"
)

type contextKey struct{}

var authContextKey = contextKey{}

// Session holds authenticated user metadata extracted from a JWT.
type Session struct {
	UserID    uuid.UUID
	CompanyID uuid.UUID
	Role      string
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
				RespondWithError(w, http.StatusUnauthorized, "authentication required", err)
				return
			}

			claims, err := ValidateJWT(token, secret)
			if err != nil {
				RespondWithError(w, http.StatusUnauthorized, "invalid or expired token", err)
				return
			}

			userID, err := uuid.Parse(claims.Subject)
			if err != nil {
				RespondWithError(w, http.StatusUnauthorized, "invalid token subject", err)
				return
			}

			companyID, err := uuid.Parse(claims.CompanyID)
			if err != nil {
				RespondWithError(w, http.StatusUnauthorized, "invalid token company", err)
				return
			}

			session := Session{
				UserID:    userID,
				CompanyID: companyID,
				Role:      claims.Role,
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
