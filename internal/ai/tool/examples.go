package tool

import "context"

// WorkspaceTools groups multiple tool definitions for convenience.
type WorkspaceTools struct {
	CustomerLookup FetchCustomerTool
	TicketCreation CreateTicketTool
}

// FetchCustomerTool demonstrates a simple data retrieval tool.
type FetchCustomerTool struct{}

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
func (FetchCustomerTool) NewHandler() Handler { return fetchCustomerHandler{} }

type fetchCustomerHandler struct{}

func (fetchCustomerHandler) Invoke(ctx context.Context, input map[string]any) (Result, error) {
	id, _ := input["customer_id"].(string)
	// TODO: replace with real customer lookup logic
	return Result{
		Output: map[string]any{
			"customer": map[string]any{
				"id":      id,
				"name":    "Sample Customer",
				"segment": "Enterprise",
			},
		},
	}, nil
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
