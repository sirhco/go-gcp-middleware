package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/sirch/go-gcp-middleware/logger"
	"github.com/sirch/go-gcp-middleware/telemetry"
	"go.opentelemetry.io/otel/trace"
)

func TestCORS(t *testing.T) {
	handler := CORS(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))

	t.Run("sets CORS headers", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("Origin", "https://example.com")
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Header().Get("Access-Control-Allow-Origin") != "*" {
			t.Error("Expected Access-Control-Allow-Origin to be *")
		}
		if rec.Header().Get("Access-Control-Allow-Methods") == "" {
			t.Error("Expected Access-Control-Allow-Methods to be set")
		}
		if rec.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", rec.Code)
		}
	})

	t.Run("handles OPTIONS preflight", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodOptions, "/test", nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusNoContent {
			t.Errorf("Expected status 204 for OPTIONS, got %d", rec.Code)
		}
		if rec.Header().Get("Access-Control-Allow-Origin") != "*" {
			t.Error("Expected CORS headers on OPTIONS")
		}
	})
}

func TestCorsMiddleware(t *testing.T) {
	handlerFunc := CorsMiddleware(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()

	handlerFunc(rec, req)

	if rec.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Error("CorsMiddleware should set CORS headers")
	}
}

func TestRecovery(t *testing.T) {
	// Initialize logger for recovery middleware
	ctx := context.Background()
	logConfig := logger.Config{
		ProjectID:     "test-project",
		ServiceName:   "test",
		EnableConsole: false,
		EnableGCP:     false,
	}
	logger.InitGlobal(ctx, logConfig)

	t.Run("recovers from panic", func(t *testing.T) {
		handler := Recovery(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			panic("test panic")
		}))

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		rec := httptest.NewRecorder()

		// Should not panic
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusInternalServerError {
			t.Errorf("Expected status 500, got %d", rec.Code)
		}

		// Check for JSON error response
		var errResp map[string]interface{}
		if err := json.NewDecoder(rec.Body).Decode(&errResp); err != nil {
			t.Fatalf("Failed to decode error response: %v", err)
		}

		if errResp["code"] != float64(500) {
			t.Errorf("Expected error code 500, got %v", errResp["code"])
		}
	})

	t.Run("does not affect normal requests", func(t *testing.T) {
		handler := Recovery(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("success"))
		}))

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", rec.Code)
		}
		if rec.Body.String() != "success" {
			t.Errorf("Expected body 'success', got %s", rec.Body.String())
		}
	})
}

func TestLogging(t *testing.T) {
	// Initialize logger
	ctx := context.Background()
	logConfig := logger.Config{
		ProjectID:     "test-project",
		ServiceName:   "test",
		EnableConsole: false,
		EnableGCP:     false,
	}
	logger.InitGlobal(ctx, logConfig)

	handler := Logging(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rec.Code)
	}
}

func TestRequestID(t *testing.T) {
	handler := RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	t.Run("generates request ID if not provided", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		requestID := rec.Header().Get("X-Request-ID")
		if requestID == "" {
			t.Error("Expected X-Request-ID header to be set")
		}
	})

	t.Run("uses provided request ID", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("X-Request-ID", "custom-request-id")
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		requestID := rec.Header().Get("X-Request-ID")
		if requestID != "custom-request-id" {
			t.Errorf("Expected request ID 'custom-request-id', got %s", requestID)
		}
	})
}

func TestTimeout(t *testing.T) {
	t.Run("completes before timeout", func(t *testing.T) {
		handler := Timeout(100 * time.Millisecond)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(10 * time.Millisecond)
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

	t.Run("times out slow request", func(t *testing.T) {
		handler := Timeout(50 * time.Millisecond)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(200 * time.Millisecond)
			w.WriteHeader(http.StatusOK)
		}))

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusServiceUnavailable {
			t.Errorf("Expected status 503 for timeout, got %d", rec.Code)
		}
	})
}

func TestResponseWriter(t *testing.T) {
	t.Run("captures status code", func(t *testing.T) {
		rec := httptest.NewRecorder()
		rw := &responseWriter{ResponseWriter: rec, statusCode: http.StatusOK}

		rw.WriteHeader(http.StatusCreated)

		if rw.statusCode != http.StatusCreated {
			t.Errorf("Expected status code 201, got %d", rw.statusCode)
		}
	})

	t.Run("write sets default status", func(t *testing.T) {
		rec := httptest.NewRecorder()
		rw := &responseWriter{ResponseWriter: rec, statusCode: http.StatusOK}

		rw.Write([]byte("test"))

		if rw.statusCode != http.StatusOK {
			t.Errorf("Expected default status 200, got %d", rw.statusCode)
		}
	})
}

func TestTracedHandler(t *testing.T) {
	// Initialize telemetry and logger
	ctx := context.Background()

	logConfig := logger.Config{
		ProjectID:     "test-project",
		ServiceName:   "test",
		EnableConsole: false,
		EnableGCP:     false,
	}
	logger.InitGlobal(ctx, logConfig)

	telemetryConfig := telemetry.Config{
		ServiceName:   "test",
		ProjectID:     "test-project",
		EnableTracing: false, // Disable actual tracing for tests
	}
	telemetry.InitGlobal(ctx, telemetryConfig)

	handler := TracedHandler("test-operation", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rec.Code)
	}
}

func TestCustomSpan(t *testing.T) {
	ctx := context.Background()

	telemetryConfig := telemetry.Config{
		ServiceName:   "test",
		ProjectID:     "test-project",
		EnableTracing: false,
	}
	telemetry.InitGlobal(ctx, telemetryConfig)

	handler := CustomSpan("test-span")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify span is in context
		span := trace.SpanFromContext(r.Context())
		if span == nil {
			t.Error("Expected span in context")
		}
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rec.Code)
	}
}

func TestClientHTTPHandler(t *testing.T) {
	ctx := context.Background()

	config := Config{
		ServiceName:   "test-service",
		ProjectID:     "test-project",
		EnableConsole: false,
		EnableGCP:     false,
		EnableTracing: false,
	}

	client, err := NewClient(ctx, config)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Shutdown(ctx)

	handler := client.HTTPHandler(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	}, "test-operation")

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rec.Code)
	}
	if rec.Body.String() != "success" {
		t.Errorf("Expected body 'success', got %s", rec.Body.String())
	}
}

func TestClientHTTPHandlerWithTimeout(t *testing.T) {
	ctx := context.Background()

	config := Config{
		ServiceName:   "test-service",
		ProjectID:     "test-project",
		EnableConsole: false,
		EnableGCP:     false,
		EnableTracing: false,
	}

	client, err := NewClient(ctx, config)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Shutdown(ctx)

	t.Run("completes within timeout", func(t *testing.T) {
		handler := client.HTTPHandlerWithTimeout(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(10 * time.Millisecond)
			w.WriteHeader(http.StatusOK)
		}, "test-op", 100*time.Millisecond)

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", rec.Code)
		}
	})
}

func TestClientHandler(t *testing.T) {
	ctx := context.Background()

	config := Config{
		ServiceName:   "test-service",
		ProjectID:     "test-project",
		EnableConsole: false,
		EnableGCP:     false,
		EnableTracing: false,
	}

	client, err := NewClient(ctx, config)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Shutdown(ctx)

	handler := client.Handler(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}, "operation")

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rec.Code)
	}
}

func TestClientStandardChain(t *testing.T) {
	ctx := context.Background()

	config := Config{
		ServiceName:   "test-service",
		ProjectID:     "test-project",
		EnableConsole: false,
		EnableGCP:     false,
		EnableTracing: false,
	}

	client, err := NewClient(ctx, config)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Shutdown(ctx)

	chain := client.StandardChain("test-op")
	if chain == nil {
		t.Fatal("Expected non-nil chain")
	}

	handler := chain.ThenFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rec.Code)
	}
}

func TestClientAPIChain(t *testing.T) {
	ctx := context.Background()

	config := Config{
		ServiceName:   "test-service",
		ProjectID:     "test-project",
		EnableConsole: false,
		EnableGCP:     false,
		EnableTracing: false,
	}

	client, err := NewClient(ctx, config)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Shutdown(ctx)

	t.Run("with timeout", func(t *testing.T) {
		chain := client.APIChain("test-op", 100*time.Millisecond)
		if chain == nil {
			t.Fatal("Expected non-nil chain")
		}

		handler := chain.ThenFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", rec.Code)
		}
	})

	t.Run("without timeout", func(t *testing.T) {
		chain := client.APIChain("test-op", 0)
		handler := chain.ThenFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", rec.Code)
		}
	})
}

func TestMiddlewareChainThen(t *testing.T) {
	called := false
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	chain := NewChain()
	wrappedHandler := chain.Then(handler)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()

	wrappedHandler.ServeHTTP(rec, req)

	if !called {
		t.Error("Handler should have been called")
	}
	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rec.Code)
	}
}

func TestMiddlewareChainThenFunc(t *testing.T) {
	called := false
	handlerFunc := func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}

	chain := NewChain()
	handler := chain.ThenFunc(handlerFunc)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if !called {
		t.Error("Handler function should have been called")
	}
}

func TestClientGetters(t *testing.T) {
	ctx := context.Background()

	config := Config{
		ServiceName:   "test-service",
		ProjectID:     "test-project",
		EnableConsole: false,
		EnableGCP:     false,
		EnableTracing: false,
	}

	client, err := NewClient(ctx, config)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Shutdown(ctx)

	t.Run("Logger returns logger", func(t *testing.T) {
		logger := client.Logger()
		if logger == nil {
			t.Error("Expected non-nil logger")
		}
	})

	t.Run("Config returns config", func(t *testing.T) {
		returnedConfig := client.Config()
		if returnedConfig.ServiceName != config.ServiceName {
			t.Errorf("Expected service name %s, got %s", config.ServiceName, returnedConfig.ServiceName)
		}
	})

	t.Run("Telemetry returns nil when disabled", func(t *testing.T) {
		telemetry := client.Telemetry()
		if telemetry != nil {
			t.Error("Expected nil telemetry when disabled")
		}
	})
}

func TestClientWithTelemetry(t *testing.T) {
	ctx := context.Background()

	config := Config{
		ServiceName:   "test-service",
		ProjectID:     "test-project",
		EnableConsole: false,
		EnableGCP:     false,
		EnableTracing: true,
		TraceRatio:    1.0,
	}

	client, err := NewClient(ctx, config)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Shutdown(ctx)

	t.Run("Telemetry returns provider when enabled", func(t *testing.T) {
		telemetry := client.Telemetry()
		if telemetry == nil {
			t.Error("Expected non-nil telemetry when enabled")
		}
	})
}

// Benchmark tests

func BenchmarkCORS(b *testing.B) {
	handler := CORS(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
	}
}

func BenchmarkRecovery(b *testing.B) {
	ctx := context.Background()
	logConfig := logger.Config{
		ProjectID:     "test-project",
		ServiceName:   "test",
		EnableConsole: false,
		EnableGCP:     false,
	}
	logger.InitGlobal(ctx, logConfig)

	handler := Recovery(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
	}
}

func BenchmarkRequestID(b *testing.B) {
	handler := RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
	}
}

func BenchmarkMiddlewareChain(b *testing.B) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	chain := NewChain(
		RequestID,
		CORS,
		Recovery,
	)

	wrappedHandler := chain.Then(handler)
	req := httptest.NewRequest(http.MethodGet, "/test", nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rec := httptest.NewRecorder()
		wrappedHandler.ServeHTTP(rec, req)
	}
}
