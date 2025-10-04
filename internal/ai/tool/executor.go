package tool

import (
	"context"
	"errors"
	"fmt"
)

// Executor wraps a registry and provides helpers for providers to surface and execute tools.
type Executor struct {
	registry *Registry
	logger   Logger
}

// Logger instruments tool operations.
type Logger interface {
	Info(ctx context.Context, msg string, attrs ...any)
	Error(ctx context.Context, msg string, err error, attrs ...any)
}

// NewExecutor constructs an Executor using the provided registry and logger.
func NewExecutor(registry *Registry, logger Logger) *Executor {
	if registry == nil {
		registry = NewRegistry()
	}
	if logger == nil {
		logger = noopLogger{}
	}
	return &Executor{registry: registry, logger: logger}
}

// Descriptors exposes tool metadata in a provider-agnostic format ready to be embedded into LLM payloads.
func (e *Executor) Descriptors() []Descriptor {
	return e.registry.Descriptors()
}

// Execute resolves the target tool and invokes it with the supplied input payload.
func (e *Executor) Execute(ctx context.Context, invocation Invocation) (Result, error) {
	if invocation.Name == "" {
		return Result{}, errors.New("ai: missing tool name")
	}

	tool, ok := e.registry.Get(invocation.Name)
	if !ok {
		err := fmt.Errorf("ai: tool %q not registered", invocation.Name)
		e.logger.Error(ctx, "ai: tool lookup failed", err, "tool", invocation.Name)
		return Result{}, err
	}

	handler := tool.NewHandler()
	if handler == nil {
		err := fmt.Errorf("ai: tool %q does not provide a handler", invocation.Name)
		e.logger.Error(ctx, "ai: tool handler missing", err, "tool", invocation.Name)
		return Result{}, err
	}

	result, err := handler.Invoke(ctx, invocation.Input)
	if err != nil {
		e.logger.Error(ctx, "ai: tool invocation failed", err, "tool", invocation.Name)
		return Result{}, err
	}

	e.logger.Info(ctx, "ai: tool invocation successful", "tool", invocation.Name)
	return result, nil
}

type noopLogger struct{}

func (noopLogger) Info(context.Context, string, ...any)         {}
func (noopLogger) Error(context.Context, string, error, ...any) {}
