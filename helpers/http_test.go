package helpers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sirhco/go-gcp-middleware/logger"
	"github.com/sirhco/go-gcp-middleware/telemetry"
)

func TestHTTPMiddleware(t *testing.T) {
	ctx := context.Background()

	// Initialize logger
	logConfig := logger.Config{
		ProjectID:     "test-project",
		ServiceName:   "test",
		EnableConsole: false,
		EnableGCP:     false,
	}
	log, err := logger.NewLogger(ctx, logConfig)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}

	middleware := HTTPMiddleware(log, "test-service")

	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rec.Code)
	}
	if rec.Body.String() != "OK" {
		t.Errorf("Expected body 'OK', got %s", rec.Body.String())
	}
}

func TestResponseWriter(t *testing.T) {
	t.Run("captures status code", func(t *testing.T) {
		rec := httptest.NewRecorder()
		rw := &responseWriter{ResponseWriter: rec, statusCode: http.StatusOK}

		rw.WriteHeader(http.StatusCreated)

		if rw.statusCode != http.StatusCreated {
			t.Errorf("Expected status 201, got %d", rw.statusCode)
		}
	})

	t.Run("write sets status if not already set", func(t *testing.T) {
		rec := httptest.NewRecorder()
		rw := &responseWriter{ResponseWriter: rec, statusCode: http.StatusOK, written: false}

		rw.Write([]byte("test"))

		if !rw.written {
			t.Error("Expected written flag to be true")
		}
		if rw.statusCode != http.StatusOK {
			t.Errorf("Expected status 200, got %d", rw.statusCode)
		}
	})

	t.Run("multiple WriteHeader calls only use first", func(t *testing.T) {
		rec := httptest.NewRecorder()
		rw := &responseWriter{ResponseWriter: rec, statusCode: http.StatusOK}

		rw.WriteHeader(http.StatusCreated)
		rw.WriteHeader(http.StatusBadRequest) // Should be ignored

		if rw.statusCode != http.StatusCreated {
			t.Errorf("Expected status 201, got %d", rw.statusCode)
		}
	})
}

func TestExtractTraceContext(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/test", nil)

	ctx := ExtractTraceContext(req)
	if ctx == nil {
		t.Error("Expected non-nil context")
	}
}

func TestInjectTraceContext(t *testing.T) {
	ctx := context.Background()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)

	// Should not panic
	InjectTraceContext(ctx, req)
}

func TestTraceHTTPClient(t *testing.T) {
	ctx := context.Background()

	logConfig := logger.Config{
		ProjectID:     "test-project",
		ServiceName:   "test",
		EnableConsole: false,
		EnableGCP:     false,
	}
	log, err := logger.NewLogger(ctx, logConfig)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}

	client := &http.Client{}
	tracedClient := TraceHTTPClient(client, log)

	if tracedClient == nil {
		t.Fatal("Expected non-nil client")
	}

	if tracedClient.Transport == nil {
		t.Error("Expected non-nil transport")
	}
}

func TestTracingTransport(t *testing.T) {
	ctx := context.Background()

	// Initialize telemetry
	telemetryConfig := telemetry.Config{
		ServiceName:   "test",
		ProjectID:     "test-project",
		EnableTracing: false,
	}
	telemetry.InitGlobal(ctx, telemetryConfig)

	logConfig := logger.Config{
		ProjectID:     "test-project",
		ServiceName:   "test",
		EnableConsole: false,
		EnableGCP:     false,
	}
	log, err := logger.NewLogger(ctx, logConfig)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}

	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))
	defer server.Close()

	// Create traced client
	client := TraceHTTPClient(&http.Client{}, log)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, server.URL, nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}
}

func TestTracingTransportError(t *testing.T) {
	ctx := context.Background()

	logConfig := logger.Config{
		ProjectID:     "test-project",
		ServiceName:   "test",
		EnableConsole: false,
		EnableGCP:     false,
	}
	log, err := logger.NewLogger(ctx, logConfig)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}

	transport := &tracingTransport{
		base: http.DefaultTransport,
		log:  log,
	}

	// Request to invalid URL should fail
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "http://invalid-domain-that-does-not-exist.local", nil)

	_, err = transport.RoundTrip(req)
	if err == nil {
		t.Error("Expected error for invalid domain")
	}
}

func TestRecoverMiddleware(t *testing.T) {
	ctx := context.Background()

	logConfig := logger.Config{
		ProjectID:     "test-project",
		ServiceName:   "test",
		EnableConsole: false,
		EnableGCP:     false,
	}
	log, err := logger.NewLogger(ctx, logConfig)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}

	middleware := RecoverMiddleware(log)

	t.Run("does not affect normal requests", func(t *testing.T) {
		handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("OK"))
		}))

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", rec.Code)
		}
	})
}

// Benchmark tests

func BenchmarkHTTPMiddleware(b *testing.B) {
	ctx := context.Background()

	logConfig := logger.Config{
		ProjectID:     "test-project",
		ServiceName:   "test",
		EnableConsole: false,
		EnableGCP:     false,
	}
	log, _ := logger.NewLogger(ctx, logConfig)

	middleware := HTTPMiddleware(log, "test-service")

	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
	}
}

func BenchmarkResponseWriter(b *testing.B) {
	rec := httptest.NewRecorder()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rw := &responseWriter{ResponseWriter: rec, statusCode: http.StatusOK}
		rw.WriteHeader(http.StatusOK)
		rw.Write([]byte("test"))
	}
}
