# Go GCP Middleware

A comprehensive, production-ready Go middleware package for web services running on Google Cloud Platform. Provides unified logging, distributed tracing with OpenTelemetry, and essential HTTP middleware with seamless GCP integration.

## Features

### 🔍 Observability
- **Distributed Tracing**: OpenTelemetry integration with Google Cloud Trace
- **Structured Logging**: Google Cloud Logging with trace correlation
- **Request Tracking**: Automatic request ID generation and propagation
- **Performance Monitoring**: Automatic latency and status code tracking

### 🛡️ Middleware Components
- **CORS**: Configurable cross-origin resource sharing
- **Recovery**: Panic recovery with detailed logging
- **Logging**: Automatic HTTP request/response logging
- **Request ID**: Request ID injection and tracking
- **Timeout**: Configurable request timeouts
- **OpenTelemetry**: Automatic span creation and propagation

### ☁️ GCP Integration
- **Cloud Logging**: Native GCP log format with severity levels
- **Cloud Trace**: Seamless trace export to Google Cloud Trace
- **Resource Detection**: Automatic GCP environment detection (Cloud Run, GCE, GKE)
- **Trace Correlation**: Logs automatically linked to traces in Cloud Console

## Installation

```bash
go get github.com/sircho/go-gcp-middleware
```

**Note**: This is a private Azure DevOps repository. See [INSTALL_AZURE_DEVOPS.md](./INSTALL_AZURE_DEVOPS.md) for authentication setup instructions.

## Development Setup

This project uses [Task](https://taskfile.dev/) for running development commands instead of Make.

### Installing Task

**macOS (Homebrew):**
```bash
brew install go-task/tap/go-task
```

**Linux:**
```bash
# Using snap
sudo snap install task --classic

# Using install script
sh -c "$(curl --location https://taskfile.dev/install.sh)" -- -d -b /usr/local/bin
```

**Windows:**
```bash
# Using Chocolatey
choco install go-task

# Using Scoop
scoop install task
```

**Using Go:**
```bash
go install github.com/go-task/task/v3/cmd/task@latest
```

For more installation options, see the [official installation guide](https://taskfile.dev/installation/).

### Using Task

List all available tasks:
```bash
task --list
# or simply
task
```

Common development tasks:
```bash
# Install development tools
task install-tools

# Download dependencies
task deps

# Build the package
task build

# Run tests
task test

# Run tests with coverage
task test:coverage

# Run tests with race detector
task test:race

# Run integration tests (requires GCP credentials)
task test:integration

# Format code
task fmt

# Run linter
task lint

# Run go vet
task vet

# Run all checks (fmt, vet, test)
task check

# Build example binaries
task examples

# Run basic example
task run:basic

# Run advanced example
task run:advanced

# Clean build artifacts
task clean

# Serve documentation
task doc

# Install git pre-commit hooks
task hooks

# Create a git tag
task tag VERSION=v1.0.0
```

CI tasks:
```bash
# Run CI tests
task ci:test

# Run CI linting
task ci:lint
```

## Quick Start

```go
package main

import (
    "context"
    "net/http"
    "os"

    middleware "github.com/sircho/go-gcp-middleware"
    "github.com/sircho/go-gcp-middleware/logger"
)

func main() {
    ctx := context.Background()

    // Configure middleware
    config := middleware.Config{
        ServiceName:    "my-api",
        ServiceVersion: "1.0.0",
        ProjectID:      os.Getenv("GOOGLE_CLOUD_PROJECT"),

        EnableConsole: true,  // Log to stdout
        EnableGCP:     true,  // Log to GCP Cloud Logging
        LogLevel:      logger.LevelInfo,

        EnableTracing: true,
        TraceRatio:    0.1,  // 10% sampling
    }

    // Initialize client
    client, err := middleware.NewClient(ctx, config)
    if err != nil {
        panic(err)
    }
    defer client.Shutdown(ctx)

    // Create handler with automatic middleware
    http.Handle("/api/hello", client.HTTPHandler(helloHandler, "GET /api/hello"))

    // Start server
    http.ListenAndServe(":8080", nil)
}

func helloHandler(w http.ResponseWriter, r *http.Request) {
    w.WriteHeader(http.StatusOK)
    w.Write([]byte("Hello, World!"))
}
```

## Configuration

### Config Options

```go
type Config struct {
    // Service identification
    ServiceName    string  // Required: Service name
    ServiceVersion string  // Service version (default: "1.0.0")
    Environment    string  // Environment (default: "production")
    ProjectID      string  // Required: GCP Project ID

    // Logging
    LogName       string        // Custom log name (default: ServiceName)
    EnableConsole bool          // Enable stdout logging
    EnableGCP     bool          // Enable GCP Cloud Logging
    LogLevel      logger.Level  // Log level (default: Info)
    PrettyLog     bool          // Pretty-print console logs

    // Tracing
    EnableTracing bool    // Enable OpenTelemetry tracing
    TraceRatio    float64 // Sampling ratio 0.0-1.0 (default: 0.1)

    // Custom attributes
    Attributes map[string]string // Custom resource attributes
}
```

### Environment Variables

The package automatically reads from these environment variables:

- `GOOGLE_CLOUD_PROJECT` or `GCP_PROJECT`: GCP Project ID
- `ENVIRONMENT`: Deployment environment (dev, staging, production)

## Usage Examples

### Basic HTTP Handler

```go
// Automatic middleware stack: Tracing -> RequestID -> Logging -> Recovery -> CORS
http.Handle("/api/users", client.HTTPHandler(usersHandler, "GET /api/users"))
```

### Handler with Timeout

```go
// Adds timeout middleware to the stack
handler := client.HTTPHandlerWithTimeout(
    slowHandler,
    "POST /api/process",
    5 * time.Second,
)
http.Handle("/api/process", handler)
```

### Custom Middleware Chain

```go
// Create a custom chain with your own middleware
chain := client.StandardChain("GET /api/data").
    Append(authMiddleware).
    Append(rateLimitMiddleware)

http.Handle("/api/data", chain.ThenFunc(dataHandler))
```

### API-Optimized Chain

```go
// Optimized for APIs: Tracing -> RequestID -> Logging -> Recovery -> Timeout -> CORS
apiChain := client.APIChain("POST /api/items", 10*time.Second)
http.Handle("/api/items", apiChain.ThenFunc(createHandler))
```

### Manual Middleware Composition

```go
// Full control over middleware order
handler := middleware.OTelHTTP("my-operation")(
    middleware.RequestID(
        middleware.Logging(
            middleware.Recovery(
                middleware.CORS(myHandler),
            ),
        ),
    ),
)
```

## Logging

### Using the Logger

```go
// Get logger from client
log := client.Logger()

// Context-aware logging (automatically includes trace info)
log.InfoContext(ctx, "User logged in", "user_id", userId)
log.ErrorContext(ctx, "Failed to process", "error", err)

// Standard logging
log.Info("Server started", "port", 8080)
log.Warn("High memory usage", "percent", 85)

// HTTP request logging (done automatically by middleware)
log.LogHTTPRequest(ctx, method, path, statusCode, duration)

// Error logging with span correlation
log.LogError(ctx, err, "Database query failed", "query", sql)
```

### Log Levels

```go
logger.LevelDebug    // Detailed debugging information
logger.LevelInfo     // General information (default)
logger.LevelWarn     // Warning messages
logger.LevelError    // Error messages
logger.LevelCritical // Critical errors (GCP-specific)
```

### Custom Log Name

```go
// Create logger with different log name for specific components
dbLogger := client.Logger().WithLogName("database-operations")
cacheLogger := client.Logger().WithLogName("cache-operations")
```

## Tracing

### Automatic Tracing

HTTP handlers automatically create spans when using the middleware:

```go
// Automatic span creation and propagation
http.Handle("/api/endpoint", client.HTTPHandler(handler, "GET /api/endpoint"))
```

### Manual Span Creation

```go
func handler(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()

    // Create custom span
    ctx, span := client.Telemetry().GetTracer("my-service").Start(ctx, "database-query")
    defer span.End()

    // Add attributes
    span.SetAttributes(
        attribute.String("db.table", "users"),
        attribute.Int("query.limit", 100),
    )

    // Use the context with the new span
    result, err := queryDatabase(ctx)
    if err != nil {
        span.RecordError(err)
        span.SetStatus(codes.Error, "Query failed")
    }
}
```

### Helper Functions

```go
import "github.com/sircho/go-gcp-middleware/telemetry"

// Start different span types
ctx, span := telemetry.StartServerSpan(ctx, "handle-request")
ctx, span := telemetry.StartClientSpan(ctx, "call-api")
ctx, span := telemetry.StartProducerSpan(ctx, "publish-message")

// Add events
telemetry.AddSpanEventContext(ctx, "cache-miss",
    attribute.String("key", cacheKey))

// Record errors
telemetry.RecordErrorContext(ctx, err, "Operation failed")

// Get trace information
traceID := telemetry.GetTraceID(ctx)
spanID := telemetry.GetSpanID(ctx)
```

## Middleware Components

### CORS

Configurable cross-origin resource sharing:

```go
// Automatic CORS with sensible defaults
handler := middleware.CORS(myHandler)

// CORS is included by default in HTTPHandler
http.Handle("/api/endpoint", client.HTTPHandler(handler, "operation"))
```

Default CORS configuration:
- Allow all origins (`*`)
- Methods: `GET, POST, PUT, DELETE, OPTIONS`
- Headers: Standard headers + custom auth headers
- Credentials: Allowed
- Max Age: 24 hours

### Recovery

Panic recovery with logging and tracing:

```go
// Automatic panic recovery
handler := middleware.Recovery(myHandler)

// Returns 500 with JSON error response
// Logs panic with stack trace
// Records panic in trace span
```

### Request ID

Automatic request ID generation and propagation:

```go
handler := middleware.RequestID(myHandler)

// Request ID from header or auto-generated
// Added to response headers
// Included in all logs
// Added to trace spans
```

### Timeout

Configurable request timeouts:

```go
// 5-second timeout
handler := middleware.Timeout(5 * time.Second)(myHandler)

// Returns 503 with timeout message if exceeded
```

## Advanced Usage

### Custom Middleware

```go
func authMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // Get logger from context or use global
        log := logger.Global()

        token := r.Header.Get("Authorization")
        if token == "" {
            log.WarnContext(r.Context(), "Missing auth token")
            http.Error(w, "Unauthorized", http.StatusUnauthorized)
            return
        }

        // Validate token and add to context
        userID, err := validateToken(token)
        if err != nil {
            log.ErrorContext(r.Context(), "Invalid token", "error", err)
            http.Error(w, "Unauthorized", http.StatusUnauthorized)
            return
        }

        // Add to context and span
        ctx := context.WithValue(r.Context(), "user_id", userID)
        telemetry.AddSpanAttributesContext(ctx,
            attribute.String("user.id", userID))

        next.ServeHTTP(w, r.WithContext(ctx))
    })
}

// Use in chain
chain := middleware.NewChain(
    middleware.OTelHTTP("api"),
    middleware.RequestID,
    authMiddleware,  // Custom middleware
    middleware.Logging,
    middleware.Recovery,
)
```

### Graceful Shutdown

```go
func main() {
    ctx := context.Background()

    client, err := middleware.NewClient(ctx, config)
    if err != nil {
        log.Fatal(err)
    }

    // Setup signal handling
    quit := make(chan os.Signal, 1)
    signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

    // Start server
    server := &http.Server{Addr: ":8080", Handler: mux}
    go server.ListenAndServe()

    // Wait for signal
    <-quit

    // Shutdown server first
    shutdownCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
    defer cancel()
    server.Shutdown(shutdownCtx)

    // Then shutdown middleware (flushes traces and logs)
    client.Shutdown(shutdownCtx)
}
```

### Multiple Log Names

```go
// Different log streams for different components
apiLogger := client.Logger().WithLogName("api-requests")
dbLogger := client.Logger().WithLogName("database")
cacheLogger := client.Logger().WithLogName("cache")

// Each logger maintains trace correlation
apiLogger.InfoContext(ctx, "Request received")
dbLogger.InfoContext(ctx, "Query executed")
cacheLogger.InfoContext(ctx, "Cache hit")

// All logs appear with the same trace_id in GCP Console
```

## Best Practices

### 1. Always Use Context

Pass context through your application for proper trace correlation:

```go
func processRequest(ctx context.Context, data string) error {
    // Create span
    ctx, span := telemetry.StartSpanFromContext(ctx, "process-request")
    defer span.End()

    // Use context-aware logging
    logger.Global().InfoContext(ctx, "Processing", "data", data)

    // Pass context to downstream functions
    return saveToDatabase(ctx, data)
}
```

### 2. Structured Logging

Use key-value pairs for structured logs:

```go
// Good
log.InfoContext(ctx, "User created",
    "user_id", userID,
    "email", email,
    "role", role)

// Avoid
log.InfoContext(ctx, fmt.Sprintf("User %s created with email %s", userID, email))
```

### 3. Error Handling

Always log errors with context:

```go
result, err := someOperation(ctx)
if err != nil {
    // Log with error details
    client.Logger().LogError(ctx, err, "Operation failed",
        "operation", "someOperation",
        "input", input)

    // Return appropriate HTTP error
    http.Error(w, "Internal server error", http.StatusInternalServerError)
    return
}
```

### 4. Sampling Strategy

Adjust sampling based on environment:

```go
traceRatio := 0.1  // 10% in production
if env == "development" {
    traceRatio = 1.0  // 100% in development
}

config := middleware.Config{
    TraceRatio: traceRatio,
    // ...
}
```

### 5. Resource Cleanup

Always shutdown gracefully to flush traces and logs:

```go
defer func() {
    shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()
    client.Shutdown(shutdownCtx)
}()
```

## GCP Integration

### Cloud Logging

Logs automatically appear in GCP Cloud Logging with:
- Proper severity levels
- Trace correlation (logs linked to traces)
- Structured fields
- Source location (file, line, function)
- HTTP request details

View in GCP Console:
```
Logging > Logs Explorer
Filter: resource.type="cloud_run_revision" AND trace="projects/PROJECT_ID/traces/TRACE_ID"
```

### Cloud Trace

Traces automatically appear in GCP Cloud Trace with:
- Span hierarchy
- Timing information
- HTTP attributes
- Custom attributes
- Error details

View in GCP Console:
```
Trace > Trace List
Click on a trace to see the full span tree
```

### Cloud Run Integration

The package automatically detects Cloud Run environment:
- Service name from metadata
- Revision from metadata
- Region from metadata
- Instance ID

### GKE Integration

Automatically detects GKE environment:
- Cluster name
- Namespace
- Pod name
- Container name

## Performance

- **Minimal Overhead**: Efficient middleware chain with ~1-2ms overhead
- **Async Export**: Traces exported asynchronously in batches
- **Sampling**: Configurable sampling to reduce costs
- **Connection Pooling**: Reuses HTTP connections for exports

## Testing

Run tests:
```bash
task test
```

Run with coverage:
```bash
task test:coverage
```

Run with race detector:
```bash
task test:race
```

Run integration tests (requires GCP credentials):
```bash
task test:integration
```

## Examples

See the [examples](./examples) directory for complete working examples:
- `examples/basic/` - Basic HTTP server with middleware
- `examples/advanced/` - Advanced patterns and custom middleware
- `examples/grpc/` - gRPC service integration (coming soon)

## Contributing

Contributions welcome! Please:
1. Fork the repository
2. Create a feature branch
3. Add tests for new functionality
4. Ensure all tests pass
5. Submit a pull request


## Related Projects

- [OpenTelemetry Go](https://github.com/open-telemetry/opentelemetry-go)
- [Google Cloud Operations](https://github.com/GoogleCloudPlatform/opentelemetry-operations-go)
- [clog](https://github.com/chainguard-dev/clog)
