# Quick Start Guide

Get up and running with go-gcp-middleware in 5 minutes.

## Prerequisites

- Go 1.21 or later
- Google Cloud Project with:
  - Cloud Logging API enabled
  - Cloud Trace API enabled
- GCP credentials configured (see [Authentication](#authentication))

## Installation

```bash
go get github.com/sirch/go-gcp-middleware
```

**Note**: This is a private Azure DevOps repository. See [INSTALL_AZURE_DEVOPS.md](./INSTALL_AZURE_DEVOPS.md) for authentication setup instructions.

## Authentication

### Option 1: Application Default Credentials (Recommended for GCP)

When running on GCP (Cloud Run, GCE, GKE), credentials are automatic.

For local development:
```bash
gcloud auth application-default login
```

### Option 2: Service Account Key (Local Development)

```bash
export GOOGLE_APPLICATION_CREDENTIALS=/path/to/service-account-key.json
```

## Set Your Project ID

```bash
export GOOGLE_CLOUD_PROJECT=your-project-id
```

## Basic Example

Create `main.go`:

```go
package main

import (
    "context"
    "encoding/json"
    "log"
    "net/http"
    "os"

    middleware "github.com/sirch/go-gcp-middleware"
    "github.com/sirch/go-gcp-middleware/logger"
)

func main() {
    ctx := context.Background()

    // Configure middleware
    config := middleware.Config{
        ServiceName:    "my-api",
        ServiceVersion: "1.0.0",
        ProjectID:      os.Getenv("GOOGLE_CLOUD_PROJECT"),

        // Logging
        EnableConsole: true,  // Log to stdout
        EnableGCP:     true,  // Log to GCP Cloud Logging
        LogLevel:      logger.LevelInfo,

        // Tracing
        EnableTracing: true,
        TraceRatio:    0.1,  // 10% sampling
    }

    // Initialize client
    client, err := middleware.NewClient(ctx, config)
    if err != nil {
        log.Fatal(err)
    }
    defer client.Shutdown(ctx)

    // Create handlers
    http.Handle("/", client.HTTPHandler(homeHandler, "GET /"))
    http.Handle("/api/hello", client.HTTPHandler(helloHandler, "GET /api/hello"))

    // Start server
    client.Logger().InfoContext(ctx, "Server starting on :8080")
    if err := http.ListenAndServe(":8080", nil); err != nil {
        log.Fatal(err)
    }
}

func homeHandler(w http.ResponseWriter, r *http.Request) {
    w.WriteHeader(http.StatusOK)
    w.Write([]byte("Welcome!"))
}

func helloHandler(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()

    // Use logger from context or get global logger
    log := logger.Global()
    log.InfoContext(ctx, "Hello endpoint called")

    response := map[string]string{
        "message": "Hello, World!",
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(response)
}
```

## Run Your Application

```bash
go run main.go
```

Test it:
```bash
curl http://localhost:8080/api/hello
```

## View Logs and Traces in GCP

### Cloud Logging

1. Go to [Cloud Logging](https://console.cloud.google.com/logs)
2. Filter by your service: `resource.labels.service_name="my-api"`
3. Click on any log entry to see trace correlation

### Cloud Trace

1. Go to [Cloud Trace](https://console.cloud.google.com/traces)
2. See distributed traces with timing information
3. Click on a trace to see the full span tree

### Correlated View

To see logs for a specific trace:
1. In Cloud Trace, copy a trace ID
2. In Cloud Logging, filter by: `trace="projects/PROJECT_ID/traces/TRACE_ID"`
3. All logs for that request appear together

## Next Steps

### Add Custom Spans

```go
func complexHandler(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()

    // Create custom span
    ctx, span := telemetry.StartSpanFromContext(ctx, "database-query")
    defer span.End()

    // Add attributes
    span.SetAttributes(
        attribute.String("query.type", "SELECT"),
        attribute.Int("limit", 100),
    )

    // Your logic here
    results := queryDatabase(ctx)

    // Log with trace correlation
    logger.Global().InfoContext(ctx, "Query completed",
        "results_count", len(results))
}
```

### Add Authentication Middleware

```go
func authMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        token := r.Header.Get("Authorization")
        if token == "" {
            http.Error(w, "Unauthorized", http.StatusUnauthorized)
            return
        }

        // Validate token and add to context
        userID := validateToken(token)
        ctx := context.WithValue(r.Context(), "user_id", userID)

        // Add to trace
        telemetry.AddSpanAttributesContext(ctx,
            attribute.String("user.id", userID))

        next.ServeHTTP(w, r.WithContext(ctx))
    })
}

// Use with custom chain
chain := client.StandardChain("api").Append(authMiddleware)
http.Handle("/api/protected", chain.ThenFunc(protectedHandler))
```

### Error Handling

```go
func handlerWithErrors(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()
    log := logger.Global()

    data, err := fetchData(ctx)
    if err != nil {
        // Log error with trace correlation
        log.LogError(ctx, err, "Failed to fetch data",
            "source", "database",
            "retry_count", retries)

        // Return error response
        http.Error(w, "Internal Server Error", http.StatusInternalServerError)
        return
    }

    // Success
    json.NewEncoder(w).Encode(data)
}
```

### Custom Middleware Chain

```go
// Create a custom chain for your API
apiChain := middleware.NewChain(
    middleware.OTelHTTP("api"),
    middleware.RequestID,
    rateLimitMiddleware,
    authMiddleware,
    middleware.Logging,
    middleware.Recovery,
    middleware.CORS,
)

http.Handle("/api/endpoint", apiChain.ThenFunc(handler))
```

## Configuration Options

### Development Configuration

```go
config := middleware.Config{
    ServiceName:    "my-api",
    ProjectID:      os.Getenv("GOOGLE_CLOUD_PROJECT"),
    Environment:    "development",

    EnableConsole: true,
    EnableGCP:     false,  // Disable GCP logging locally
    LogLevel:      logger.LevelDebug,
    PrettyLog:     true,   // Pretty print for readability

    EnableTracing: true,
    TraceRatio:    1.0,    // 100% sampling in dev
}
```

### Production Configuration

```go
config := middleware.Config{
    ServiceName:    "my-api",
    ProjectID:      os.Getenv("GOOGLE_CLOUD_PROJECT"),
    Environment:    "production",

    EnableConsole: false,  // Console not needed in production
    EnableGCP:     true,   // GCP Cloud Logging
    LogLevel:      logger.LevelInfo,

    EnableTracing: true,
    TraceRatio:    0.1,    // 10% sampling to control costs
}
```

## Common Patterns

### Graceful Shutdown

```go
func main() {
    // ... setup ...

    server := &http.Server{Addr: ":8080", Handler: mux}

    // Start server
    go server.ListenAndServe()

    // Wait for interrupt
    quit := make(chan os.Signal, 1)
    signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
    <-quit

    // Shutdown
    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()

    server.Shutdown(ctx)      // Stop server first
    client.Shutdown(ctx)      // Then flush traces/logs
}
```

### Different Log Streams

```go
// Create separate loggers for different components
apiLogger := client.Logger().WithLogName("api-requests")
dbLogger := client.Logger().WithLogName("database")

// Each maintains trace correlation
apiLogger.InfoContext(ctx, "Request received")
dbLogger.InfoContext(ctx, "Query executed")
```

## Troubleshooting

### Logs Not Appearing in GCP

- Check `GOOGLE_CLOUD_PROJECT` is set correctly
- Verify Cloud Logging API is enabled
- Check IAM permissions (need `roles/logging.logWriter`)
- Ensure `EnableGCP: true` in config

### Traces Not Appearing

- Check Cloud Trace API is enabled
- Verify IAM permissions (need `roles/cloudtrace.agent`)
- Sampling might be too low - increase `TraceRatio` for testing
- Wait a few minutes for traces to propagate

### "Permission Denied" Errors

Your service account needs these roles:
- `roles/logging.logWriter` for Cloud Logging
- `roles/cloudtrace.agent` for Cloud Trace

Grant with:
```bash
gcloud projects add-iam-policy-binding PROJECT_ID \
    --member=serviceAccount:SERVICE_ACCOUNT_EMAIL \
    --role=roles/logging.logWriter

gcloud projects add-iam-policy-binding PROJECT_ID \
    --member=serviceAccount:SERVICE_ACCOUNT_EMAIL \
    --role=roles/cloudtrace.agent
```

## Learn More

- [Full Documentation](./README.md)
- [Advanced Examples](./examples/advanced/)
- [API Reference](https://pkg.go.dev/dev.azure.com/mclm/3f05d17c-a1a0-4d63-a72c-dcefedf9a211/go-gcp-middleware)
- [Contributing Guide](./CONTRIBUTING.md)

## Support

- [GitHub Issues](https://dev.azure.com/mclm/3f05d17c-a1a0-4d63-a72c-dcefedf9a211/go-gcp-middleware/issues)
- [GitHub Discussions](https://dev.azure.com/mclm/3f05d17c-a1a0-4d63-a72c-dcefedf9a211/go-gcp-middleware/discussions)
