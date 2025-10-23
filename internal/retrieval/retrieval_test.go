package retrieval

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestBuildGuardrailSQL_DefaultFilters(t *testing.T) {
	svc := &Service{guardrailsEnabled: true}

	sqlText, args, err := svc.buildGuardrailSQL("[1.0]", 5, QueryParams{})
	if err != nil {
		t.Fatalf("buildGuardrailSQL returned error: %v", err)
	}

	if !strings.Contains(sqlText, "p.superseded = false") {
		t.Fatalf("expected superseded filter, got %s", sqlText)
	}
	if !strings.Contains(sqlText, "p.source_type = 'authoritative'") {
		t.Fatalf("expected authoritative filter, got %s", sqlText)
	}
	if !strings.Contains(sqlText, "(p.tenant_id IS NULL)") {
		t.Fatalf("expected tenant null filter, got %s", sqlText)
	}

	if len(args) != 2 {
		t.Fatalf("expected 2 args (vector, limit); got %d", len(args))
	}
}

func TestBuildGuardrailSQL_InterpretiveAndInternal(t *testing.T) {
	svc := &Service{guardrailsEnabled: true}
	tenantID := uuid.New()

	sqlText, args, err := svc.buildGuardrailSQL("[1.0]", 5, QueryParams{
		IncludeInterpretive: true,
		IncludeInternal:     true,
		TenantID:            tenantID,
	})
	if err != nil {
		t.Fatalf("buildGuardrailSQL returned error: %v", err)
	}

	if !strings.Contains(sqlText, "p.source_type = ANY") {
		t.Fatalf("expected source type ANY filter, got %s", sqlText)
	}
	if !strings.Contains(sqlText, "p.tenant_id = $") {
		t.Fatalf("expected tenant equality filter, got %s", sqlText)
	}
	if len(args) != 4 {
		t.Fatalf("expected 4 args (vector, limit, source types, tenant); got %d", len(args))
	}
	if gotTenant, ok := args[len(args)-1].(uuid.UUID); !ok || gotTenant != tenantID {
		t.Fatalf("expected tenant id %s, got %#v", tenantID, args[len(args)-1])
	}
}

func TestBuildGuardrailSQL_AsOf(t *testing.T) {
	svc := &Service{guardrailsEnabled: true}
	asOf := time.Date(2024, 10, 23, 0, 0, 0, 0, time.UTC)

	sqlText, args, err := svc.buildGuardrailSQL("[1.0]", 5, QueryParams{
		AsOf: &asOf,
	})
	if err != nil {
		t.Fatalf("buildGuardrailSQL returned error: %v", err)
	}

	if !strings.Contains(sqlText, "effective_date") {
		t.Fatalf("expected effective_date filter, got %s", sqlText)
	}
	if len(args) != 3 {
		t.Fatalf("expected 3 args (vector, limit, asOf); got %d", len(args))
	}
	if gotAsOf, ok := args[2].(time.Time); !ok || !gotAsOf.Equal(asOf) {
		t.Fatalf("expected asOf arg to be %v, got %#v", asOf, args[2])
	}
}

func TestSearchReturnsTenantError(t *testing.T) {
	svc := &Service{
		db:                &sql.DB{},
		guardrailsEnabled: true,
	}

	_, err := svc.Search(context.Background(), QueryParams{
		Query:           "revenue",
		IncludeInternal: true,
	})
	if !errors.Is(err, ErrTenantRequired) {
		t.Fatalf("expected ErrTenantRequired, got %v", err)
	}
}
