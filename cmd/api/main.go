package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"

	"github.com/JonMunkholm/RevProject1/internal/retrieval"
	"github.com/JonMunkholm/RevProject1/internal/stage1"
)

type server struct {
	retrieval *retrieval.Service
}

func main() {
	log.SetFlags(0)
	if err := run(); err != nil {
		log.Fatalf("api: %v", err)
	}
}

func run() error {
	_ = godotenv.Load()

	dbURL := os.Getenv("DB_URL")
	if dbURL == "" {
		return errors.New("DB_URL must be set")
	}
	openAIKey := os.Getenv("OPENAI_API_KEY")
	if openAIKey == "" {
		return errors.New("OPENAI_API_KEY must be set")
	}
	openAIBase := os.Getenv("OPENAI_API_BASE")
	openAIProject := os.Getenv("OPENAI_PROJECT_ID")
	port := os.Getenv("PORT")
	if strings.TrimSpace(port) == "" {
		port = ":8080"
	}

	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		return fmt.Errorf("open db: %w", err)
	}
	defer db.Close()
	db.SetConnMaxLifetime(5 * time.Minute)
	db.SetMaxIdleConns(4)
	db.SetMaxOpenConns(8)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		return fmt.Errorf("ping db: %w", err)
	}

	if err := stage1.EnsureSchema(ctx, db); err != nil {
		return fmt.Errorf("ensure schema: %w", err)
	}

	retrievalSvc, err := retrieval.NewService(retrieval.Config{
		DB:        db,
		OpenAIKey: openAIKey,
		OpenAIURL: openAIBase,
		ProjectID: openAIProject,
	})
	if err != nil {
		return fmt.Errorf("init retrieval: %w", err)
	}

	srv := &server{retrieval: retrievalSvc}
	router := chi.NewRouter()
	router.Use(middleware.Logger)
	router.Get("/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/api/search", http.StatusSeeOther)
	})
	router.Get("/api/search", srv.handleSearch)

	log.Printf("listening on %s", port)
	return http.ListenAndServe(port, router)
}

func (s *server) handleSearch(w http.ResponseWriter, r *http.Request) {
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	if query == "" {
		respondError(w, http.StatusBadRequest, "query parameter 'q' is required")
		return
	}

	limit := parseLimit(r.URL.Query().Get("limit"))

	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()

	results, err := s.retrieval.Search(ctx, retrieval.QueryParams{
		Query: query,
		Limit: limit,
	})
	if err != nil {
		log.Printf("retrieval error: %v", err)
		respondError(w, http.StatusInternalServerError, "search failed")
		return
	}

	resp := searchResponse{Results: make([]searchResult, 0, len(results))}
	for _, result := range results {
		resp.Results = append(resp.Results, searchResult{
			Reference: result.ASCReference,
			Score:     result.Score,
			Excerpt:   buildExcerpt(result.Content),
			Content:   result.Content,
		})
	}

	respondJSON(w, http.StatusOK, resp)
}

func parseLimit(raw string) int {
	if raw == "" {
		return 5
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value <= 0 {
		return 5
	}
	if value > 25 {
		return 25
	}
	return value
}

func respondError(w http.ResponseWriter, status int, message string) {
	respondJSON(w, status, map[string]string{"error": message})
}

func respondJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		log.Printf("failed to write json: %v", err)
	}
}

func buildExcerpt(content string) string {
	const max = 240
	sanitised := strings.TrimSpace(content)
	if len(sanitised) <= max {
		return sanitised
	}
	return sanitised[:max] + "…"
}

type searchResponse struct {
	Results []searchResult `json:"results"`
}

type searchResult struct {
	Reference string  `json:"ref"`
	Score     float64 `json:"score"`
	Excerpt   string  `json:"excerpt"`
	Content   string  `json:"content"`
}
