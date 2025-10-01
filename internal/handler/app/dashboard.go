package handler

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/JonMunkholm/RevProject1/internal/auth"
	"github.com/JonMunkholm/RevProject1/internal/database"
	"github.com/google/uuid"
)

type Dashboard struct {
	DB *database.Queries
}

type dashboardMetricsResponse struct {
	TotalUsers             int64 `json:"totalUsers"`
	ActiveUsers            int64 `json:"activeUsers"`
	TotalCustomers         int64 `json:"totalCustomers"`
	ActiveCustomers        int64 `json:"activeCustomers"`
	TotalContracts         int64 `json:"totalContracts"`
	FinalizedContracts     int64 `json:"finalizedContracts"`
	TotalProducts          int64 `json:"totalProducts"`
	ActiveProducts         int64 `json:"activeProducts"`
	TotalBundles           int64 `json:"totalBundles"`
	ActiveBundles          int64 `json:"activeBundles"`
	PerformanceObligations int64 `json:"performanceObligations"`
}

type dashboardContractResponse struct {
	ID           string    `json:"id"`
	CustomerName string    `json:"customerName"`
	StartDate    time.Time `json:"startDate"`
	EndDate      time.Time `json:"endDate"`
	IsFinal      bool      `json:"isFinal"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

type dashboardSummaryResponse struct {
	Metrics         dashboardMetricsResponse    `json:"metrics"`
	RecentContracts []dashboardContractResponse `json:"recentContracts"`
}

func (d *Dashboard) Summary(w http.ResponseWriter, r *http.Request) {
	if d == nil || d.DB == nil {
		RespondWithError(w, http.StatusInternalServerError, "dashboard unavailable", errors.New("database not initialized"))
		return
	}

	session, ok := auth.SessionFromContext(r.Context())
	if !ok {
		RespondWithError(w, http.StatusUnauthorized, "authentication required", errors.New("session missing"))
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	metrics, err := d.collectMetrics(ctx, session.CompanyID)
	if err != nil {
		RespondWithError(w, http.StatusInternalServerError, "failed to load metrics", err)
		return
	}

	contracts, err := d.DB.DashboardRecentContracts(ctx, session.CompanyID, 5)
	if err != nil {
		RespondWithError(w, http.StatusInternalServerError, "failed to load recent contracts", err)
		return
	}

	response := dashboardSummaryResponse{Metrics: metrics}
	for _, c := range contracts {
		response.RecentContracts = append(response.RecentContracts, dashboardContractResponse{
			ID:           c.ID.String(),
			CustomerName: c.CustomerName,
			StartDate:    c.StartDate,
			EndDate:      c.EndDate,
			IsFinal:      c.IsFinal,
			UpdatedAt:    c.UpdatedAt,
		})
	}

	RespondWithJSON(w, http.StatusOK, response)
}

func (d *Dashboard) collectMetrics(ctx context.Context, companyID uuid.UUID) (dashboardMetricsResponse, error) {
	var metrics dashboardMetricsResponse

	var err error
	if metrics.TotalUsers, err = d.DB.CountCompanyUsers(ctx, companyID); err != nil {
		return dashboardMetricsResponse{}, err
	}
	if metrics.ActiveUsers, err = d.DB.CountActiveCompanyUsers(ctx, companyID); err != nil {
		return dashboardMetricsResponse{}, err
	}
	if metrics.TotalCustomers, err = d.DB.CountCompanyCustomers(ctx, companyID); err != nil {
		return dashboardMetricsResponse{}, err
	}
	if metrics.ActiveCustomers, err = d.DB.CountActiveCompanyCustomers(ctx, companyID); err != nil {
		return dashboardMetricsResponse{}, err
	}
	if metrics.TotalContracts, err = d.DB.CountCompanyContracts(ctx, companyID); err != nil {
		return dashboardMetricsResponse{}, err
	}
	if metrics.FinalizedContracts, err = d.DB.CountCompanyFinalContracts(ctx, companyID); err != nil {
		return dashboardMetricsResponse{}, err
	}
	if metrics.TotalProducts, err = d.DB.CountCompanyProducts(ctx, companyID); err != nil {
		return dashboardMetricsResponse{}, err
	}
	if metrics.ActiveProducts, err = d.DB.CountActiveCompanyProducts(ctx, companyID); err != nil {
		return dashboardMetricsResponse{}, err
	}
	if metrics.TotalBundles, err = d.DB.CountCompanyBundles(ctx, companyID); err != nil {
		return dashboardMetricsResponse{}, err
	}
	if metrics.ActiveBundles, err = d.DB.CountActiveCompanyBundles(ctx, companyID); err != nil {
		return dashboardMetricsResponse{}, err
	}
	if metrics.PerformanceObligations, err = d.DB.CountCompanyPerformanceObligations(ctx, companyID); err != nil {
		return dashboardMetricsResponse{}, err
	}

	return metrics, nil
}
