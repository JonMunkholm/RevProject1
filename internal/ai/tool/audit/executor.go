package audit

import (
	"context"
	"time"

	"github.com/JonMunkholm/RevProject1/internal/ai/tool"
)

// InvocationStore records tool invocations for auditing.
type InvocationStore interface {
	InsertToolInvocation(ctx context.Context, params InvocationRecord) error
}

// InvocationRecord mirrors the ai_tool_invocations schema.
type InvocationRecord struct {
	UserID       *string
	ProviderID   string
	ToolName     string
	Status       string
	Request      map[string]any
	Response     map[string]any
	ErrorMessage *string
	CreatedAt    time.Time
}

// AuditingExecutor wraps an inner tool.Executor and forwards invocation metadata to a store.
type AuditingExecutor struct {
	inner *tool.Executor
	store InvocationStore
}

// NewAuditingExecutor constructs an executor wrapper. If store is nil the returned executor behaves like the inner executor.
func NewAuditingExecutor(inner *tool.Executor, store InvocationStore) *AuditingExecutor {
	return &AuditingExecutor{inner: inner, store: store}
}

// Descriptors proxies the inner executor descriptors.
func (a *AuditingExecutor) Descriptors() []tool.Descriptor {
	return a.inner.Descriptors()
}

// Execute runs the tool and optionally records the audit trail.
func (a *AuditingExecutor) Execute(ctx context.Context, invocation tool.Invocation, audit InvocationRecord) (tool.Result, error) {
	result, err := a.inner.Execute(ctx, invocation)
	if a.store != nil {
		record := audit
		record.ToolName = invocation.Name
		record.Request = invocation.Input
		record.CreatedAt = time.Now()
		if err != nil {
			record.Status = "error"
			message := err.Error()
			record.ErrorMessage = &message
		} else {
			record.Status = "success"
			record.Response = result.Output
		}
		if record.Status == "" {
			record.Status = "success"
		}
		_ = a.store.InsertToolInvocation(ctx, record)
	}
	return result, err
}
