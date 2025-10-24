package tool

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/JonMunkholm/RevProject1/internal/contextutil"
	"github.com/JonMunkholm/RevProject1/internal/database"
)

// WorkspaceTools groups multiple tool definitions for convenience.
type WorkspaceTools struct {
	CustomerLookup FetchCustomerTool
	TicketCreation CreateTicketTool
}

// CustomerStore exposes customer lookups needed by FetchCustomerTool.
type CustomerStore interface {
	GetCustomer(ctx context.Context, params database.GetCustomerParams) (database.Customer, error)
}

// FetchCustomerTool retrieves customer details for the active company.
type FetchCustomerTool struct {
	Store CustomerStore
}

func (FetchCustomerTool) Name() string    { return "fetch_customer" }
func (FetchCustomerTool) Summary() string { return "Retrieve customer profile details by ID" }
func (FetchCustomerTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"customer_id": map[string]any{
				"type":        "string",
				"description": "Unique identifier of the customer",
			},
		},
		"required": []string{"customer_id"},
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

	rawID, ok := input["customer_id"]
	if !ok {
		return Result{}, errors.New("customer_id is required")
	}

	customerID, err := parseUUIDString(rawID)
	if err != nil {
		return Result{}, err
	}

	session, ok := contextutil.Session(ctx)
	if !ok || session.CompanyID == uuid.Nil {
		return Result{}, errors.New("authentication context missing for customer lookup")
	}

	ctxLookup, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	record, err := h.store.GetCustomer(ctxLookup, database.GetCustomerParams{
		ID:        customerID,
		CompanyID: session.CompanyID,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Result{}, fmt.Errorf("customer %s not found", customerID)
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
	str, _ := value.(string)
	str = strings.TrimSpace(str)
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
