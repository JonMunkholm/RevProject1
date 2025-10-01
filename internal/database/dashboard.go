package database

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
)

// count executes a COUNT(*) style query and returns the resulting value.
func (q *Queries) count(ctx context.Context, query string, args ...interface{}) (int64, error) {
	if q == nil || q.db == nil {
		return 0, errors.New("database not configured")
	}
	row := q.db.QueryRowContext(ctx, query, args...)
	var total int64
	if err := row.Scan(&total); err != nil {
		return 0, err
	}
	return total, nil
}

func (q *Queries) CountCompanyUsers(ctx context.Context, companyID uuid.UUID) (int64, error) {
	return q.count(ctx, `SELECT COUNT(*) FROM users WHERE company_id = $1`, companyID)
}

func (q *Queries) CountActiveCompanyUsers(ctx context.Context, companyID uuid.UUID) (int64, error) {
	return q.count(ctx, `SELECT COUNT(*) FROM users WHERE company_id = $1 AND is_active`, companyID)
}

func (q *Queries) CountCompanyCustomers(ctx context.Context, companyID uuid.UUID) (int64, error) {
	return q.count(ctx, `SELECT COUNT(*) FROM customers WHERE company_id = $1`, companyID)
}

func (q *Queries) CountActiveCompanyCustomers(ctx context.Context, companyID uuid.UUID) (int64, error) {
	return q.count(ctx, `SELECT COUNT(*) FROM customers WHERE company_id = $1 AND is_active`, companyID)
}

func (q *Queries) CountCompanyContracts(ctx context.Context, companyID uuid.UUID) (int64, error) {
	return q.count(ctx, `SELECT COUNT(*) FROM contracts WHERE company_id = $1`, companyID)
}

func (q *Queries) CountCompanyFinalContracts(ctx context.Context, companyID uuid.UUID) (int64, error) {
	return q.count(ctx, `SELECT COUNT(*) FROM contracts WHERE company_id = $1 AND is_final`, companyID)
}

func (q *Queries) CountCompanyProducts(ctx context.Context, companyID uuid.UUID) (int64, error) {
	return q.count(ctx, `SELECT COUNT(*) FROM products WHERE company_id = $1`, companyID)
}

func (q *Queries) CountActiveCompanyProducts(ctx context.Context, companyID uuid.UUID) (int64, error) {
	return q.count(ctx, `SELECT COUNT(*) FROM products WHERE company_id = $1 AND is_active`, companyID)
}

func (q *Queries) CountCompanyBundles(ctx context.Context, companyID uuid.UUID) (int64, error) {
	return q.count(ctx, `SELECT COUNT(*) FROM bundles WHERE company_id = $1`, companyID)
}

func (q *Queries) CountActiveCompanyBundles(ctx context.Context, companyID uuid.UUID) (int64, error) {
	return q.count(ctx, `SELECT COUNT(*) FROM bundles WHERE company_id = $1 AND is_active`, companyID)
}

func (q *Queries) CountCompanyPerformanceObligations(ctx context.Context, companyID uuid.UUID) (int64, error) {
	return q.count(ctx, `
		SELECT COUNT(*)
		FROM performance_obligations po
		JOIN contracts c ON c.id = po.contract_id
		WHERE c.company_id = $1
	`, companyID)
}

type DashboardContractRow struct {
	ID           uuid.UUID
	CustomerName string
	StartDate    time.Time
	EndDate      time.Time
	IsFinal      bool
	UpdatedAt    time.Time
}

func (q *Queries) DashboardRecentContracts(ctx context.Context, companyID uuid.UUID, limit int32) ([]DashboardContractRow, error) {
	if limit <= 0 {
		limit = 5
	}
	rows, err := q.db.QueryContext(ctx, `
		SELECT c.id,
		       cu.customer_name,
		       c.start_date,
		       c.end_date,
		       c.is_final,
		       c.updated_at
		FROM contracts c
		JOIN customers cu ON c.customer_id = cu.id
		WHERE c.company_id = $1
		ORDER BY c.updated_at DESC
		LIMIT $2
	`, companyID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var contracts []DashboardContractRow
	for rows.Next() {
		var item DashboardContractRow
		if err := rows.Scan(&item.ID, &item.CustomerName, &item.StartDate, &item.EndDate, &item.IsFinal, &item.UpdatedAt); err != nil {
			return nil, err
		}
		contracts = append(contracts, item)
	}

	return contracts, rows.Err()
}
