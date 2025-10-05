package metrics

import (
	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// CredentialMetrics captures counters related to credential lifecycle events.
type CredentialMetrics interface {
	CredentialMissing(companyID uuid.UUID, providerID, scope string)
	CredentialTestFailure(companyID uuid.UUID, providerID string)
	CredentialResolveFailure(companyID uuid.UUID, providerID string)
}

type prometheusCredentialMetrics struct {
	missing        *prometheus.CounterVec
	testFailures   *prometheus.CounterVec
	resolveFailure *prometheus.CounterVec
}

// NewCredentialMetrics constructs a Prometheus-backed metrics recorder. If reg is nil
// the default Prometheus registerer is used.
func NewCredentialMetrics(reg prometheus.Registerer) CredentialMetrics {
	if reg == nil {
		reg = prometheus.DefaultRegisterer
	}
	return &prometheusCredentialMetrics{
		missing: promauto.With(reg).NewCounterVec(prometheus.CounterOpts{
			Namespace: "ai",
			Name:      "credentials_missing_total",
			Help:      "Number of times credential resolution failed for a company/provider scope.",
		}, []string{"company_id", "provider_id", "scope"}),
		testFailures: promauto.With(reg).NewCounterVec(prometheus.CounterOpts{
			Namespace: "ai",
			Name:      "credential_test_failures_total",
			Help:      "Number of times credential test operations failed.",
		}, []string{"company_id", "provider_id"}),
		resolveFailure: promauto.With(reg).NewCounterVec(prometheus.CounterOpts{
			Namespace: "ai",
			Name:      "credential_resolve_failures_total",
			Help:      "Number of times credential resolution returned an error.",
		}, []string{"company_id", "provider_id"}),
	}
}

func (m *prometheusCredentialMetrics) CredentialMissing(companyID uuid.UUID, providerID, scope string) {
	if m == nil {
		return
	}
	m.missing.WithLabelValues(companyID.String(), providerID, scope).Inc()
}

func (m *prometheusCredentialMetrics) CredentialTestFailure(companyID uuid.UUID, providerID string) {
	if m == nil {
		return
	}
	m.testFailures.WithLabelValues(companyID.String(), providerID).Inc()
}

func (m *prometheusCredentialMetrics) CredentialResolveFailure(companyID uuid.UUID, providerID string) {
	if m == nil {
		return
	}
	m.resolveFailure.WithLabelValues(companyID.String(), providerID).Inc()
}
