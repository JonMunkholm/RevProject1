package credentials

import "context"

// Resolver retrieves and manages stored credentials such as per-user LLM API keys.
type Resolver interface {
	Resolve(ctx context.Context, reference string) (string, error)
	Rotate(ctx context.Context, reference string) error
	Audit(ctx context.Context, reference string, metadata map[string]any)
}

type Logger interface {
	Info(ctx context.Context, msg string, attrs map[string]any)
	Warn(ctx context.Context, msg string, err error, attrs map[string]any)
}

type noopResolver struct{}

func (noopResolver) Resolve(context.Context, string) (string, error) { return "", nil }
func (noopResolver) Rotate(context.Context, string) error            { return nil }
func (noopResolver) Audit(context.Context, string, map[string]any)   {}

// NewNoopResolver returns a resolver that performs no operations.
func NewNoopResolver() Resolver { return noopResolver{} }

type noopLogger struct{}

func (noopLogger) Info(context.Context, string, map[string]any)        {}
func (noopLogger) Warn(context.Context, string, error, map[string]any) {}

func NewNoopLogger() Logger { return noopLogger{} }
