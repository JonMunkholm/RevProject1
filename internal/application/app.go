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
	openaiProvider "github.com/JonMunkholm/RevProject1/internal/ai/provider/openai"
	"github.com/JonMunkholm/RevProject1/internal/database"
	_ "github.com/lib/pq"
)

const defaultAIProvider = "openai"

type App struct {
	router            http.Handler
	db                *database.Queries
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
}

// Define app struct and load routes
func New() *App {
	app := &App{
		db:        dbConnect(),
		jwtSecret: setValEnv("JWT_SECRET"),
		port:      setValEnv("PORT"),
	}

	app.initAI()

	app.loadRoutes()

	return app
}

func (a *App) initAI() {
	key := setValEnv("AI_CREDENTIAL_KEY")
	cipher, err := ai.NewAESCipherFromBase64(key)
	if err != nil {
		log.Fatalf("invalid AI_CREDENTIAL_KEY: %v", err)
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
		BaseURL:      os.Getenv("OPENAI_API_BASE"),
		Model:        os.Getenv("OPENAI_MODEL"),
		SystemPrompt: a.aiSystemPrompt,
		Logger:       clientLogger,
	}

	clientConfig := ai.Config{
		Providers: map[string]ai.ProviderFactory{
			defaultAIProvider: ai.NewOpenAIProviderFactory(openAIConfig),
		},
		DefaultProvider: defaultAIProvider,
		Logger:          clientLogger,
		Credentials:     a.aiResolver,
	}

	client, err := ai.NewClient(clientConfig)
	if err != nil {
		log.Fatalf("failed to initialise ai client: %v", err)
	}
	a.aiClient = client

	if a.docWorker != nil {
		processor := docsvr.NewAIProcessor(a.aiClient, a.aiResolver, a.aiAPIKey, defaultAIProvider)
		a.docWorker.SetProcessor(processor)
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

func dbConnect() *database.Queries {
	dbURL := os.Getenv("DB_URL")
	if dbURL == "" {
		log.Fatal("DB_URL must be set")
	}

	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		log.Fatal("Failed to open DB:", err)
	}

	// Verify connection is alive
	if err := db.Ping(); err != nil {
		log.Fatal("Failed to ping DB:", err)
	}

	return database.New(db)
}

func setValEnv(req string) string {
	val := os.Getenv(req)
	if val == "" {
		log.Fatalf("%v must be set", req)
	}

	return val
}
