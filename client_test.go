package middleware

import (
	"context"
	"net/http"
	"testing"

	"github.com/sirch/go-gcp-middleware/logger"
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
				LogName:        "default-service",
				TraceRatio:     0.1,
				LogLevel:       logger.LevelInfo,
			},
		},
		{
			name: "partial config preserves values",
			config: Config{
				ServiceName: "custom-service",
			},
			want: Config{
				ServiceName:    "custom-service",
				ServiceVersion: "1.0.0",
				Environment:    "production",
				LogName:        "custom-service",
				TraceRatio:     0.1,
				LogLevel:       logger.LevelInfo,
			},
		},
		{
			name: "custom log name preserved",
			config: Config{
				ServiceName: "service",
				LogName:     "custom-log",
			},
			want: Config{
				ServiceName:    "service",
				ServiceVersion: "1.0.0",
				Environment:    "production",
				LogName:        "custom-log",
				TraceRatio:     0.1,
				LogLevel:       logger.LevelInfo,
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
			if config.LogName != tt.want.LogName {
				t.Errorf("LogName = %v, want %v", config.LogName, tt.want.LogName)
			}
			if config.TraceRatio != tt.want.TraceRatio {
				t.Errorf("TraceRatio = %v, want %v", config.TraceRatio, tt.want.TraceRatio)
			}
		})
	}
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
			name: "missing project id",
			config: Config{
				ServiceName: "test-service",
				TraceRatio:  0.5,
			},
			wantErr: true,
		},
		{
			name: "invalid trace ratio - negative",
			config: Config{
				ServiceName: "test-service",
				ProjectID:   "test-project",
				TraceRatio:  -0.1,
			},
			wantErr: true,
		},
		{
			name: "invalid trace ratio - over 1",
			config: Config{
				ServiceName: "test-service",
				ProjectID:   "test-project",
				TraceRatio:  1.5,
			},
			wantErr: true,
		},
		{
			name: "trace ratio 0 is valid",
			config: Config{
				ServiceName: "test-service",
				ProjectID:   "test-project",
				TraceRatio:  0,
			},
			wantErr: false,
		},
		{
			name: "trace ratio 1 is valid",
			config: Config{
				ServiceName: "test-service",
				ProjectID:   "test-project",
				TraceRatio:  1.0,
			},
			wantErr: false,
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

func TestNewClient(t *testing.T) {
	// Note: This test requires GCP credentials to fully pass
	// In CI, you may want to skip this test or use mocks

	t.Run("valid config creates client", func(t *testing.T) {
		config := Config{
			ServiceName:   "test-service",
			ProjectID:     "test-project",
			EnableConsole: true,
			EnableGCP:     false, // Disable GCP to avoid needing credentials
			LogLevel:      logger.LevelInfo,
			EnableTracing: false, // Disable tracing to avoid needing credentials
		}

		client, err := NewClient(context.Background(), config)
		if err != nil {
			t.Fatalf("NewClient() error = %v", err)
		}

		if client == nil {
			t.Fatal("NewClient() returned nil client")
		}

		if client.Logger() == nil {
			t.Error("Client logger is nil")
		}

		// Shutdown
		ctx := context.Background()
		if err := client.Shutdown(ctx); err != nil {
			t.Errorf("Shutdown() error = %v", err)
		}
	})

	t.Run("invalid config returns error", func(t *testing.T) {
		config := Config{
			// Missing required fields
			ServiceName: "test",
			// Missing ProjectID
		}

		_, err := NewClient(context.Background(), config)
		if err == nil {
			t.Error("NewClient() expected error for invalid config")
		}
	})
}

func TestMiddlewareChain(t *testing.T) {
	t.Run("create empty chain", func(t *testing.T) {
		chain := NewChain()
		if chain == nil {
			t.Fatal("NewChain() returned nil")
		}
		if len(chain.middlewares) != 0 {
			t.Errorf("Expected empty chain, got %d middlewares", len(chain.middlewares))
		}
	})

	t.Run("create chain with middleware", func(t *testing.T) {
		mw1 := func(next http.Handler) http.Handler { return next }
		mw2 := func(next http.Handler) http.Handler { return next }

		chain := NewChain(mw1, mw2)
		if len(chain.middlewares) != 2 {
			t.Errorf("Expected 2 middlewares, got %d", len(chain.middlewares))
		}
	})

	t.Run("append middleware", func(t *testing.T) {
		mw1 := func(next http.Handler) http.Handler { return next }
		mw2 := func(next http.Handler) http.Handler { return next }

		chain := NewChain(mw1)
		chain.Append(mw2)

		if len(chain.middlewares) != 2 {
			t.Errorf("Expected 2 middlewares after append, got %d", len(chain.middlewares))
		}
	})
}

// Benchmark tests
func BenchmarkNewClient(b *testing.B) {
	config := Config{
		ServiceName:   "bench-service",
		ProjectID:     "bench-project",
		EnableConsole: true,
		EnableGCP:     false,
		EnableTracing: false,
	}

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		client, err := NewClient(ctx, config)
		if err != nil {
			b.Fatal(err)
		}
		client.Shutdown(ctx)
	}
}

func BenchmarkConfigSetDefaults(b *testing.B) {
	for i := 0; i < b.N; i++ {
		config := Config{}
		config.SetDefaults()
	}
}

func BenchmarkConfigValidate(b *testing.B) {
	config := Config{
		ServiceName: "test",
		ProjectID:   "test-project",
		TraceRatio:  0.5,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = config.Validate()
	}
}
