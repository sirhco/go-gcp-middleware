package middleware

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/sirhco/go-gcp-middleware/logger"
	"github.com/sirhco/go-gcp-middleware/telemetry"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// Logging middleware logs HTTP requests with tracing integration
func Logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Create a custom response writer to capture status code
		wrapped := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

		// Get span from context for enhanced logging
		span := trace.SpanFromContext(r.Context())

		// Call the next handler
		next.ServeHTTP(wrapped, r)

		// Log the request details using enhanced logger with trace correlation
		duration := time.Since(start)

		// Add HTTP attributes to span if available
		if span != nil {
			span.SetAttributes(
				attribute.String("http.method", r.Method),
				attribute.String("http.url", r.RequestURI),
				attribute.String("http.remote_addr", r.RemoteAddr),
				attribute.Int("http.status_code", wrapped.statusCode),
				attribute.Int64("http.duration_ms", duration.Milliseconds()),
			)

			// Set span status based on HTTP status code
			if wrapped.statusCode >= 400 {
				span.SetStatus(codes.Error, fmt.Sprintf("HTTP %d", wrapped.statusCode))
			}
		}

		logger.Global().LogHTTPRequest(
			r.Context(),
			r.Method,
			r.RequestURI,
			wrapped.statusCode,
			duration,
			"remote_addr", r.RemoteAddr,
			"user_agent", r.UserAgent(),
		)
	})
}

// CORS middleware adds CORS headers to HTTP responses with tracing.
// It allows all origins and handles preflight OPTIONS requests for cross-origin resource sharing.
func CORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Add CORS event to span if available
		if span := trace.SpanFromContext(r.Context()); span != nil {
			span.AddEvent("cors.processing", trace.WithAttributes(
				attribute.String("cors.origin", r.Header.Get("Origin")),
				attribute.String("cors.method", r.Method),
			))
		}

		// Set CORS headers to allow all origins and common HTTP methods/headers
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Accept, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, X-User-Email, X-User-Groups")
		w.Header().Set("Access-Control-Allow-Credentials", "true")
		w.Header().Set("Access-Control-Max-Age", "86400")

		// If the request is a preflight OPTIONS request, respond with allowed headers and return
		if r.Method == "OPTIONS" {
			w.Header().Set("Content-Type", "text/plain")
			w.WriteHeader(http.StatusNoContent)

			// Mark span as preflight request
			if span := trace.SpanFromContext(r.Context()); span != nil {
				span.SetAttributes(attribute.Bool("cors.preflight", true))
			}
			return
		}

		// Call the next handler in the chain
		next.ServeHTTP(w, r)
	})
}

// CorsMiddleware is a wrapper function for compatibility with http.HandlerFunc
func CorsMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		CORS(next).ServeHTTP(w, r)
	}
}

// Recovery middleware recovers from panics with tracing integration
func Recovery(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				// Record panic in span
				if span := trace.SpanFromContext(r.Context()); span != nil {
					span.SetStatus(codes.Error, "panic recovered")
					span.RecordError(fmt.Errorf("panic: %v", err), trace.WithAttributes(
						attribute.String("panic.value", fmt.Sprintf("%v", err)),
					))
				}

				// Log the panic using enhanced logger with trace correlation
				logger.Global().LogPanic(r.Context())

				// Return a 500 error
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusInternalServerError)

				errorResponse := map[string]any{
					"error":   "Internal server error",
					"code":    500,
					"details": map[string]any{"panic": fmt.Sprintf("%v", err)},
				}

				if err := json.NewEncoder(w).Encode(errorResponse); err != nil {
					// If we can't encode the error response, at least log it
					fmt.Printf("Failed to encode error response: %v\n", err)
				}
			}
		}()

		next.ServeHTTP(w, r)
	})
}

// RequestID middleware adds a request ID to the context and span
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := r.Header.Get("X-Request-ID")
		if requestID == "" {
			// Use trace ID as request ID if available, otherwise generate one
			if span := trace.SpanFromContext(r.Context()); span != nil {
				if sc := span.SpanContext(); sc.IsValid() {
					requestID = sc.TraceID().String()
				}
			}
			if requestID == "" {
				requestID = fmt.Sprintf("%d", time.Now().UnixNano())
			}
		}

		// Add request ID to span
		if span := trace.SpanFromContext(r.Context()); span != nil {
			span.SetAttributes(attribute.String("http.request_id", requestID))
		}

		w.Header().Set("X-Request-ID", requestID)
		next.ServeHTTP(w, r)
	})
}

// Timeout middleware sets a timeout for requests with tracing
func Timeout(timeout time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Add timeout attribute to span
			if span := trace.SpanFromContext(r.Context()); span != nil {
				span.SetAttributes(attribute.String("http.timeout", timeout.String()))
			}

			// Use the standard timeout handler with enhanced error message
			handler := http.TimeoutHandler(next, timeout, "Request timeout - operation took longer than expected")
			handler.ServeHTTP(w, r)
		})
	}
}

// responseWriter wraps http.ResponseWriter to capture status code
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	return rw.ResponseWriter.Write(b)
}

// OTelHTTP wraps a handler with OpenTelemetry HTTP instrumentation
func OTelHTTP(operation string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return otelhttp.NewHandler(next, operation,
			otelhttp.WithTracerProvider(otel.GetTracerProvider()),
			otelhttp.WithMeterProvider(otel.GetMeterProvider()),
			otelhttp.WithPropagators(otel.GetTextMapPropagator()),
		)
	}
}

// TracedHandler creates a handler with full OpenTelemetry instrumentation
func TracedHandler(operationName string, handler http.HandlerFunc) http.Handler {
	return OTelHTTP(operationName)(
		RequestID(
			Logging(
				Recovery(
					CORS(handler),
				),
			),
		),
	)
}

// CustomSpan creates a middleware that starts a custom span
func CustomSpan(spanName string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx, span := telemetry.StartSpanFromContext(r.Context(), spanName, telemetry.SpanOptions{
				Kind: trace.SpanKindServer,
				Attributes: []attribute.KeyValue{
					attribute.String("http.method", r.Method),
					attribute.String("http.url", r.RequestURI),
					attribute.String("http.scheme", r.URL.Scheme),
					attribute.String("http.host", r.Host),
				},
			})
			defer span.End()

			// Update request context with new span
			r = r.WithContext(ctx)
			next.ServeHTTP(w, r)
		})
	}
}
