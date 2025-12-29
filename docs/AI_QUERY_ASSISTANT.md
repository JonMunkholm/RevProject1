# AI Query Assistant - Technical Documentation

## Overview

The AI Query Assistant is a natural language to SQL conversion tool that allows users to describe their data needs in plain English and receive executable PostgreSQL queries. It combines a rich SQL editing experience with AI-powered query generation.

### Key Features

- **Natural Language to SQL**: Describe queries in plain English, AI generates valid PostgreSQL
- **Monaco Editor**: Professional SQL editor with syntax highlighting and autocomplete
- **Dynamic Schema Introspection**: Automatically retrieves database schema without exposing data
- **Query Execution**: Run queries directly against the database with results display
- **CSV Export**: Download query results as CSV files
- **Security**: Read-only queries only (SELECT), role-based access control

---

## Architecture

```
┌─────────────────────────────────────────────────────────────────────┐
│                           Frontend                                   │
│  ┌─────────────────┐  ┌─────────────────┐  ┌─────────────────────┐ │
│  │  Monaco Editor  │  │  State Store    │  │    UI Renderer      │ │
│  │  (SQL editing)  │  │  (TypeScript)   │  │  (DOM updates)      │ │
│  └────────┬────────┘  └────────┬────────┘  └──────────┬──────────┘ │
│           │                    │                       │            │
│           └────────────────────┼───────────────────────┘            │
│                                │                                     │
│                    ┌───────────┴───────────┐                        │
│                    │      QueryAPI         │                        │
│                    │  (fetch + FormData)   │                        │
│                    └───────────┬───────────┘                        │
└────────────────────────────────┼────────────────────────────────────┘
                                 │ HTTP (multipart/form-data)
                                 ▼
┌─────────────────────────────────────────────────────────────────────┐
│                           Backend (Go)                               │
│  ┌─────────────────┐  ┌─────────────────┐  ┌─────────────────────┐ │
│  │  Auth Middleware│  │  QueryBuilder   │  │   AI Client         │ │
│  │  (JWT + Session)│  │   Handler       │  │   (OpenAI)          │ │
│  └────────┬────────┘  └────────┬────────┘  └──────────┬──────────┘ │
│           │                    │                       │            │
│           ▼                    ▼                       ▼            │
│  ┌─────────────────────────────────────────────────────────────┐   │
│  │                    PostgreSQL Database                       │   │
│  │  ┌──────────────────┐  ┌─────────────────────────────────┐  │   │
│  │  │ information_schema│  │      Application Tables         │  │   │
│  │  │ (schema metadata)│  │      (user data)                │  │   │
│  │  └──────────────────┘  └─────────────────────────────────┘  │   │
│  └─────────────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────────────┘
```

---

## Frontend Components

### File: `app/src/query-builder.ts`

The frontend is built with vanilla TypeScript using a component-based architecture.

#### 1. State Management (`Store` class)

```typescript
interface QueryBuilderState {
  sql: string;                    // Current SQL in the editor
  prompt: string;                 // Natural language prompt
  results: QueryResults | null;   // Query execution results
  generationError: string | null; // Errors from AI generation
  executionError: string | null;  // Errors from query execution
  isGenerating: boolean;          // Loading state for generation
  isExecuting: boolean;           // Loading state for execution
  schema: TableSchema[] | null;   // Database schema (for autocomplete)
}
```

- **Publish-subscribe pattern**: Components subscribe to state changes
- **Immutable updates**: State is replaced, not mutated
- **Separation of concerns**: Generation errors vs execution errors displayed separately

#### 2. API Client (`QueryAPI` class)

| Method | Endpoint | Description |
|--------|----------|-------------|
| `generateSQL(prompt)` | `POST /api/queries/generate` | Convert NL to SQL |
| `executeQuery(sql)` | `POST /api/queries/execute` | Run SQL query |
| `downloadCSV(sql)` | `POST /api/queries/download` | Export results as CSV |

**Key implementation details:**
- Uses `FormData` with `multipart/form-data` content type
- `credentials: 'include'` ensures session cookies are sent
- Parses HTML responses from server into structured data
- Error extraction from both JSON and HTML responses

#### 3. Monaco Editor (`SQLEditor` class)

Configuration:
```typescript
{
  language: 'sql',
  theme: 'vs-dark',
  minimap: { enabled: false },
  fontSize: 14,
  fontFamily: "'JetBrains Mono', 'Fira Code', monospace",
  automaticLayout: true,
  wordWrap: 'on',
  // ... additional settings
}
```

Features:
- SQL syntax highlighting
- Auto-completion for tables, columns, and SQL keywords
- Keyboard shortcut: `Ctrl+Enter` to run query
- Smooth scrolling and cursor animations

#### 4. UI Renderer (`UIRenderer` class)

Manages DOM updates based on state changes:
- Loading indicators (generation vs execution)
- Error displays (NL section vs results section)
- Results table rendering with proper escaping
- Button state management (disabled during operations)

#### 5. Main Controller (`QueryBuilder` class)

Orchestrates all components:
- Initializes Monaco Editor on DOM ready
- Binds event listeners for buttons and keyboard
- Coordinates state updates between API calls and UI

### File: `app/pages/queries.templ`

Server-rendered HTML template using Templ (Go templating).

#### Components:

| Component | Purpose |
|-----------|---------|
| `QueriesPage` | Full page layout with assets |
| `QueriesShell` | Sidebar + main content area |
| `QueryBuilder` | Three-section layout (Editor, AI, Results) |
| `QueryResultsTable` | Server-rendered results table |
| `QueryResultsError` | Error display component |
| `QueryGenerateError` | AI generation error display |

#### Monaco Editor Loading:
```javascript
// Loaded from CDN
<script src="https://cdn.jsdelivr.net/npm/monaco-editor@0.45.0/min/vs/loader.js"></script>

// Web Worker configuration for cross-origin loading
window.MonacoEnvironment = {
  getWorkerUrl: function(workerId, label) {
    return `data:text/javascript;charset=utf-8,${encodeURIComponent(`
      self.MonacoEnvironment = { baseUrl: '...' };
      importScripts('...workerMain.js');
    `)}`;
  }
};
```

---

## Backend Components

### File: `internal/handler/query_builder.go`

#### Handler Struct

```go
type QueryBuilder struct {
    AIClient *ai.Client  // AI provider client
    DB       *sql.DB     // Raw database connection
}
```

#### Endpoints

##### 1. `GenerateSQL` - Natural Language to SQL

**Route:** `POST /api/queries/generate`

**Flow:**
1. Parse request (supports JSON, multipart, URL-encoded)
2. Validate user session
3. Build dynamic schema context from `information_schema`
4. Call AI provider with schema + user prompt
5. Clean response (remove markdown code blocks)
6. Validate query is SELECT-only
7. Return raw SQL text

**Schema Context Generation:**
```go
func (qb *QueryBuilder) buildSchemaContext(ctx context.Context) (string, error) {
    // 1. Get all tables from public schema
    tables, err := qb.getTables(ctx)

    // 2. Get columns for each table
    for i := range tables {
        cols, err := qb.getColumns(ctx, tables[i].Name)
        tables[i].Columns = cols

        pk, err := qb.getPrimaryKey(ctx, tables[i].Name)
        tables[i].PrimaryKey = pk
    }

    // 3. Get foreign key relationships
    fks, err := qb.getForeignKeys(ctx)

    // 4. Build formatted schema text for LLM
    // ...
}
```

**Schema queries used:**
- `information_schema.tables` - List all tables
- `information_schema.columns` - Column names, types, nullability
- `information_schema.table_constraints` - Primary keys
- `information_schema.key_column_usage` - Foreign key relationships

##### 2. `ExecuteQuery` - Run SQL Query

**Route:** `POST /api/queries/execute`

**Flow:**
1. Parse multipart or URL-encoded form data
2. Validate query is SELECT-only
3. Execute query against database
4. Scan results into string slices
5. Render HTML table via Templ

**Security validations:**
```go
func isSelectQuery(sql string) bool {
    normalized := strings.ToUpper(strings.TrimSpace(sql))

    // Block dangerous keywords
    dangerous := []string{
        "INSERT", "UPDATE", "DELETE", "DROP",
        "TRUNCATE", "ALTER", "CREATE", "GRANT", "REVOKE"
    }
    for _, keyword := range dangerous {
        if strings.Contains(normalized, keyword) {
            return false
        }
    }

    // Must start with SELECT or WITH (CTEs)
    return strings.HasPrefix(normalized, "SELECT") ||
           strings.HasPrefix(normalized, "WITH")
}
```

##### 3. `DownloadCSV` - Export Results

**Route:** `POST /api/queries/download`

**Flow:**
1. Parse and validate query
2. Execute query
3. Stream results as CSV with proper escaping
4. Set `Content-Disposition: attachment` header

#### Value Formatting

```go
func formatValue(v interface{}) string {
    if v == nil {
        return "NULL"
    }
    // Handle []byte (PostgreSQL returns text as bytes)
    if b, ok := v.([]byte); ok {
        return string(b)
    }
    return fmt.Sprintf("%v", v)
}
```

---

## API Endpoints

### Authentication

All endpoints require authentication via:
- **Session cookie**: `access_token` (JWT)
- **Authorization header**: `Bearer <token>` (fallback)

Role required: `viewer` (minimum)

### Endpoint Reference

| Method | Path | Content-Type | Request | Response |
|--------|------|--------------|---------|----------|
| POST | `/api/queries/generate` | `multipart/form-data` | `prompt: string` | Raw SQL text |
| POST | `/api/queries/execute` | `multipart/form-data` | `sql: string` | HTML table |
| POST | `/api/queries/download` | `multipart/form-data` | `sql: string` | CSV file |

### Error Responses

**Generation Errors (400 Bad Request):**
```json
{"error": "Please enter a query description"}
{"error": "OpenAI API key not configured. Go to Settings > AI to add your credentials."}
```

**Execution Errors (200 OK, HTML):**
```html
<div class="query-builder__results-error">
  <span>Query execution failed: column "x" does not exist</span>
</div>
```

---

## Data Flow

### SQL Generation Flow

```
User Input                     Frontend                        Backend
    │                             │                               │
    │ "Show active customers"     │                               │
    ├────────────────────────────►│                               │
    │                             │ POST /api/queries/generate    │
    │                             ├──────────────────────────────►│
    │                             │                               │ Query information_schema
    │                             │                               │ Build schema context
    │                             │                               │ Call OpenAI API
    │                             │                               │ Clean response
    │                             │◄──────────────────────────────┤
    │                             │ "SELECT * FROM customers..."  │
    │◄────────────────────────────┤                               │
    │ SQL in Monaco Editor        │                               │
```

### Query Execution Flow

```
User Click "Run"               Frontend                        Backend
    │                             │                               │
    │                             │ POST /api/queries/execute     │
    │                             ├──────────────────────────────►│
    │                             │                               │ Validate SELECT-only
    │                             │                               │ Execute on PostgreSQL
    │                             │                               │ Render HTML table
    │                             │◄──────────────────────────────┤
    │                             │ <table>...</table>            │
    │◄────────────────────────────┤                               │
    │ Results displayed           │                               │
```

---

## Security Considerations

### Query Validation

1. **SELECT-only enforcement**: Queries must start with `SELECT` or `WITH`
2. **Keyword blocking**: INSERT, UPDATE, DELETE, DROP, etc. are rejected
3. **Row limiting**: Results capped at 100 rows by default

### Authentication & Authorization

1. **JWT middleware**: Validates session tokens
2. **Role-based access**: Requires `viewer` role minimum
3. **Company scoping**: AI credentials are company-specific

### Data Privacy

1. **Schema-only context**: LLM receives table/column metadata, never actual data
2. **No data in prompts**: User prompts don't include row contents
3. **Session isolation**: Each user's queries run in their authenticated context

### LLM Prompt Rules

```
Rules:
1. Only generate SELECT queries (no INSERT, UPDATE, DELETE, DROP, etc.)
2. Always use proper JOINs when relating tables
3. Use meaningful column aliases for clarity
4. Add appropriate WHERE clauses based on the request
5. Include ORDER BY when ordering is implied
6. Limit results to 100 rows unless otherwise specified
7. Return ONLY the SQL query, no explanations or markdown
```

---

## File Structure

```
RevProject1/
├── app/
│   ├── src/
│   │   └── query-builder.ts      # Frontend TypeScript module
│   ├── assets/
│   │   ├── js/
│   │   │   └── query-builder.js  # Compiled JavaScript
│   │   └── css/
│   │       └── pages/
│   │           └── queries.css   # Query builder styles
│   └── pages/
│       └── queries.templ         # Page template
├── internal/
│   ├── handler/
│   │   └── query_builder.go      # HTTP handlers
│   ├── ai/
│   │   └── client.go             # AI provider client
│   └── auth/
│       ├── authMiddleware.go     # JWT authentication
│       └── role_middleware.go    # Role-based authorization
├── package.json                  # NPM dependencies
└── tsconfig.json                 # TypeScript configuration
```

---

## Keyboard Shortcuts

| Shortcut | Context | Action |
|----------|---------|--------|
| `Enter` | NL input focused | Generate SQL from prompt |
| `Ctrl+Enter` | Anywhere | Run query |
| `Ctrl+Enter` | Monaco Editor | Run query |

---

## Configuration

### AI Provider Setup

Users must configure OpenAI credentials in Settings > AI:
- API Key stored per company
- Credential reference format: `{companyID}:{userID}:openai`

### Environment Variables

```env
JWT_SECRET=your-jwt-secret
DATABASE_URL=postgres://user:pass@host:5432/dbname
```

---

## Error Handling

### Frontend Error States

| Error Type | Display Location | Clears On |
|------------|------------------|-----------|
| Generation Error | AI Assistant section | New generation attempt |
| Execution Error | Results section | New query run |
| Network Error | Respective section | Retry action |

### Backend Error Responses

| Condition | Status | Response |
|-----------|--------|----------|
| Missing prompt | 400 | JSON error |
| Auth required | 401 | JSON error |
| Invalid query type | 200 | HTML error component |
| Query execution failed | 200 | HTML error component |
| Missing AI credentials | 400 | JSON with setup instructions |

---

## Future Enhancements

Potential improvements for the AI Query Assistant:

1. **Query History**: Save and recall previous queries
2. **Schema Browser**: Visual tree view of tables and columns
3. **Query Explain**: Show execution plan for generated queries
4. **Streaming Responses**: Real-time token display during generation
5. **Query Templates**: Pre-built queries for common operations
6. **Multi-statement Support**: Run multiple queries in sequence
7. **Result Visualization**: Charts and graphs from query results
8. **Query Sharing**: Share queries with team members
