package auth

import "context"

type Capabilities struct {
	CanViewCompanySettings       bool
	CanViewProviderCredentials   bool
	CanManagePersonalCredentials bool
	CanManageCompanyCredentials  bool
}

type capabilityKey struct{}

var authCapabilitiesKey = capabilityKey{}

func withCapabilities(ctx context.Context, caps Capabilities) context.Context {
	return context.WithValue(ctx, authCapabilitiesKey, caps)
}

func CapabilitiesFromContext(ctx context.Context) (Capabilities, bool) {
	if ctx == nil {
		return Capabilities{}, false
	}
	caps, ok := ctx.Value(authCapabilitiesKey).(Capabilities)
	return caps, ok
}
