package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	middleware "github.com/sirhco/go-gcp-middleware"
	"github.com/sirhco/go-gcp-middleware/logger"
)

func main() {
	ctx := context.Background()

	// Get project ID from environment
	projectID := os.Getenv("GOOGLE_CLOUD_PROJECT")
	if projectID == "" {
		projectID = os.Getenv("GCP_PROJECT")
	}
	if projectID == "" {
		log.Fatal("GOOGLE_CLOUD_PROJECT or GCP_PROJECT environment variable must be set")
	}

	// Configure the middleware client
	config := middleware.Config{
		ServiceName:    "basic-api",
		ServiceVersion: "1.0.0",
		Environment:    "development",
		ProjectID:      projectID,

		// Logger settings
		EnableConsole: true, // Log to stdout
		EnableGCP:     true, // Log to GCP Cloud Logging
		LogLevel:      logger.LevelInfo,
		PrettyLog:     true, // Pretty print console logs

		LogName: "basic-api",

		// Tracing settings
		EnableTracing: true,
		TraceRatio:    1.0, // 100% sampling for development

		// Custom attributes
		Attributes: map[string]string{
			"team":   "platform",
			"region": "us-central1",
		},
	}

	// Initialize the middleware client
	client, err := middleware.NewClient(ctx, config)
	if err != nil {
		log.Fatalf("Failed to initialize middleware client: %v", err)
	}
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := client.Shutdown(shutdownCtx); err != nil {
			log.Printf("Error shutting down client: %v", err)
		}
	}()

	// Create HTTP server with instrumented handlers
	mux := http.NewServeMux()

	// Example 1: Simple handler with automatic middleware
	mux.Handle("/health", client.HTTPHandler(healthHandler, "GET /health"))

	// Example 2: Handler with custom timeout
	mux.Handle("/api/users", client.HTTPHandlerWithTimeout(usersHandler(client), "GET /api/users", 5*time.Second))

	// Example 3: Custom middleware chain
	customChain := client.StandardChain("GET /api/data").
		Append(authMiddleware) // Add custom middleware
	mux.Handle("/api/data", customChain.ThenFunc(dataHandler(client)))

	// Example 4: API-optimized chain
	apiChain := client.APIChain("POST /api/items", 10*time.Second)
	mux.Handle("/api/items", apiChain.ThenFunc(createItemHandler(client)))

	// Create server
	server := &http.Server{
		Addr:         ":8080",
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in goroutine
	go func() {
		client.Logger().InfoContext(ctx, "Starting HTTP server", "addr", server.Addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			client.Logger().ErrorContext(ctx, "Server error", "error", err)
			log.Fatalf("Server error: %v", err)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	client.Logger().InfoContext(ctx, "Shutting down server...")

	// Graceful shutdown
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		client.Logger().ErrorContext(ctx, "Server forced to shutdown", "error", err)
	}

	client.Logger().InfoContext(ctx, "Server exited")
}

// healthHandler is a simple health check endpoint
func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]any{
		"status": "healthy",
		"time":   time.Now().UTC(),
	})
}

// usersHandler demonstrates logging within a handler
func usersHandler(client *middleware.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		// Log using the client's logger
		client.Logger().InfoContext(ctx, "Fetching users")

		// Simulate some work
		time.Sleep(100 * time.Millisecond)

		users := []map[string]any{
			{"id": 1, "name": "Alice", "email": "alice@example.com"},
			{"id": 2, "name": "Bob", "email": "bob@example.com"},
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{
			"users": users,
			"count": len(users),
		})
	}
}

// dataHandler demonstrates custom span creation
func dataHandler(client *middleware.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		// Create a custom span for a specific operation
		ctx, span := client.Telemetry().GetTracer("basic-api").Start(ctx, "fetch-data-from-db")
		defer span.End()

		client.Logger().InfoContext(ctx, "Fetching data from database")

		// Simulate database query
		time.Sleep(50 * time.Millisecond)

		data := map[string]any{
			"items": []string{"item1", "item2", "item3"},
			"total": 3,
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(data)
	}
}

// createItemHandler demonstrates error handling
func createItemHandler(client *middleware.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var item map[string]any
		if err := json.NewDecoder(r.Body).Decode(&item); err != nil {
			client.Logger().ErrorContext(ctx, "Failed to decode request body", "error", err)
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		client.Logger().InfoContext(ctx, "Creating item", "item", item)

		// Simulate item creation
		time.Sleep(200 * time.Millisecond)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]any{
			"id":      123,
			"message": "Item created successfully",
			"item":    item,
		})
	}
}

// authMiddleware is an example custom middleware
func authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check for API key in header
		apiKey := r.Header.Get("X-API-Key")
		if apiKey == "" {
			http.Error(w, "Missing API key", http.StatusUnauthorized)
			return
		}

		// In a real app, validate the API key here
		if apiKey != "demo-key-12345" {
			http.Error(w, "Invalid API key", http.StatusUnauthorized)
			return
		}

		// Add user info to context
		ctx := context.WithValue(r.Context(), "user_id", "user-123")
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
