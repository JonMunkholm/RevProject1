package application

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/JonMunkholm/RevProject1/internal/database"
	_ "github.com/lib/pq"
)

type App struct {
	router    http.Handler
	db        *database.Queries
	jwtSecret string
	port      string
}

// Define app struct and load routes
func New() *App {
	app := &App{
		db:        dbConnect(),
		jwtSecret: setValEnv("JWT_SECRET"),
		port:      setValEnv("PORT"),
	}

	app.loadRoutes()

	return app
}

// Start server on port, with graceful shutdown
func (a *App) Start(ctx context.Context) error {
	server := &http.Server{
		Addr:    a.port,
		Handler: a.router,
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
