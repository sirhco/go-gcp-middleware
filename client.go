package middleware

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/sircho/go-gcp-middleware/logger"
	"github.com/sircho/go-gcp-middleware/telemetry"
)

// Config holds the complete middleware configuration
type Config struct {
	// Service information
	ServiceName    string
	ServiceVersion string
	Environment    string

	// GCP Project ID (required for tracing and logging)
	ProjectID string

	// Logger configuration
	LogName       string // Log name added as a field for filtering (defaults to ServiceName). Note: Actual GCP logName will be "stderr" when using structured logging to stderr.
	EnableConsole bool   // Enable console output
	EnableGCP     bool   // Enable GCP Cloud Logging
	LogLevel      logger.Level
	PrettyLog     bool // Pretty-print console logs

	// Telemetry configuration
	EnableTracing bool    // Enable OpenTelemetry tracing
	EnableMetrics bool    // Enable metrics (future)
	TraceRatio    float64 // Sampling ratio (0.0 to 1.0)

	// Custom attributes
	Attributes map[string]string
}

// SetDefaults sets reasonable defaults for the configuration
func (c *Config) SetDefaults() {
	if c.ServiceName == "" {
		c.ServiceName = "default-service"
	}
	if c.ServiceVersion == "" {
		c.ServiceVersion = "1.0.0"
	}
	if c.Environment == "" {
		c.Environment = "production"
	}
	if c.LogName == "" {
		c.LogName = c.ServiceName
	}
	if c.TraceRatio == 0 {
		c.TraceRatio = 0.1 // Default to 10% sampling
	}
	if c.LogLevel == 0 {
		c.LogLevel = logger.LevelInfo
	}
}

// Validate validates the configuration
func (c *Config) Validate() error {
	if c.ServiceName == "" {
		return fmt.Errorf("ServiceName is required")
	}
	if c.ProjectID == "" {
		return fmt.Errorf("ProjectID is required")
	}
	if c.TraceRatio < 0 || c.TraceRatio > 1 {
		return fmt.Errorf("TraceRatio must be between 0.0 and 1.0")
	}
	return nil
}

// Client provides a unified interface to the middleware stack
type Client struct {
	config    Config
	logger    *logger.Logger
	telemetry *telemetry.Provider
}

// NewClient creates and initializes a new middleware client
func NewClient(ctx context.Context, config Config) (*Client, error) {
	config.SetDefaults()
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	client := &Client{
		config: config,
	}

	// Initialize logger
	loggerConfig := logger.Config{
		ProjectID:      config.ProjectID,
		ServiceName:    config.ServiceName,
		ServiceVersion: config.ServiceVersion,
		LogName:        config.LogName,
		EnableConsole:  config.EnableConsole,
		EnableGCP:      config.EnableGCP,
		Level:          config.LogLevel,
		Pretty:         config.PrettyLog,
	}

	log, err := logger.NewLogger(ctx, loggerConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize logger: %w", err)
	}
	client.logger = log

	// Initialize global logger
	if err := logger.InitGlobal(ctx, loggerConfig); err != nil {
		return nil, fmt.Errorf("failed to initialize global logger: %w", err)
	}

	// Initialize telemetry if enabled
	if config.EnableTracing {
		telemetryConfig := telemetry.Config{
			ServiceName:    config.ServiceName,
			ServiceVersion: config.ServiceVersion,
			Environment:    config.Environment,
			ProjectID:      config.ProjectID,
			EnableTracing:  config.EnableTracing,
			EnableMetrics:  config.EnableMetrics,
			TraceRatio:     config.TraceRatio,
			Attributes:     config.Attributes,
		}

		provider, err := telemetry.NewProvider(ctx, telemetryConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize telemetry: %w", err)
		}
		client.telemetry = provider

		// Initialize global telemetry
		if err := telemetry.InitGlobal(ctx, telemetryConfig); err != nil {
			return nil, fmt.Errorf("failed to initialize global telemetry: %w", err)
		}
	}

	client.logger.InfoContext(ctx, "Middleware client initialized",
		"service", config.ServiceName,
		"version", config.ServiceVersion,
		"environment", config.Environment,
		"tracing_enabled", config.EnableTracing,
	)

	return client, nil
}

// Logger returns the logger instance
func (c *Client) Logger() *logger.Logger {
	return c.logger
}

// Telemetry returns the telemetry provider
func (c *Client) Telemetry() *telemetry.Provider {
	return c.telemetry
}

// Config returns the client configuration
func (c *Client) Config() Config {
	return c.config
}

// Shutdown gracefully shuts down the client
func (c *Client) Shutdown(ctx context.Context) error {
	c.logger.InfoContext(ctx, "Shutting down middleware client")

	// Shutdown telemetry first to flush traces
	if c.telemetry != nil {
		if err := c.telemetry.Shutdown(ctx); err != nil {
			c.logger.ErrorContext(ctx, "Failed to shutdown telemetry", "error", err)
			return fmt.Errorf("failed to shutdown telemetry: %w", err)
		}
	}

	// Shutdown logger
	if err := c.logger.Close(); err != nil {
		return fmt.Errorf("failed to close logger: %w", err)
	}

	return nil
}

// HTTPHandler creates a fully instrumented HTTP handler with all middleware
func (c *Client) HTTPHandler(handler http.HandlerFunc, operationName string) http.Handler {
	// Build middleware chain from outside to inside:
	// OTelHTTP -> RequestID -> Logging -> Recovery -> CORS -> Handler
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

// HTTPHandlerWithTimeout creates a fully instrumented HTTP handler with timeout
func (c *Client) HTTPHandlerWithTimeout(handler http.HandlerFunc, operationName string, timeout time.Duration) http.Handler {
	return OTelHTTP(operationName)(
		RequestID(
			Logging(
				Recovery(
					Timeout(timeout)(
						CORS(handler),
					),
				),
			),
		),
	)
}

// Handler creates a basic handler with just logging and recovery (no CORS)
func (c *Client) Handler(handler http.HandlerFunc, operationName string) http.Handler {
	return OTelHTTP(operationName)(
		RequestID(
			Logging(
				Recovery(handler),
			),
		),
	)
}

// MiddlewareChain creates a customizable middleware chain
type MiddlewareChain struct {
	middlewares []func(http.Handler) http.Handler
}

// NewChain creates a new middleware chain
func NewChain(middlewares ...func(http.Handler) http.Handler) *MiddlewareChain {
	return &MiddlewareChain{
		middlewares: middlewares,
	}
}

// Then applies the middleware chain to a handler
func (mc *MiddlewareChain) Then(handler http.Handler) http.Handler {
	// Apply middleware in reverse order so the first middleware wraps everything
	for i := len(mc.middlewares) - 1; i >= 0; i-- {
		handler = mc.middlewares[i](handler)
	}
	return handler
}

// ThenFunc applies the middleware chain to a handler function
func (mc *MiddlewareChain) ThenFunc(handlerFunc http.HandlerFunc) http.Handler {
	return mc.Then(handlerFunc)
}

// Append adds middleware to the end of the chain
func (mc *MiddlewareChain) Append(middlewares ...func(http.Handler) http.Handler) *MiddlewareChain {
	mc.middlewares = append(mc.middlewares, middlewares...)
	return mc
}

// StandardChain creates a standard middleware chain with common middleware
func (c *Client) StandardChain(operationName string) *MiddlewareChain {
	return NewChain(
		OTelHTTP(operationName),
		RequestID,
		Logging,
		Recovery,
		CORS,
	)
}

// APIChain creates an API-optimized middleware chain
func (c *Client) APIChain(operationName string, timeout time.Duration) *MiddlewareChain {
	chain := NewChain(
		OTelHTTP(operationName),
		RequestID,
		Logging,
		Recovery,
	)

	if timeout > 0 {
		chain.Append(Timeout(timeout))
	}

	chain.Append(CORS)

	return chain
}
