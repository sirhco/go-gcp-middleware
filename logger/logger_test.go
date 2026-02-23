package logger

import (
	"context"
	"testing"

	"go.opentelemetry.io/otel/trace"
)

func TestNewLogger(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name    string
		config  Config
		wantErr bool
	}{
		{
			name: "valid config with console only",
			config: Config{
				ProjectID:     "test-project",
				ServiceName:   "test-service",
				EnableConsole: true,
				EnableGCP:     false,
				Level:         LevelInfo,
			},
			wantErr: false,
		},
		{
			name: "valid config with GCP only",
			config: Config{
				ProjectID:     "test-project",
				ServiceName:   "test-service",
				EnableConsole: false,
				EnableGCP:     true,
				Level:         LevelInfo,
			},
			wantErr: false,
		},
		{
			name: "valid config with both handlers",
			config: Config{
				ProjectID:     "test-project",
				ServiceName:   "test-service",
				EnableConsole: true,
				EnableGCP:     true,
				Level:         LevelDebug,
			},
			wantErr: false,
		},
		{
			name: "valid config with neither handler",
			config: Config{
				ProjectID:     "test-project",
				ServiceName:   "test-service",
				EnableConsole: false,
				EnableGCP:     false,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			log, err := NewLogger(ctx, tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewLogger() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if log == nil && !tt.wantErr {
				t.Error("NewLogger() returned nil logger")
			}
			if log != nil {
				defer log.Close()
			}
		})
	}
}

func TestLoggerMethods(t *testing.T) {
	ctx := context.Background()

	log, err := NewLogger(ctx, Config{
		ProjectID:     "test-project",
		ServiceName:   "test-service",
		EnableConsole: false, // Disable output for tests
		EnableGCP:     false,
		Level:         LevelDebug,
	})
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer log.Close()

	// Test basic logging methods
	t.Run("Debug", func(t *testing.T) {
		log.Debug("debug message", "key", "value")
	})

	t.Run("Info", func(t *testing.T) {
		log.Info("info message", "key", "value")
	})

	t.Run("Warn", func(t *testing.T) {
		log.Warn("warn message", "key", "value")
	})

	t.Run("Error", func(t *testing.T) {
		log.Error("error message", "key", "value")
	})

	t.Run("Critical", func(t *testing.T) {
		log.Critical("critical message", "key", "value")
	})
}

func TestLoggerContextMethods(t *testing.T) {
	ctx := context.Background()

	log, err := NewLogger(ctx, Config{
		ProjectID:     "test-project",
		ServiceName:   "test-service",
		EnableConsole: false,
		EnableGCP:     false,
		Level:         LevelDebug,
	})
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer log.Close()

	t.Run("DebugContext", func(t *testing.T) {
		log.DebugContext(ctx, "debug message", "key", "value")
	})

	t.Run("InfoContext", func(t *testing.T) {
		log.InfoContext(ctx, "info message", "key", "value")
	})

	t.Run("WarnContext", func(t *testing.T) {
		log.WarnContext(ctx, "warn message", "key", "value")
	})

	t.Run("ErrorContext", func(t *testing.T) {
		log.ErrorContext(ctx, "error message", "key", "value")
	})

	t.Run("CriticalContext", func(t *testing.T) {
		log.CriticalContext(ctx, "critical message", "key", "value")
	})
}

func TestLoggerWith(t *testing.T) {
	ctx := context.Background()

	log, err := NewLogger(ctx, Config{
		ProjectID:     "test-project",
		ServiceName:   "test-service",
		EnableConsole: false,
		EnableGCP:     false,
		Level:         LevelInfo,
	})
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer log.Close()

	t.Run("With", func(t *testing.T) {
		childLog := log.With("component", "database", "version", "1.0")
		if childLog == nil {
			t.Error("With() returned nil logger")
		}
		childLog.Info("test message")
	})

	t.Run("WithLogName", func(t *testing.T) {
		childLog := log.WithLogName("custom-component")
		if childLog == nil {
			t.Error("WithLogName() returned nil logger")
		}
		if childLog.config.LogName != "custom-component" {
			t.Errorf("WithLogName() LogName = %v, want custom-component", childLog.config.LogName)
		}
		childLog.Info("test message")
	})

	t.Run("WithTrace", func(t *testing.T) {
		// Create a mock span context
		traceID, _ := trace.TraceIDFromHex("4bf92f3577b34da6a3ce929d0e0e4736")
		spanID, _ := trace.SpanIDFromHex("00f067aa0ba902b7")
		spanContext := trace.NewSpanContext(trace.SpanContextConfig{
			TraceID:    traceID,
			SpanID:     spanID,
			TraceFlags: trace.FlagsSampled,
		})

		// Create context with span
		ctx := trace.ContextWithSpanContext(ctx, spanContext)

		childLog := log.WithTrace(ctx)
		if childLog == nil {
			t.Error("WithTrace() returned nil logger")
		}
		childLog.Info("test message with trace")
	})
}

func TestLogHTTPRequest(t *testing.T) {
	ctx := context.Background()

	log, err := NewLogger(ctx, Config{
		ProjectID:     "test-project",
		ServiceName:   "test-service",
		EnableConsole: false,
		EnableGCP:     false,
		Level:         LevelInfo,
	})
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer log.Close()

	tests := []struct {
		name       string
		method     string
		path       string
		statusCode int
	}{
		{"success", "GET", "/api/users", 200},
		{"client error", "POST", "/api/users", 400},
		{"server error", "GET", "/api/users", 500},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			log.LogHTTPRequest(ctx, tt.method, tt.path, tt.statusCode, 0,
				"user_agent", "test-agent",
			)
		})
	}
}

func TestLogError(t *testing.T) {
	ctx := context.Background()

	log, err := NewLogger(ctx, Config{
		ProjectID:     "test-project",
		ServiceName:   "test-service",
		EnableConsole: false,
		EnableGCP:     false,
		Level:         LevelInfo,
	})
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer log.Close()

	t.Run("with error", func(t *testing.T) {
		testErr := context.DeadlineExceeded
		log.LogError(ctx, testErr, "operation failed", "operation", "test")
	})

	t.Run("without error", func(t *testing.T) {
		log.LogError(ctx, nil, "operation failed", "operation", "test")
	})
}

func TestSetLevel(t *testing.T) {
	ctx := context.Background()

	log, err := NewLogger(ctx, Config{
		ProjectID:     "test-project",
		ServiceName:   "test-service",
		EnableConsole: false,
		EnableGCP:     false,
		Level:         LevelInfo,
	})
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer log.Close()

	log.SetLevel(LevelDebug)
	if log.config.Level != LevelDebug {
		t.Errorf("SetLevel() Level = %v, want %v", log.config.Level, LevelDebug)
	}

	log.SetLevel(LevelError)
	if log.config.Level != LevelError {
		t.Errorf("SetLevel() Level = %v, want %v", log.config.Level, LevelError)
	}
}

func TestGlobalLogger(t *testing.T) {
	ctx := context.Background()

	err := InitGlobal(ctx, Config{
		ProjectID:     "test-project",
		ServiceName:   "test-service",
		EnableConsole: false,
		EnableGCP:     false,
		Level:         LevelInfo,
	})
	if err != nil {
		t.Fatalf("Failed to initialize global logger: %v", err)
	}

	global := Global()
	if global == nil {
		t.Error("Global() returned nil logger")
	}

	// Test global functions
	Info("test info message", "key", "value")
	InfoContext(ctx, "test info context message", "key", "value")

	// Cleanup
	if err := Shutdown(ctx); err != nil {
		t.Errorf("Shutdown() error = %v", err)
	}
}

func TestConfigSetDefaults(t *testing.T) {
	config := Config{}
	config.SetDefaults()

	if config.ServiceName == "" {
		t.Error("SetDefaults() did not set ServiceName")
	}
	if config.ServiceVersion == "" {
		t.Error("SetDefaults() did not set ServiceVersion")
	}
	if config.LogName == "" {
		t.Error("SetDefaults() did not set LogName")
	}
}

func TestConfigValidate(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr bool
	}{
		{
			name: "valid config",
			config: Config{
				ProjectID:   "test-project",
				ServiceName: "test-service",
				EnableGCP:   true,
			},
			wantErr: false,
		},
		{
			name: "missing ProjectID with GCP enabled",
			config: Config{
				ServiceName: "test-service",
				EnableGCP:   true,
			},
			wantErr: true,
		},
		{
			name: "missing ProjectID with GCP disabled",
			config: Config{
				ServiceName:   "test-service",
				EnableConsole: true,
				EnableGCP:     false,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.config.SetDefaults()
			err := tt.config.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
