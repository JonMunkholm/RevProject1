package metrics

import (
	"testing"

	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestCredentialMetricsCounters(t *testing.T) {
	reg := prometheus.NewRegistry()
	cm := NewCredentialMetrics(reg)

	prom, ok := cm.(*prometheusCredentialMetrics)
	if !ok {
		t.Fatalf("expected prometheus-backed metrics implementation")
	}

	companyID := uuid.New()

	cm.CredentialMissing(companyID, "openai", "user")
	cm.CredentialMissing(companyID, "openai", "user")
	cm.CredentialTestFailure(companyID, "openai")
	cm.CredentialResolveFailure(companyID, "openai")
	cm.CredentialResolveFailure(companyID, "openai")

	if got := testutil.ToFloat64(prom.missing.WithLabelValues(companyID.String(), "openai", "user")); got != 2 {
		t.Fatalf("expected missing counter to be 2, got %v", got)
	}

	if got := testutil.ToFloat64(prom.testFailures.WithLabelValues(companyID.String(), "openai")); got != 1 {
		t.Fatalf("expected test failure counter to be 1, got %v", got)
	}

	if got := testutil.ToFloat64(prom.resolveFailure.WithLabelValues(companyID.String(), "openai")); got != 2 {
		t.Fatalf("expected resolve failure counter to be 2, got %v", got)
	}
}
