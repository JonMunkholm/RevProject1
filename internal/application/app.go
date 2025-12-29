package application

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/JonMunkholm/RevProject1/internal/ai"
	docsvr "github.com/JonMunkholm/RevProject1/internal/ai/documents"
	catalogProvider "github.com/JonMunkholm/RevProject1/internal/ai/provider/catalog"
	geminiProvider "github.com/JonMunkholm/RevProject1/internal/ai/provider/gemini"
	openaiProvider "github.com/JonMunkholm/RevProject1/internal/ai/provider/openai"
	aitool "github.com/JonMunkholm/RevProject1/internal/ai/tool"
	"github.com/JonMunkholm/RevProject1/internal/config"
	"github.com/JonMunkholm/RevProject1/internal/database"
	"github.com/JonMunkholm/RevProject1/internal/handler"
	"github.com/JonMunkholm/RevProject1/internal/middleware"
	_ "github.com/lib/pq"
)

const (
	defaultAIProvider = "openai"
	catalogCacheTTL   = 5 * time.Minute
)

type App struct {
	router            http.Handler
	db                *database.Queries
	rawDB             *sql.DB
	jwtSecret         string
	port              string
	credentialStore   ai.CredentialStore
	credentialCipher  ai.CredentialCipher
	credentialEvents  *ai.CredentialEventStore
	credentialMetrics ai.CredentialMetrics
	aiResolver        ai.CredentialResolver
	convService       *ai.ConversationService
	docService        *ai.DocumentService
	toolAuditStore    ai.ToolInvocationStore
	aiSystemPrompt    string
	docWorker         *docsvr.Worker
	aiClient          *ai.Client
	aiAPIKey          string
	providerCatalog   *catalogProvider.Loader
	aiHandler         *handler.AI

	// Rate limiters for auth endpoints
	loginLimiter    *middleware.IPRateLimiter
	registerLimiter *middleware.IPRateLimiter
	refreshLimiter  *middleware.IPRateLimiter
}

// New creates a new App instance with all dependencies initialized.
// Returns an error if any required configuration is missing or invalid.
func New() (*App, error) {
	db, rawDB, err := dbConnect()
	if err != nil {
		return nil, fmt.Errorf("database connection: %w", err)
	}

	jwtSecret, err := requireEnv("JWT_SECRET")
	if err != nil {
		return nil, err
	}

	port, err := requireEnv("PORT")
	if err != nil {
		return nil, err
	}

	app := &App{
		db:        db,
		rawDB:     rawDB,
		jwtSecret: jwtSecret,
		port:      port,
	}

	if err := app.initAI(); err != nil {
		return nil, fmt.Errorf("AI initialization: %w", err)
	}

	// Initialize rate limiters for auth endpoints
	if config.RateLimitEnabled {
		app.loginLimiter = middleware.NewIPRateLimiter(config.RateLimitLogin, 2)
		app.registerLimiter = middleware.NewIPRateLimiter(config.RateLimitRegister, 1)
		app.refreshLimiter = middleware.NewIPRateLimiter(config.RateLimitRefresh, 3)
	}

	app.loadRoutes()

	return app, nil
}

func (a *App) initAI() error {
	key, err := requireEnv("AI_CREDENTIAL_KEY")
	if err != nil {
		return err
	}

	cipher, err := ai.NewAESCipherFromBase64(key)
	if err != nil {
		return fmt.Errorf("invalid AI_CREDENTIAL_KEY: %w", err)
	}

	a.aiSystemPrompt = os.Getenv("AI_SYSTEM_PROMPT")

	store := ai.NewCredentialSQLStore(a.db)
	credentialEvents := ai.NewCredentialEventSQLStore(a.db)
	credentialMetrics := ai.NewCredentialMetrics(nil)
	logger := ai.NewNoopCredentialLogger()
	a.credentialStore = store
	a.credentialCipher = cipher
	a.aiResolver = ai.NewDBCredentialResolver(store, cipher, logger)
	a.credentialEvents = credentialEvents
	a.credentialMetrics = credentialMetrics

	clientLogger := ai.NewNoopLogger()
	convStore := ai.NewConversationSQLStore(a.db)
	a.convService = ai.NewConversationService(convStore, clientLogger)

	docStore := ai.NewDocumentSQLStore(a.db)
	a.docService = ai.NewDocumentService(docStore, clientLogger)

	a.toolAuditStore = ai.NewToolAuditSQLStore(a.db)
	a.docWorker = docsvr.NewWorker(a.docService, nil, clientLogger, 0)

	a.aiAPIKey = os.Getenv("OPENAI_API_KEY")
	if a.aiAPIKey == "" {
		log.Println("AI: OPENAI_API_KEY not set; expecting per-tenant credentials")
	}

	openAIConfig := openaiProvider.Config{
		BaseURL:      getEnvOrDefault("OPENAI_API_BASE", "https://api.openai.com/v1"),
		Model:        getEnvOrDefault("OPENAI_MODEL", "gpt-4o"),
		SystemPrompt: a.aiSystemPrompt,
		Logger:       clientLogger,
	}

	geminiConfig := geminiProvider.Config{
		BaseURL: getEnvOrDefault("GEMINI_API_BASE", "https://generativelanguage.googleapis.com/v1beta"),
		Model:   getEnvOrDefault("GEMINI_MODEL", "gemini-pro"),
		Logger:  clientLogger,
	}

	clientConfig := ai.Config{
		Providers: map[string]ai.ProviderFactory{
			defaultAIProvider: ai.NewOpenAIProviderFactory(openAIConfig),
			"gemini":          ai.NewGeminiProviderFactory(geminiConfig),
		},
		DefaultProvider: defaultAIProvider,
		Logger:          clientLogger,
		Credentials:     a.aiResolver,
		Tools: []ai.Tool{
			aitool.ListCustomersTool{Store: a.db},
			aitool.FetchCustomerTool{Store: a.db},
		},
	}

	client, err := ai.NewClient(clientConfig)
	if err != nil {
		return fmt.Errorf("failed to initialise ai client: %w", err)
	}
	a.aiClient = client

	if a.docWorker != nil {
		processor := docsvr.NewAIProcessor(a.aiClient, a.aiResolver, a.aiAPIKey, defaultAIProvider)
		a.docWorker.SetProcessor(processor)
	}

	a.providerCatalog = catalogProvider.NewLoader(a.db, catalogCacheTTL)
	return nil
}

func (a *App) newAIHandler() *handler.AI {
	catalogEntries := ai.ProviderCatalog()
	if a.providerCatalog != nil {
		if entries := a.providerCatalog.Entries(context.Background()); len(entries) > 0 {
			catalogEntries = entries
		}
	}

	return &handler.AI{
		Conversations:     a.convService,
		Documents:         a.docService,
		DefaultProvider:   defaultAIProvider,
		Client:            a.aiClient,
		Resolver:          a.aiResolver,
		APIKey:            a.aiAPIKey,
		CredentialStore:   a.credentialStore,
		CredentialCipher:  a.credentialCipher,
		CredentialEvents:  a.credentialEvents,
		CredentialMetrics: a.credentialMetrics,
		ProviderCatalog:   catalogEntries,
		CatalogLoader:     a.providerCatalog,
	}
}

// Start server on port, with graceful shutdown
func (a *App) Start(ctx context.Context) error {
	server := &http.Server{
		Addr:    a.port,
		Handler: a.router,
	}

	if a.docWorker != nil && a.aiClient != nil {
		a.docWorker.Start(ctx)
		defer a.docWorker.Stop()
	}

	// Run server in a goroutine so we can listen for context cancellation
	errCh := make(chan error, 1)
	go func() {
		err := server.ListenAndServe()
		if err != nil {
			errCh <- fmt.Errorf("failed to start server: %w", err)
		}
		close(errCh)
	}()

	fmt.Println("Starting server")
	select {
	case <-ctx.Done():
		log.Println("Shutting down server...")

		timeout, cancel := context.WithTimeout(context.Background(), time.Second*10)
		defer cancel()

		return server.Shutdown(timeout)
	case err := <-errCh:
		return err
	}
}

func dbConnect() (*database.Queries, *sql.DB, error) {
	dbURL := os.Getenv("DB_URL")
	if dbURL == "" {
		return nil, nil, fmt.Errorf("DB_URL must be set")
	}

	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to open DB: %w", err)
	}

	// Verify connection is alive
	if err := db.Ping(); err != nil {
		return nil, nil, fmt.Errorf("failed to ping DB: %w", err)
	}

	return database.New(db), db, nil
}

func requireEnv(name string) (string, error) {
	val := os.Getenv(name)
	if val == "" {
		return "", fmt.Errorf("%s must be set", name)
	}
	return val, nil
}

func getEnvOrDefault(name, defaultValue string) string {
	if val := os.Getenv(name); val != "" {
		return val
	}
	return defaultValue
}
