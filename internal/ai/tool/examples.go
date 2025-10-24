package tool

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/JonMunkholm/RevProject1/internal/contextutil"
	"github.com/JonMunkholm/RevProject1/internal/database"
)

// WorkspaceTools groups multiple tool definitions for convenience.
type WorkspaceTools struct {
	ListCustomers  ListCustomersTool
	CustomerLookup FetchCustomerTool
	TicketCreation CreateTicketTool
}

// CustomerStore exposes customer lookups needed by FetchCustomerTool.
type CustomerStore interface {
	GetAllCustomersCompany(ctx context.Context, companyID uuid.UUID) ([]database.Customer, error)
	GetCustomer(ctx context.Context, params database.GetCustomerParams) (database.Customer, error)
	GetCustomerByName(ctx context.Context, params database.GetCustomerByNameParams) (database.Customer, error)
}

// ListCustomersTool enumerates customer summaries for the active company.
type ListCustomersTool struct {
	Store CustomerStore
}

func (ListCustomersTool) Name() string { return "list_customers" }
func (ListCustomersTool) Summary() string {
	return "List customers for the current account with optional name filtering"
}
func (ListCustomersTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"query": map[string]any{
				"type":        "string",
				"description": "Optional case-insensitive substring to match customer names",
			},
			"limit": map[string]any{
				"type":        "integer",
				"description": "Maximum number of customers to return (default 20, max 100)",
				"minimum":     1,
				"maximum":     100,
			},
		},
	}
}
func (t ListCustomersTool) NewHandler() Handler {
	return listCustomersHandler{store: t.Store}
}

type listCustomersHandler struct {
	store CustomerStore
}

func (h listCustomersHandler) Invoke(ctx context.Context, input map[string]any) (Result, error) {
	if h.store == nil {
		return Result{}, errors.New("customer list unavailable")
	}

	session, ok := contextutil.Session(ctx)
	if !ok || session.CompanyID == uuid.Nil {
		return Result{}, errors.New("authentication context missing for customer list")
	}

	query := ""
	if rawQuery, ok := input["query"].(string); ok {
		query = strings.TrimSpace(rawQuery)
	}

	limit := 20
	if rawLimit, ok := input["limit"]; ok {
		switch typed := rawLimit.(type) {
		case float64:
			limit = int(math.Round(typed))
		case json.Number:
			if parsed, err := typed.Int64(); err == nil {
				limit = int(parsed)
			}
		}
	}
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	ctxList, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	allCustomers, err := h.store.GetAllCustomersCompany(ctxList, session.CompanyID)
	if err != nil {
		return Result{}, fmt.Errorf("list customers failed: %w", err)
	}

	sort.Slice(allCustomers, func(i, j int) bool {
		return strings.ToLower(allCustomers[i].CustomerName) < strings.ToLower(allCustomers[j].CustomerName)
	})

	needle := strings.ToLower(query)
	summaries := make([]map[string]any, 0, limit)
	for _, c := range allCustomers {
		if needle != "" && !strings.Contains(strings.ToLower(c.CustomerName), needle) {
			continue
		}
		status := "inactive"
		if c.IsActive {
			status = "active"
		}
		summaries = append(summaries, map[string]any{
			"id":     c.ID.String(),
			"name":   c.CustomerName,
			"status": status,
		})
		if len(summaries) >= limit {
			break
		}
	}

	log.Printf("ai.tool.list_customers: company=%s user=%s count=%d query=%q", session.CompanyID, session.UserID, len(summaries), query)

	return Result{Output: map[string]any{"customers": summaries}}, nil
}

// FetchCustomerTool retrieves customer details for the active company.
type FetchCustomerTool struct {
	Store CustomerStore
}

func (FetchCustomerTool) Name() string    { return "fetch_customer" }
func (FetchCustomerTool) Summary() string { return "Retrieve a customer by unique identifier or name" }
func (FetchCustomerTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"customer_id": map[string]any{
				"type":        "string",
				"description": "Unique identifier of the customer",
			},
			"name": map[string]any{
				"type":        "string",
				"description": "Case-insensitive customer name match",
			},
		},
		"anyOf": []any{
			map[string]any{"required": []string{"customer_id"}},
			map[string]any{"required": []string{"name"}},
		},
	}
}
func (t FetchCustomerTool) NewHandler() Handler {
	return fetchCustomerHandler{
		store: t.Store,
	}
}

type fetchCustomerHandler struct {
	store CustomerStore
}

func (h fetchCustomerHandler) Invoke(ctx context.Context, input map[string]any) (Result, error) {
	if h.store == nil {
		return Result{}, errors.New("customer lookup unavailable")
	}

	session, ok := contextutil.Session(ctx)
	if !ok || session.CompanyID == uuid.Nil {
		return Result{}, errors.New("authentication context missing for customer lookup")
	}

	ctxLookup, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	var (
		record database.Customer
		err    error
	)

	if rawID, ok := input["customer_id"]; ok {
		customerID, parseErr := parseUUIDString(rawID)
		if parseErr != nil {
			return Result{}, parseErr
		}
		record, err = h.store.GetCustomer(ctxLookup, database.GetCustomerParams{
			ID:        customerID,
			CompanyID: session.CompanyID,
		})
	} else if rawName, ok := input["name"]; ok {
		name := strings.TrimSpace(fmt.Sprintf("%v", rawName))
		if name == "" {
			return Result{}, errors.New("name cannot be empty")
		}
		record, err = h.store.GetCustomerByName(ctxLookup, database.GetCustomerByNameParams{
			CompanyID:    session.CompanyID,
			CustomerName: name,
		})
	} else {
		return Result{}, errors.New("provide customer_id or name")
	}

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Result{}, errors.New("customer not found")
		}
		return Result{}, fmt.Errorf("lookup failed: %w", err)
	}

	status := "inactive"
	if record.IsActive {
		status = "active"
	}

	log.Printf("ai.tool.fetch_customer: company=%s user=%s customer=%s status=%s", session.CompanyID, session.UserID, record.ID, status)

	return Result{
		Output: map[string]any{
			"customer": map[string]any{
				"id":     record.ID.String(),
				"name":   record.CustomerName,
				"tier":   "",
				"status": status,
			},
		},
	}, nil
}

func parseUUIDString(value any) (uuid.UUID, error) {
	switch v := value.(type) {
	case string:
		return parseUUIDFromString(v)
	case json.Number:
		return parseUUIDFromString(v.String())
	default:
		if s, ok := value.(fmt.Stringer); ok {
			return parseUUIDFromString(s.String())
		}
	}
	return uuid.Nil, errors.New("customer_id is required")
}

func parseUUIDFromString(raw string) (uuid.UUID, error) {
	str := strings.TrimSpace(raw)
	if str == "" {
		return uuid.Nil, errors.New("customer_id is required")
	}
	parsed, err := uuid.Parse(str)
	if err != nil {
		return uuid.Nil, fmt.Errorf("invalid customer_id %q", str)
	}
	return parsed, nil
}

// CreateTicketTool demonstrates a mutation tool.
type CreateTicketTool struct{}

func (CreateTicketTool) Name() string { return "create_ticket" }
func (CreateTicketTool) Summary() string {
	return "Open a new support ticket with subject and description"
}
func (CreateTicketTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"subject": map[string]any{
				"type":        "string",
				"description": "Short summary of the issue",
			},
			"description": map[string]any{
				"type":        "string",
				"description": "Detailed description of the issue",
			},
		},
		"required": []string{"subject", "description"},
	}
}
func (CreateTicketTool) NewHandler() Handler { return createTicketHandler{} }

type createTicketHandler struct{}

func (createTicketHandler) Invoke(ctx context.Context, input map[string]any) (Result, error) {
	// TODO: call ticketing system here. For now we just echo the payload.
	return Result{
		Output: map[string]any{
			"ticket": map[string]any{
				"id":          "TICKET-123",
				"subject":     input["subject"],
				"description": input["description"],
				"status":      "open",
			},
		},
	}, nil
}
