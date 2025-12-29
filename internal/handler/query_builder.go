package handler

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"

	"github.com/JonMunkholm/RevProject1/app/pages"
	"github.com/JonMunkholm/RevProject1/internal/ai"
	"github.com/JonMunkholm/RevProject1/internal/auth"
	"github.com/a-h/templ"
	"github.com/google/uuid"
)

// QueryBuilder handles natural language to SQL conversion and execution.
type QueryBuilder struct {
	AIClient *ai.Client
	DB       *sql.DB
}

// schemaPromptPrefix is the instruction header for the LLM.
const schemaPromptPrefix = `You are a SQL query generator. Generate PostgreSQL queries based on natural language descriptions.

Available tables and their schemas (dynamically retrieved from database):

`

// schemaPromptRules are the rules the LLM should follow.
const schemaPromptRules = `
Rules:
1. Only generate SELECT queries (no INSERT, UPDATE, DELETE, DROP, etc.)
2. Always use proper JOINs when relating tables
3. Use meaningful column aliases for clarity
4. Add appropriate WHERE clauses based on the request
5. Include ORDER BY when ordering is implied
6. Limit results to 100 rows unless otherwise specified
7. Return ONLY the SQL query, no explanations or markdown

`

// tableInfo holds metadata about a database table.
type tableInfo struct {
	Name       string
	Columns    []columnInfo
	PrimaryKey string
}

// columnInfo holds metadata about a table column.
type columnInfo struct {
	Name       string
	DataType   string
	IsNullable bool
	ForeignKey string // empty if not a FK, otherwise "table.column"
}

// buildSchemaContext dynamically retrieves database schema metadata.
// This queries information_schema which contains only metadata, no actual data records.
func (qb *QueryBuilder) buildSchemaContext(ctx context.Context) (string, error) {
	// Get all tables in public schema
	tables, err := qb.getTables(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get tables: %w", err)
	}

	// Get columns for each table
	for i := range tables {
		cols, err := qb.getColumns(ctx, tables[i].Name)
		if err != nil {
			return "", fmt.Errorf("failed to get columns for %s: %w", tables[i].Name, err)
		}
		tables[i].Columns = cols

		pk, err := qb.getPrimaryKey(ctx, tables[i].Name)
		if err == nil {
			tables[i].PrimaryKey = pk
		}
	}

	// Get foreign key relationships
	fks, err := qb.getForeignKeys(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get foreign keys: %w", err)
	}

	// Apply FK info to columns
	for i := range tables {
		for j := range tables[i].Columns {
			key := tables[i].Name + "." + tables[i].Columns[j].Name
			if ref, ok := fks[key]; ok {
				tables[i].Columns[j].ForeignKey = ref
			}
		}
	}

	// Build the schema text
	var sb strings.Builder
	sb.WriteString(schemaPromptPrefix)

	for idx, t := range tables {
		sb.WriteString(fmt.Sprintf("%d. %s\n", idx+1, t.Name))
		for _, c := range t.Columns {
			nullable := ""
			if !c.IsNullable {
				nullable = " NOT NULL"
			}
			pk := ""
			if c.Name == t.PrimaryKey {
				pk = " (primary key)"
			}
			fk := ""
			if c.ForeignKey != "" {
				fk = fmt.Sprintf(" (foreign key -> %s)", c.ForeignKey)
			}
			sb.WriteString(fmt.Sprintf("   - %s: %s%s%s%s\n", c.Name, c.DataType, nullable, pk, fk))
		}
		sb.WriteString("\n")
	}

	sb.WriteString(schemaPromptRules)
	return sb.String(), nil
}

// getTables retrieves all table names from the public schema.
func (qb *QueryBuilder) getTables(ctx context.Context) ([]tableInfo, error) {
	query := `
		SELECT table_name
		FROM information_schema.tables
		WHERE table_schema = 'public'
		  AND table_type = 'BASE TABLE'
		ORDER BY table_name`

	rows, err := qb.DB.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tables []tableInfo
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		tables = append(tables, tableInfo{Name: name})
	}
	return tables, rows.Err()
}

// getColumns retrieves column metadata for a specific table.
func (qb *QueryBuilder) getColumns(ctx context.Context, tableName string) ([]columnInfo, error) {
	query := `
		SELECT column_name, data_type, is_nullable
		FROM information_schema.columns
		WHERE table_schema = 'public' AND table_name = $1
		ORDER BY ordinal_position`

	rows, err := qb.DB.QueryContext(ctx, query, tableName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var columns []columnInfo
	for rows.Next() {
		var c columnInfo
		var nullable string
		if err := rows.Scan(&c.Name, &c.DataType, &nullable); err != nil {
			return nil, err
		}
		c.IsNullable = nullable == "YES"
		c.DataType = strings.ToUpper(c.DataType)
		columns = append(columns, c)
	}
	return columns, rows.Err()
}

// getPrimaryKey retrieves the primary key column for a table.
func (qb *QueryBuilder) getPrimaryKey(ctx context.Context, tableName string) (string, error) {
	query := `
		SELECT kcu.column_name
		FROM information_schema.table_constraints tc
		JOIN information_schema.key_column_usage kcu
		  ON tc.constraint_name = kcu.constraint_name
		  AND tc.table_schema = kcu.table_schema
		WHERE tc.constraint_type = 'PRIMARY KEY'
		  AND tc.table_schema = 'public'
		  AND tc.table_name = $1
		LIMIT 1`

	var pk string
	err := qb.DB.QueryRowContext(ctx, query, tableName).Scan(&pk)
	return pk, err
}

// getForeignKeys retrieves all foreign key relationships.
// Returns a map from "source_table.column" -> "target_table.column"
func (qb *QueryBuilder) getForeignKeys(ctx context.Context) (map[string]string, error) {
	query := `
		SELECT
			kcu.table_name AS source_table,
			kcu.column_name AS source_column,
			ccu.table_name AS target_table,
			ccu.column_name AS target_column
		FROM information_schema.table_constraints tc
		JOIN information_schema.key_column_usage kcu
		  ON tc.constraint_name = kcu.constraint_name
		  AND tc.table_schema = kcu.table_schema
		JOIN information_schema.constraint_column_usage ccu
		  ON ccu.constraint_name = tc.constraint_name
		  AND ccu.table_schema = tc.table_schema
		WHERE tc.constraint_type = 'FOREIGN KEY'
		  AND tc.table_schema = 'public'`

	rows, err := qb.DB.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	fks := make(map[string]string)
	for rows.Next() {
		var srcTable, srcCol, tgtTable, tgtCol string
		if err := rows.Scan(&srcTable, &srcCol, &tgtTable, &tgtCol); err != nil {
			return nil, err
		}
		fks[srcTable+"."+srcCol] = tgtTable + "." + tgtCol
	}
	return fks, rows.Err()
}

// sortedTableNames returns table names in a consistent order for deterministic output.
func sortedTableNames(tables map[string]tableInfo) []string {
	names := make([]string, 0, len(tables))
	for name := range tables {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// GenerateSQL converts a natural language prompt to SQL using AI.
func (qb *QueryBuilder) GenerateSQL(w http.ResponseWriter, r *http.Request) {
	var prompt string
	contentType := r.Header.Get("Content-Type")

	// Handle JSON requests
	if strings.Contains(contentType, "application/json") {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			renderGenerateError(w, r, "Failed to parse request")
			return
		}
		var req struct {
			Prompt string `json:"prompt"`
		}
		if err := json.Unmarshal(body, &req); err != nil {
			renderGenerateError(w, r, "Failed to parse request")
			return
		}
		prompt = strings.TrimSpace(req.Prompt)
	} else if strings.Contains(contentType, "multipart/form-data") {
		// Handle multipart form data (from FormData in JS)
		if err := r.ParseMultipartForm(32 << 20); err != nil {
			renderGenerateError(w, r, "Failed to parse request")
			return
		}
		prompt = strings.TrimSpace(r.FormValue("prompt"))
	} else {
		// Handle URL-encoded form data
		if err := r.ParseForm(); err != nil {
			renderGenerateError(w, r, "Failed to parse request")
			return
		}
		prompt = strings.TrimSpace(r.FormValue("prompt"))
	}

	if prompt == "" {
		renderGenerateError(w, r, "Please enter a query description")
		return
	}

	session, ok := auth.SessionFromContext(r.Context())
	if !ok {
		renderGenerateError(w, r, "Authentication required")
		return
	}

	// Build dynamic schema context from database metadata (no actual data exposed)
	schemaContext, err := qb.buildSchemaContext(r.Context())
	if err != nil {
		renderGenerateError(w, r, "Failed to retrieve database schema")
		return
	}

	// Build the full prompt with schema context
	fullPrompt := schemaContext + "\nUser request: " + prompt

	// Get provider credentials
	opts := ai.UserOptions{
		Provider: "openai",
	}

	// Try to get credential reference from session (format: companyID:userID:providerID)
	if session.CompanyID != (uuid.UUID{}) && session.UserID != (uuid.UUID{}) {
		opts.APIKeyRef = fmt.Sprintf("%s:%s:openai", session.CompanyID.String(), session.UserID.String())
	}

	// Call the AI to generate SQL
	resp, err := qb.AIClient.Completion(r.Context(), opts, ai.CompletionRequest{
		Prompt: fullPrompt,
	})
	if err != nil {
		errMsg := err.Error()
		// Provide helpful message for missing credentials
		if strings.Contains(errMsg, "credential") || strings.Contains(errMsg, "API key") || strings.Contains(errMsg, "unauthorized") || strings.Contains(errMsg, "401") {
			renderGenerateError(w, r, "OpenAI API key not configured. Go to Settings > AI to add your credentials.")
			return
		}
		if strings.Contains(errMsg, "no providers") || strings.Contains(errMsg, "not configured") {
			renderGenerateError(w, r, "AI provider not configured. Please add OpenAI credentials in Settings > AI.")
			return
		}
		renderGenerateError(w, r, "Failed to generate SQL: "+errMsg)
		return
	}

	// Clean up the response - remove markdown code blocks if present
	sql := cleanSQLResponse(resp.Text)

	// Validate it's a SELECT query
	if !isSelectQuery(sql) {
		renderQueryError(w, r, "Only SELECT queries are allowed")
		return
	}

	renderComponent(w, r, pages.QuerySQLOutput(sql))
}

// ExecuteQuery runs the provided SQL query and returns results.
func (qb *QueryBuilder) ExecuteQuery(w http.ResponseWriter, r *http.Request) {
	contentType := r.Header.Get("Content-Type")

	if strings.Contains(contentType, "multipart/form-data") {
		// Handle multipart form data (from FormData in JS)
		if err := r.ParseMultipartForm(32 << 20); err != nil {
			renderQueryError(w, r, "Failed to parse request")
			return
		}
	} else {
		// Handle URL-encoded form data
		if err := r.ParseForm(); err != nil {
			renderQueryError(w, r, "Failed to parse request")
			return
		}
	}

	sqlQuery := strings.TrimSpace(r.FormValue("sql"))
	if sqlQuery == "" {
		renderQueryError(w, r, "No SQL query provided")
		return
	}

	// Validate it's a SELECT query
	if !isSelectQuery(sqlQuery) {
		renderQueryError(w, r, "Only SELECT queries are allowed")
		return
	}

	// Execute the query
	rows, err := qb.DB.QueryContext(r.Context(), sqlQuery)
	if err != nil {
		renderQueryError(w, r, "Query execution failed: "+err.Error())
		return
	}
	defer rows.Close()

	// Get column names
	columns, err := rows.Columns()
	if err != nil {
		renderQueryError(w, r, "Failed to get columns: "+err.Error())
		return
	}

	// Read all rows
	var results [][]string
	for rows.Next() {
		// Create a slice of interface{} to hold each column value
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			renderQueryError(w, r, "Failed to scan row: "+err.Error())
			return
		}

		// Convert values to strings
		row := make([]string, len(columns))
		for i, v := range values {
			row[i] = formatValue(v)
		}
		results = append(results, row)

		// Limit to 100 rows
		if len(results) >= 100 {
			break
		}
	}

	if err := rows.Err(); err != nil {
		renderQueryError(w, r, "Error reading results: "+err.Error())
		return
	}

	renderComponent(w, r, pages.QueryResultsTable(columns, results, len(results)))
}

// DownloadCSV executes the query and returns results as a CSV file.
func (qb *QueryBuilder) DownloadCSV(w http.ResponseWriter, r *http.Request) {
	contentType := r.Header.Get("Content-Type")

	if strings.Contains(contentType, "multipart/form-data") {
		if err := r.ParseMultipartForm(32 << 20); err != nil {
			http.Error(w, "Failed to parse request", http.StatusBadRequest)
			return
		}
	} else {
		if err := r.ParseForm(); err != nil {
			http.Error(w, "Failed to parse request", http.StatusBadRequest)
			return
		}
	}

	sqlQuery := strings.TrimSpace(r.FormValue("sql"))
	if sqlQuery == "" {
		http.Error(w, "No SQL query provided", http.StatusBadRequest)
		return
	}

	// Validate it's a SELECT query
	if !isSelectQuery(sqlQuery) {
		http.Error(w, "Only SELECT queries are allowed", http.StatusBadRequest)
		return
	}

	// Execute the query
	rows, err := qb.DB.QueryContext(r.Context(), sqlQuery)
	if err != nil {
		http.Error(w, "Query execution failed: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	// Get column names
	columns, err := rows.Columns()
	if err != nil {
		http.Error(w, "Failed to get columns: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Set headers for CSV download
	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", "attachment; filename=query_results.csv")

	// Write CSV header
	w.Write([]byte(strings.Join(columns, ",") + "\n"))

	// Write rows
	for rows.Next() {
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			continue
		}

		row := make([]string, len(columns))
		for i, v := range values {
			val := formatValue(v)
			// Escape CSV values with quotes if they contain commas or quotes
			if strings.ContainsAny(val, ",\"\n") {
				val = "\"" + strings.ReplaceAll(val, "\"", "\"\"") + "\""
			}
			row[i] = val
		}
		w.Write([]byte(strings.Join(row, ",") + "\n"))
	}
}

// cleanSQLResponse removes markdown code blocks and cleans up the SQL.
func cleanSQLResponse(text string) string {
	text = strings.TrimSpace(text)

	// Remove markdown code blocks
	if strings.HasPrefix(text, "```sql") {
		text = strings.TrimPrefix(text, "```sql")
	} else if strings.HasPrefix(text, "```") {
		text = strings.TrimPrefix(text, "```")
	}

	if strings.HasSuffix(text, "```") {
		text = strings.TrimSuffix(text, "```")
	}

	return strings.TrimSpace(text)
}

// isSelectQuery validates that the query is a SELECT statement.
func isSelectQuery(sql string) bool {
	normalized := strings.ToUpper(strings.TrimSpace(sql))

	// Check for dangerous keywords
	dangerous := []string{"INSERT", "UPDATE", "DELETE", "DROP", "TRUNCATE", "ALTER", "CREATE", "GRANT", "REVOKE"}
	for _, keyword := range dangerous {
		if strings.Contains(normalized, keyword) {
			return false
		}
	}

	// Must start with SELECT or WITH (for CTEs)
	return strings.HasPrefix(normalized, "SELECT") || strings.HasPrefix(normalized, "WITH")
}

// formatValue converts a database value to a display string.
func formatValue(v interface{}) string {
	if v == nil {
		return "NULL"
	}
	// Handle []byte (returned by database driver for text columns)
	if b, ok := v.([]byte); ok {
		return string(b)
	}
	return fmt.Sprintf("%v", v)
}

// renderQueryError renders an error message for the query results area.
func renderQueryError(w http.ResponseWriter, r *http.Request, message string) {
	renderComponent(w, r, pages.QueryResultsError(message))
}

// renderGenerateError renders an error for the AI generate endpoint.
func renderGenerateError(w http.ResponseWriter, r *http.Request, message string) {
	// Check if this is an HTMX request
	if r.Header.Get("HX-Request") == "true" {
		w.Header().Set("HX-Retarget", "#nl-error")
		w.Header().Set("HX-Reswap", "innerHTML")
		renderComponent(w, r, pages.QueryGenerateError(message))
		return
	}
	// For non-HTMX requests (like from our TypeScript), return JSON error with 400 status
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusBadRequest)
	w.Write([]byte(`{"error":"` + message + `"}`))
}

// renderComponent renders a templ component to the response.
func renderComponent(w http.ResponseWriter, r *http.Request, component templ.Component) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := component.Render(r.Context(), w); err != nil {
		http.Error(w, "Failed to render", http.StatusInternalServerError)
	}
}
