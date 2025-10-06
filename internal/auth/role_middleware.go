package auth

import (
	"context"
	"errors"
	"log"
	"net/http"
)

var (
	errSessionMissing   = errors.New("session missing")
	errInsufficientRole = errors.New("insufficient role")
)

func RequireCompanyRole(min Role) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			session, ok := SessionFromContext(r.Context())
			if !ok {
				log.Printf("auth: missing session for path=%s", r.URL.Path)
				RespondWithError(w, http.StatusUnauthorized, "authentication required", errSessionMissing)
				return
			}

			activeRole := session.CurrentRole
			if activeRole == RoleUnknown {
				if role, ok := session.RoleFor(session.CompanyID); ok {
					activeRole = role
				} else {
					activeRole = RoleViewer
				}
			}

			allowed := activeRole.Meets(min)
			log.Printf("auth: role decision user=%s company=%s role=%s required=%s allowed=%t path=%s flow=db->session->context->templates", session.UserID, session.CompanyID, activeRole, min, allowed, r.URL.Path)

			if !allowed {
				RespondWithError(w, http.StatusForbidden, "insufficient permissions", errInsufficientRole)
				return
			}

			caps := capabilitiesForRole(activeRole)
			session.CurrentRole = activeRole
			session.Capabilities = caps

			ctx := context.WithValue(r.Context(), authContextKey, session)
			ctx = withCapabilities(ctx, caps)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
