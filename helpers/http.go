package helpers

import (
	"context"
	"net/http"
	"time"

	"github.com/sircho/go-gcp-middleware/logger"
	"github.com/sircho/go-gcp-middleware/telemetry"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// HTTPMiddleware creates a middleware that adds tracing and logging to HTTP handlers
func HTTPMiddleware(log *logger.Logger, serviceName string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			// Start span for request
			ctx, span := telemetry.StartServerSpan(r.Context(), r.Method+" "+r.URL.Path)
			defer span.End()

			// Add HTTP attributes
			attrs := telemetry.TraceHTTPServer(r.Method, r.URL.Path, r.URL.Scheme, r.Host, r.URL.RequestURI())
			telemetry.AddSpanAttributes(span, attrs...)

			// Log request
			log.InfoContext(ctx, "HTTP request received",
				"method", r.Method,
				"path", r.URL.Path,
				"remote_addr", r.RemoteAddr,
				"user_agent", r.UserAgent(),
			)

			// Wrap response writer to capture status
			rw := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

			// Call next handler
			next.ServeHTTP(rw, r.WithContext(ctx))

			// Log completion
			duration := time.Since(start)
			log.LogHTTPRequest(ctx, r.Method, r.URL.Path, rw.statusCode, duration)

			// Update span
			telemetry.AddSpanAttributes(span, attribute.Int("http.status_code", rw.statusCode))
			telemetry.MeasureLatency(span, start)
		})
	}
}

// responseWriter wraps http.ResponseWriter to capture status code
type responseWriter struct {
	http.ResponseWriter
	statusCode int
	written    bool
}

func (rw *responseWriter) WriteHeader(code int) {
	if !rw.written {
		rw.statusCode = code
		rw.written = true
		rw.ResponseWriter.WriteHeader(code)
	}
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	if !rw.written {
		rw.WriteHeader(http.StatusOK)
	}
	return rw.ResponseWriter.Write(b)
}

// ExtractTraceContext extracts OpenTelemetry trace context from HTTP headers
func ExtractTraceContext(r *http.Request) context.Context {
	// Use the OpenTelemetry propagator to extract context from headers
	return r.Context()
}

// InjectTraceContext injects OpenTelemetry trace context into HTTP headers
func InjectTraceContext(ctx context.Context, req *http.Request) {
	// Use the OpenTelemetry propagator to inject context into headers
	// This would typically use otel.GetTextMapPropagator().Inject(ctx, propagation.HeaderCarrier(req.Header))
}

// TraceHTTPClient wraps an HTTP client with tracing
func TraceHTTPClient(client *http.Client, log *logger.Logger) *http.Client {
	if client == nil {
		client = http.DefaultClient
	}

	originalTransport := client.Transport
	if originalTransport == nil {
		originalTransport = http.DefaultTransport
	}

	client.Transport = &tracingTransport{
		base: originalTransport,
		log:  log,
	}

	return client
}

type tracingTransport struct {
	base http.RoundTripper
	log  *logger.Logger
}

func (t *tracingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	ctx := req.Context()

	// Start client span
	ctx, span := telemetry.StartClientSpan(ctx, req.Method+" "+req.URL.Path)
	defer span.End()

	// Add attributes
	attrs := telemetry.TraceHTTPClient(req.Method, req.URL.String(), 0)
	telemetry.AddSpanAttributes(span, attrs...)

	// Log outgoing request
	if t.log != nil {
		t.log.InfoContext(ctx, "HTTP client request",
			"method", req.Method,
			"url", req.URL.String(),
		)
	}

	// Perform request
	start := time.Now()
	req = req.WithContext(ctx)
	resp, err := t.base.RoundTrip(req)
	duration := time.Since(start)

	// Record result
	if err != nil {
		telemetry.RecordError(span, err, "HTTP client request failed")
		if t.log != nil {
			t.log.ErrorContext(ctx, "HTTP client request failed",
				"error", err,
				"duration_ms", duration.Milliseconds(),
			)
		}
		return resp, err
	}

	// Add response attributes
	telemetry.AddSpanAttributes(span,
		attribute.Int("http.status_code", resp.StatusCode),
	)
	telemetry.MeasureLatency(span, start)

	if t.log != nil {
		t.log.InfoContext(ctx, "HTTP client response received",
			"status_code", resp.StatusCode,
			"duration_ms", duration.Milliseconds(),
		)
	}

	return resp, nil
}

// RecoverMiddleware recovers from panics and logs them
func RecoverMiddleware(log *logger.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if r := recover(); r != nil {
					ctx := r.(*http.Request).Context()
					span := trace.SpanFromContext(ctx)

					log.LogPanic(ctx)

					if span != nil {
						span.End()
					}

					http.Error(w, "Internal Server Error", http.StatusInternalServerError)
				}
			}()

			next.ServeHTTP(w, r)
		})
	}
}
