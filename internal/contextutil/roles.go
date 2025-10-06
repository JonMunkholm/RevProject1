package contextutil

import (
	"context"

	"github.com/JonMunkholm/RevProject1/internal/auth"
)

// Session returns the current auth session stored in the context.
func Session(ctx context.Context) (auth.Session, bool) {
	return auth.SessionFromContext(ctx)
}

// CurrentRole returns the active company role for the request, if available.
func CurrentRole(ctx context.Context) (auth.Role, bool) {
	session, ok := Session(ctx)
	if !ok {
		return auth.RoleUnknown, false
	}
	return session.CurrentRole, true
}

// HasRole checks whether the current role meets or exceeds the provided minimum role.
func HasRole(ctx context.Context, min auth.Role) bool {
	session, ok := Session(ctx)
	if !ok {
		return false
	}
	return session.CurrentRole.Meets(min)
}

// capabilitiesFromContext attempts to load the capability flags from the request context.
func capabilitiesFromContext(ctx context.Context) (auth.Capabilities, bool) {
	if caps, ok := auth.CapabilitiesFromContext(ctx); ok {
		return caps, true
	}
	session, ok := Session(ctx)
	if !ok {
		return auth.Capabilities{}, false
	}
	return session.Capabilities, true
}

// CanViewProviderCredentials reports if the requester may view provider credential listings.
func CanViewProviderCredentials(ctx context.Context) bool {
	if caps, ok := capabilitiesFromContext(ctx); ok {
		return caps.CanViewProviderCredentials
	}
	return false
}

// CanManagePersonalCredentials reports if the requester may manage their own scoped credentials.
func CanManagePersonalCredentials(ctx context.Context) bool {
	if caps, ok := capabilitiesFromContext(ctx); ok {
		return caps.CanManagePersonalCredentials
	}
	return false
}

// CanManageCompanyCredentials reports if the requester may manage company-scoped credentials.
func CanManageCompanyCredentials(ctx context.Context) bool {
	if caps, ok := capabilitiesFromContext(ctx); ok {
		return caps.CanManageCompanyCredentials
	}
	return false
}

// CanViewCompanySettings indicates whether the requester can see the settings surface at all.
func CanViewCompanySettings(ctx context.Context) bool {
	if caps, ok := capabilitiesFromContext(ctx); ok {
		return caps.CanViewCompanySettings
	}
	return false
}
