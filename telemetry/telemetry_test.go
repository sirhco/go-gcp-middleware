package telemetry

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestConfig_SetDefaults(t *testing.T) {
	tests := []struct {
		name   string
		config Config
		want   Config
	}{
		{
			name:   "empty config gets defaults",
			config: Config{},
			want: Config{
				ServiceName:    "default-service",
				ServiceVersion: "1.0.0",
				Environment:    "production",
				TraceRatio:     0.1,
				ExportTimeout:  30 * time.Second,
				BatchTimeout:   5 * time.Second,
				MaxBatchSize:   512,
				MaxQueueSize:   2048,
				Attributes:     make(map[string]string),
			},
		},
		{
			name: "partial config preserves values",
			config: Config{
				ServiceName: "custom-service",
				TraceRatio:  0.5,
			},
			want: Config{
				ServiceName:    "custom-service",
				ServiceVersion: "1.0.0",
				Environment:    "production",
				TraceRatio:     0.5,
				ExportTimeout:  30 * time.Second,
				BatchTimeout:   5 * time.Second,
				MaxBatchSize:   512,
				MaxQueueSize:   2048,
				Attributes:     make(map[string]string),
			},
		},
		{
			name: "custom values preserved",
			config: Config{
				ServiceName:    "service",
				ServiceVersion: "2.0.0",
				Environment:    "staging",
				TraceRatio:     1.0,
				ExportTimeout:  60 * time.Second,
			},
			want: Config{
				ServiceName:    "service",
				ServiceVersion: "2.0.0",
				Environment:    "staging",
				TraceRatio:     1.0,
				ExportTimeout:  60 * time.Second,
				BatchTimeout:   5 * time.Second,
				MaxBatchSize:   512,
				MaxQueueSize:   2048,
				Attributes:     make(map[string]string),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := tt.config
			config.SetDefaults()

			if config.ServiceName != tt.want.ServiceName {
				t.Errorf("ServiceName = %v, want %v", config.ServiceName, tt.want.ServiceName)
			}
			if config.ServiceVersion != tt.want.ServiceVersion {
				t.Errorf("ServiceVersion = %v, want %v", config.ServiceVersion, tt.want.ServiceVersion)
			}
			if config.Environment != tt.want.Environment {
				t.Errorf("Environment = %v, want %v", config.Environment, tt.want.Environment)
			}
			if config.TraceRatio != tt.want.TraceRatio {
				t.Errorf("TraceRatio = %v, want %v", config.TraceRatio, tt.want.TraceRatio)
			}
		})
	}
}

func TestConfig_SetDefaultsWithEnv(t *testing.T) {
	// Save original env
	origEnv := os.Getenv("ENVIRONMENT")
	origProject := os.Getenv("GOOGLE_CLOUD_PROJECT")
	defer func() {
		os.Setenv("ENVIRONMENT", origEnv)
		os.Setenv("GOOGLE_CLOUD_PROJECT", origProject)
	}()

	t.Run("reads ENVIRONMENT from env", func(t *testing.T) {
		os.Setenv("ENVIRONMENT", "development")
		config := Config{}
		config.SetDefaults()

		if config.Environment != "development" {
			t.Errorf("Expected environment 'development', got %s", config.Environment)
		}
	})

	t.Run("reads GOOGLE_CLOUD_PROJECT from env", func(t *testing.T) {
		os.Setenv("GOOGLE_CLOUD_PROJECT", "test-project")
		config := Config{}
		config.SetDefaults()

		if config.ProjectID != "test-project" {
			t.Errorf("Expected project ID 'test-project', got %s", config.ProjectID)
		}
	})
}

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr bool
	}{
		{
			name: "valid config",
			config: Config{
				ServiceName: "test-service",
				ProjectID:   "test-project",
				TraceRatio:  0.5,
				MaxBatchSize: 512,
				MaxQueueSize: 2048,
			},
			wantErr: false,
		},
		{
			name: "missing service name",
			config: Config{
				ProjectID:  "test-project",
				TraceRatio: 0.5,
			},
			wantErr: true,
		},
		{
			name: "missing project ID with tracing enabled",
			config: Config{
				ServiceName:   "test",
				EnableTracing: true,
				TraceRatio:    0.5,
			},
			wantErr: true,
		},
		{
			name: "invalid trace ratio - negative",
			config: Config{
				ServiceName: "test",
				ProjectID:   "test-project",
				TraceRatio:  -0.1,
			},
			wantErr: true,
		},
		{
			name: "invalid trace ratio - over 1",
			config: Config{
				ServiceName: "test",
				ProjectID:   "test-project",
				TraceRatio:  1.5,
			},
			wantErr: true,
		},
		{
			name: "invalid batch size",
			config: Config{
				ServiceName:  "test",
				ProjectID:    "test-project",
				TraceRatio:   0.5,
				MaxBatchSize: 0,
			},
			wantErr: true,
		},
		{
			name: "invalid queue size",
			config: Config{
				ServiceName:  "test",
				ProjectID:    "test-project",
				TraceRatio:   0.5,
				MaxBatchSize: 512,
				MaxQueueSize: -1,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestNewProvider(t *testing.T) {
	ctx := context.Background()

	t.Run("creates provider without tracing", func(t *testing.T) {
		config := Config{
			ServiceName:   "test-service",
			ProjectID:     "test-project",
			EnableTracing: false,
		}

		provider, err := NewProvider(ctx, config)
		if err != nil {
			t.Fatalf("NewProvider() error = %v", err)
		}

		if provider == nil {
			t.Fatal("Expected non-nil provider")
		}

		if provider.GetServiceName() != "test-service" {
			t.Errorf("Expected service name 'test-service', got %s", provider.GetServiceName())
		}

		if provider.GetProjectID() != "test-project" {
			t.Errorf("Expected project ID 'test-project', got %s", provider.GetProjectID())
		}
	})

	t.Run("invalid config returns error", func(t *testing.T) {
		config := Config{
			ServiceName:   "test",
			EnableTracing: true,
			// Missing ProjectID when tracing is enabled
		}

		_, err := NewProvider(ctx, config)
		if err == nil {
			t.Error("Expected error for invalid config")
		}
	})
}

func TestProviderGetTracer(t *testing.T) {
	ctx := context.Background()

	config := Config{
		ServiceName:   "test-service",
		ProjectID:     "test-project",
		EnableTracing: false,
	}

	provider, err := NewProvider(ctx, config)
	if err != nil {
		t.Fatalf("NewProvider() error = %v", err)
	}

	tracer := provider.GetTracer("test-tracer")
	if tracer == nil {
		t.Error("Expected non-nil tracer")
	}
}

func TestProviderTracer(t *testing.T) {
	ctx := context.Background()

	config := Config{
		ServiceName:   "test-service",
		ProjectID:     "test-project",
		EnableTracing: false,
	}

	provider, err := NewProvider(ctx, config)
	if err != nil {
		t.Fatalf("NewProvider() error = %v", err)
	}

	tracer := provider.Tracer()
	if tracer == nil {
		t.Error("Expected non-nil default tracer")
	}
}

func TestProviderTracerProvider(t *testing.T) {
	ctx := context.Background()

	config := Config{
		ServiceName:   "test-service",
		ProjectID:     "test-project",
		EnableTracing: false,
	}

	provider, err := NewProvider(ctx, config)
	if err != nil {
		t.Fatalf("NewProvider() error = %v", err)
	}

	tracerProvider := provider.TracerProvider()
	if tracerProvider == nil {
		t.Error("Expected non-nil tracer provider")
	}
}

func TestProviderShutdown(t *testing.T) {
	ctx := context.Background()

	config := Config{
		ServiceName:   "test-service",
		ProjectID:     "test-project",
		EnableTracing: false,
	}

	provider, err := NewProvider(ctx, config)
	if err != nil {
		t.Fatalf("NewProvider() error = %v", err)
	}

	err = provider.Shutdown(ctx)
	if err != nil {
		t.Errorf("Shutdown() error = %v", err)
	}
}

func TestGlobalProvider(t *testing.T) {
	ctx := context.Background()

	t.Run("InitGlobal sets global provider", func(t *testing.T) {
		config := Config{
			ServiceName:   "test-service",
			ProjectID:     "test-project",
			EnableTracing: false,
		}

		err := InitGlobal(ctx, config)
		if err != nil {
			t.Fatalf("InitGlobal() error = %v", err)
		}

		provider := Global()
		if provider == nil {
			t.Error("Expected non-nil global provider")
		}

		// Cleanup
		ShutdownGlobal(ctx)
	})

	t.Run("GetGlobalTracer returns tracer", func(t *testing.T) {
		config := Config{
			ServiceName:   "test-service",
			ProjectID:     "test-project",
			EnableTracing: false,
		}

		InitGlobal(ctx, config)
		defer ShutdownGlobal(ctx)

		tracer := GetGlobalTracer("test")
		if tracer == nil {
			t.Error("Expected non-nil tracer")
		}
	})

	t.Run("GetGlobalProjectID returns project ID", func(t *testing.T) {
		config := Config{
			ServiceName:   "test-service",
			ProjectID:     "test-project-123",
			EnableTracing: false,
		}

		InitGlobal(ctx, config)
		defer ShutdownGlobal(ctx)

		projectID := GetGlobalProjectID()
		if projectID != "test-project-123" {
			t.Errorf("Expected project ID 'test-project-123', got %s", projectID)
		}
	})
}

func TestProviderWithCustomAttributes(t *testing.T) {
	ctx := context.Background()

	config := Config{
		ServiceName:   "test-service",
		ProjectID:     "test-project",
		EnableTracing: false,
		Attributes: map[string]string{
			"team":   "platform",
			"region": "us-central1",
		},
	}

	provider, err := NewProvider(ctx, config)
	if err != nil {
		t.Fatalf("NewProvider() error = %v", err)
	}

	if provider == nil {
		t.Fatal("Expected non-nil provider")
	}
}

// Benchmark tests

func BenchmarkNewProvider(b *testing.B) {
	ctx := context.Background()
	config := Config{
		ServiceName:   "bench-service",
		ProjectID:     "bench-project",
		EnableTracing: false,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		provider, err := NewProvider(ctx, config)
		if err != nil {
			b.Fatal(err)
		}
		provider.Shutdown(ctx)
	}
}

func BenchmarkGetTracer(b *testing.B) {
	ctx := context.Background()
	config := Config{
		ServiceName:   "bench-service",
		ProjectID:     "bench-project",
		EnableTracing: false,
	}

	provider, _ := NewProvider(ctx, config)
	defer provider.Shutdown(ctx)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = provider.GetTracer("test")
	}
}
