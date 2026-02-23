package telemetry

import (
	"context"
	"fmt"
	"os"
	"time"

	gcptrace "github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/trace"
	"go.opentelemetry.io/contrib/detectors/gcp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
	"go.opentelemetry.io/otel/trace"
)

// Config holds telemetry configuration
type Config struct {
	ServiceName    string
	ServiceVersion string
	Environment    string
	ProjectID      string
	EnableTracing  bool
	EnableMetrics  bool
	TraceRatio     float64
	EnableDebug    bool

	// Export configuration
	ExportTimeout time.Duration
	BatchTimeout  time.Duration
	MaxBatchSize  int
	MaxQueueSize  int

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
		c.Environment = os.Getenv("ENVIRONMENT")
		if c.Environment == "" {
			c.Environment = "production"
		}
	}
	if c.TraceRatio == 0 {
		c.TraceRatio = 0.1 // Default to 10% sampling
	}
	if c.ProjectID == "" {
		c.ProjectID = os.Getenv("GOOGLE_CLOUD_PROJECT")
		if c.ProjectID == "" {
			c.ProjectID = os.Getenv("GCP_PROJECT")
		}
	}
	if c.ExportTimeout == 0 {
		c.ExportTimeout = 30 * time.Second
	}
	if c.BatchTimeout == 0 {
		c.BatchTimeout = 5 * time.Second
	}
	if c.MaxBatchSize == 0 {
		c.MaxBatchSize = 512
	}
	if c.MaxQueueSize == 0 {
		c.MaxQueueSize = 2048
	}
	if c.Attributes == nil {
		c.Attributes = make(map[string]string)
	}
}

// Validate validates telemetry configuration
func (c *Config) Validate() error {
	if c.ServiceName == "" {
		return fmt.Errorf("ServiceName is required")
	}
	if c.EnableTracing && c.ProjectID == "" {
		return fmt.Errorf("ProjectID is required when tracing is enabled")
	}
	if c.TraceRatio < 0 || c.TraceRatio > 1 {
		return fmt.Errorf("TraceRatio must be between 0.0 and 1.0")
	}
	if c.MaxBatchSize <= 0 {
		return fmt.Errorf("MaxBatchSize must be positive")
	}
	if c.MaxQueueSize <= 0 {
		return fmt.Errorf("MaxQueueSize must be positive")
	}
	return nil
}

// Provider manages OpenTelemetry setup for Google Cloud
type Provider struct {
	tracerProvider *sdktrace.TracerProvider
	config         Config
	tracer         trace.Tracer
}

// NewProvider creates and configures OpenTelemetry for Google Cloud
func NewProvider(ctx context.Context, config Config) (*Provider, error) {
	config.SetDefaults()
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid telemetry config: %w", err)
	}

	provider := &Provider{
		config: config,
	}

	if config.EnableTracing {
		if err := provider.setupTracing(ctx); err != nil {
			return nil, fmt.Errorf("failed to setup tracing: %w", err)
		}
	}

	// Configure propagation for distributed tracing
	provider.setupPropagation()

	// Create default tracer
	provider.tracer = provider.GetTracer(config.ServiceName)

	return provider, nil
}

// setupTracing configures Google Cloud Trace
func (p *Provider) setupTracing(ctx context.Context) error {
	// Create resource with GCP detection
	res, err := p.createResource(ctx)
	if err != nil {
		return fmt.Errorf("failed to create resource: %w", err)
	}

	// Create Google Cloud Trace exporter
	exporter, err := gcptrace.New(
		gcptrace.WithProjectID(p.config.ProjectID),
		gcptrace.WithTimeout(p.config.ExportTimeout),
	)
	if err != nil {
		return fmt.Errorf("failed to create Google Cloud Trace exporter: %w", err)
	}

	// Configure sampler
	var sampler sdktrace.Sampler
	if p.config.TraceRatio >= 1.0 {
		sampler = sdktrace.AlwaysSample()
	} else if p.config.TraceRatio <= 0.0 {
		sampler = sdktrace.NeverSample()
	} else {
		sampler = sdktrace.TraceIDRatioBased(p.config.TraceRatio)
	}

	// Create tracer provider with batch processing
	p.tracerProvider = sdktrace.NewTracerProvider(
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sampler),
		sdktrace.WithBatcher(exporter,
			sdktrace.WithBatchTimeout(p.config.BatchTimeout),
			sdktrace.WithMaxExportBatchSize(p.config.MaxBatchSize),
			sdktrace.WithMaxQueueSize(p.config.MaxQueueSize),
		),
	)

	// Set as global tracer provider
	otel.SetTracerProvider(p.tracerProvider)

	return nil
}

// createResource creates resource with GCP detection
func (p *Provider) createResource(ctx context.Context) (*resource.Resource, error) {
	// Base service attributes
	baseAttrs := []attribute.KeyValue{
		semconv.ServiceName(p.config.ServiceName),
		semconv.ServiceVersion(p.config.ServiceVersion),
		semconv.DeploymentEnvironment(p.config.Environment),
	}

	// Add custom attributes
	for k, v := range p.config.Attributes {
		baseAttrs = append(baseAttrs, attribute.String(k, v))
	}

	baseRes := resource.NewWithAttributes(semconv.SchemaURL, baseAttrs...)

	// Detect GCP environment
	gcpDetector := gcp.NewDetector()
	gcpRes, err := gcpDetector.Detect(ctx)
	if err != nil {
		// GCP detection failed, use base resource
		if p.config.EnableDebug {
			fmt.Fprintf(os.Stderr, "GCP detection failed: %v\n", err)
		}
		return baseRes, nil
	}

	// Merge base and GCP resources
	return resource.Merge(baseRes, gcpRes)
}

// setupPropagation configures trace context propagation
func (p *Provider) setupPropagation() {
	otel.SetTextMapPropagator(
		propagation.NewCompositeTextMapPropagator(
			propagation.TraceContext{},
			propagation.Baggage{},
		),
	)
}

// Shutdown gracefully shuts down the tracer provider
func (p *Provider) Shutdown(ctx context.Context) error {
	if p.tracerProvider != nil {
		return p.tracerProvider.Shutdown(ctx)
	}
	return nil
}

// GetTracer returns a tracer with the given name
func (p *Provider) GetTracer(name string, opts ...trace.TracerOption) trace.Tracer {
	if p.tracerProvider != nil {
		return p.tracerProvider.Tracer(name, opts...)
	}
	return otel.Tracer(name, opts...)
}

// Tracer returns the default tracer for this provider
func (p *Provider) Tracer() trace.Tracer {
	return p.tracer
}

// TracerProvider returns the underlying tracer provider
func (p *Provider) TracerProvider() trace.TracerProvider {
	if p.tracerProvider != nil {
		return p.tracerProvider
	}
	return otel.GetTracerProvider()
}

// GetProjectID returns the configured project ID
func (p *Provider) GetProjectID() string {
	return p.config.ProjectID
}

// GetServiceName returns the configured service name
func (p *Provider) GetServiceName() string {
	return p.config.ServiceName
}

// Global provider instance
var globalProvider *Provider

// InitGlobal initializes the global telemetry provider
func InitGlobal(ctx context.Context, config Config) error {
	provider, err := NewProvider(ctx, config)
	if err != nil {
		return err
	}
	globalProvider = provider
	return nil
}

// Global returns the global telemetry provider
func Global() *Provider {
	return globalProvider
}

// ShutdownGlobal shuts down the global telemetry provider
func ShutdownGlobal(ctx context.Context) error {
	if globalProvider != nil {
		return globalProvider.Shutdown(ctx)
	}
	return nil
}

// GetGlobalTracer returns a tracer from the global provider
func GetGlobalTracer(name string, opts ...trace.TracerOption) trace.Tracer {
	if globalProvider != nil {
		return globalProvider.GetTracer(name, opts...)
	}
	return otel.Tracer(name, opts...)
}

// GetGlobalProjectID returns the project ID from the global provider
func GetGlobalProjectID() string {
	if globalProvider != nil {
		return globalProvider.GetProjectID()
	}
	return ""
}
