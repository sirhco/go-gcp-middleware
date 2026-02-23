// Package middleware provides production-ready middleware for Go web services on Google Cloud Platform.
//
// This package offers unified logging, distributed tracing with OpenTelemetry, and essential HTTP
// middleware with seamless GCP integration. It includes automatic trace correlation between logs
// and traces, making debugging and monitoring straightforward.
//
// # Features
//
// - Distributed tracing with OpenTelemetry and Google Cloud Trace
// - Structured logging with Google Cloud Logging integration
// - Automatic request tracking and trace correlation
// - Essential HTTP middleware (CORS, Recovery, Logging, Request ID, Timeout)
// - GCP resource detection (Cloud Run, GCE, GKE)
// - Zero-configuration defaults with extensive customization options
//
// # Quick Start
//
// Initialize the middleware client with minimal configuration:
//
//	config := middleware.Config{
//	    ServiceName: "my-api",
//	    ProjectID:   os.Getenv("GOOGLE_CLOUD_PROJECT"),
//	    EnableConsole: true,
//	    EnableGCP:     true,
//	    EnableTracing: true,
//	    TraceRatio:    0.1, // 10% sampling
//	}
//
//	client, err := middleware.NewClient(context.Background(), config)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer client.Shutdown(context.Background())
//
// Create an instrumented HTTP handler:
//
//	http.Handle("/api/endpoint", client.HTTPHandler(myHandler, "GET /api/endpoint"))
//
// # Middleware Components
//
// The package provides several middleware components that can be used individually
// or composed together:
//
//   - OTelHTTP: OpenTelemetry HTTP instrumentation with automatic span creation
//   - RequestID: Request ID generation and propagation
//   - Logging: Automatic HTTP request/response logging with trace correlation
//   - Recovery: Panic recovery with detailed error logging
//   - CORS: Cross-origin resource sharing with configurable options
//   - Timeout: Request timeout handling
//
// # Logging
//
// The logger provides structured logging with automatic trace correlation:
//
//	log := client.Logger()
//	log.InfoContext(ctx, "Processing request", "user_id", userID)
//	log.ErrorContext(ctx, "Failed to save", "error", err)
//
// Logs automatically include trace IDs when using context-aware methods,
// allowing logs to be correlated with traces in Google Cloud Console.
//
// # Tracing
//
// Create custom spans for detailed tracing:
//
//	ctx, span := client.Telemetry().GetTracer("my-service").Start(ctx, "operation")
//	defer span.End()
//
//	span.SetAttributes(
//	    attribute.String("operation.type", "database"),
//	    attribute.Int("records.count", count),
//	)
//
// The telemetry package provides helper functions for common span types:
//
//	ctx, span := telemetry.StartServerSpan(ctx, "handle-request")
//	ctx, span := telemetry.StartClientSpan(ctx, "call-external-api")
//
// # Middleware Chains
//
// Create custom middleware chains for specific needs:
//
//	// Standard chain with common middleware
//	chain := client.StandardChain("operation-name")
//
//	// API-optimized chain with timeout
//	apiChain := client.APIChain("api-operation", 10*time.Second)
//
//	// Custom chain with your own middleware
//	customChain := middleware.NewChain(
//	    middleware.OTelHTTP("custom"),
//	    middleware.RequestID,
//	    authMiddleware,
//	    middleware.Logging,
//	    middleware.Recovery,
//	)
//	handler := customChain.ThenFunc(myHandler)
//
// # GCP Integration
//
// The package automatically integrates with Google Cloud Platform:
//
//   - Cloud Logging: Logs appear in GCP Console with proper severity and trace correlation
//   - Cloud Trace: Traces are exported to Google Cloud Trace for distributed tracing
//   - Resource Detection: Automatically detects Cloud Run, GCE, and GKE environments
//   - IAM: Uses Application Default Credentials for authentication
//
// # Error Handling
//
// Errors are logged with context and recorded in traces:
//
//	if err := operation(ctx); err != nil {
//	    client.Logger().LogError(ctx, err, "Operation failed",
//	        "operation", "database-query",
//	        "table", "users")
//	    return err
//	}
//
// The Recovery middleware automatically catches panics and logs them:
//
//	handler := middleware.Recovery(myHandler)
//
// # Performance
//
// The middleware is designed for production use with minimal overhead:
//
//   - Middleware chain: ~1-2ms overhead per request
//   - Trace export: Asynchronous batching (no request blocking)
//   - Sampling: Configurable to control trace volume and costs
//   - Connection pooling: Reuses connections to GCP services
//
// # Best Practices
//
// Always use context for proper trace propagation:
//
//	func processRequest(ctx context.Context, data string) error {
//	    ctx, span := telemetry.StartSpanFromContext(ctx, "process")
//	    defer span.End()
//
//	    logger.Global().InfoContext(ctx, "Processing", "data", data)
//	    return downstream(ctx, data)
//	}
//
// Use structured logging with key-value pairs:
//
//	// Good
//	log.InfoContext(ctx, "User created", "user_id", id, "email", email)
//
//	// Avoid
//	log.InfoContext(ctx, fmt.Sprintf("User %s created", id))
//
// Always shutdown gracefully to flush traces and logs:
//
//	defer func() {
//	    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
//	    defer cancel()
//	    client.Shutdown(ctx)
//	}()
//
// # Examples
//
// See the examples directory for complete working examples:
//   - examples/basic: Simple HTTP server with middleware
//   - examples/advanced: Advanced patterns with caching, auth, and more
//
// # Package Structure
//
//   - middleware: Core middleware and client (this package)
//   - middleware/logger: Structured logging with GCP integration
//   - middleware/telemetry: OpenTelemetry and tracing utilities
//   - middleware/helpers: HTTP utilities and helpers
package middleware
