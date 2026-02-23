package logger

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"runtime"
	"runtime/debug"
	"sync"
	"time"

	"github.com/chainguard-dev/clog/gcp"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

var (
	globalLogger *Logger
	once         sync.Once
)

// Level represents logging levels
type Level slog.Level

// Log levels
const (
	LevelDebug    = Level(slog.LevelDebug)
	LevelInfo     = Level(slog.LevelInfo)
	LevelWarn     = Level(slog.LevelWarn)
	LevelError    = Level(slog.LevelError)
	LevelCritical = Level(gcp.LevelCritical)
)

// Config holds logger configuration
type Config struct {
	ProjectID      string
	ServiceName    string
	ServiceVersion string
	LogName        string // LogName added as a field in logs for filtering/categorization. NOTE: When using stderr output, GCP automatically sets the actual logName to "projects/{PROJECT_ID}/logs/stderr". To use custom log names in GCP, you must use the Cloud Logging API client library.
	EnableConsole  bool   // Enable console output to stdout
	EnableGCP      bool   // Enable GCP structured logging to stderr
	Level          Level
	Pretty         bool // Pretty-print console logs (text format vs JSON)
}

// SetDefaults sets reasonable defaults for the config
func (c *Config) SetDefaults() {
	if c.ServiceName == "" {
		c.ServiceName = "default-service"
	}
	if c.ServiceVersion == "" {
		c.ServiceVersion = "1.0.0"
	}
	if c.LogName == "" {
		c.LogName = c.ServiceName
	}
}

// Validate validates the configuration
func (c *Config) Validate() error {
	if c.EnableGCP && c.ProjectID == "" {
		return fmt.Errorf("ProjectID is required when GCP logging is enabled")
	}
	return nil
}

// Logger wraps slog with GCP structured logging support via clog/gcp
type Logger struct {
	logger *slog.Logger
	config Config
	mu     sync.RWMutex
}

// NewLogger creates a new logger instance with the provided config
func NewLogger(ctx context.Context, config Config) (*Logger, error) {
	config.SetDefaults()
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	handler := createHandler(config)

	logger := &Logger{
		logger: slog.New(handler),
		config: config,
	}

	return logger, nil
}

// createHandler creates the appropriate slog handler based on config
func createHandler(config Config) slog.Handler {
	var handlers []slog.Handler

	// Console handler (stdout)
	if config.EnableConsole {
		opts := &slog.HandlerOptions{Level: slog.Level(config.Level)}
		if config.Pretty {
			handlers = append(handlers, slog.NewTextHandler(os.Stdout, opts))
		} else {
			handlers = append(handlers, slog.NewJSONHandler(os.Stdout, opts))
		}
	}

	// GCP handler (stderr) - clog/gcp automatically formats for GCP Cloud Logging
	if config.EnableGCP {
		handlers = append(handlers, gcp.NewHandler(slog.Level(config.Level)))
	}

	// If no handlers enabled, use a discard handler
	if len(handlers) == 0 {
		return slog.NewTextHandler(io.Discard, nil)
	}

	// If only one handler, return it directly
	if len(handlers) == 1 {
		return handlers[0]
	}

	// Multiple handlers - use a multi-handler
	return &multiHandler{handlers: handlers}
}

// multiHandler writes to multiple handlers
type multiHandler struct {
	handlers []slog.Handler
}

func (m *multiHandler) Enabled(ctx context.Context, level slog.Level) bool {
	for _, h := range m.handlers {
		if h.Enabled(ctx, level) {
			return true
		}
	}
	return false
}

func (m *multiHandler) Handle(ctx context.Context, rec slog.Record) error {
	for _, h := range m.handlers {
		if h.Enabled(ctx, rec.Level) {
			if err := h.Handle(ctx, rec); err != nil {
				return err
			}
		}
	}
	return nil
}

func (m *multiHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	newHandlers := make([]slog.Handler, len(m.handlers))
	for i, h := range m.handlers {
		newHandlers[i] = h.WithAttrs(attrs)
	}
	return &multiHandler{handlers: newHandlers}
}

func (m *multiHandler) WithGroup(name string) slog.Handler {
	newHandlers := make([]slog.Handler, len(m.handlers))
	for i, h := range m.handlers {
		newHandlers[i] = h.WithGroup(name)
	}
	return &multiHandler{handlers: newHandlers}
}

// InitGlobal initializes the global logger
func InitGlobal(ctx context.Context, config Config) error {
	var err error
	once.Do(func() {
		globalLogger, err = NewLogger(ctx, config)
		if err == nil {
			slog.SetDefault(globalLogger.logger)
		}
	})
	return err
}

// Global returns the global logger instance
func Global() *Logger {
	if globalLogger == nil {
		return &Logger{logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
	}
	return globalLogger
}

// addTraceContext adds trace information from context to log attributes
func addTraceContext(ctx context.Context, projectID string, attrs []any) []any {
	span := trace.SpanFromContext(ctx)
	if span == nil || !span.SpanContext().IsValid() {
		return attrs
	}

	sc := span.SpanContext()
	traceID := sc.TraceID().String()
	spanID := sc.SpanID().String()

	// Add trace context for both console and GCP
	attrs = append(attrs,
		"trace_id", traceID,
		"span_id", spanID,
		"trace_sampled", sc.TraceFlags().IsSampled(),
	)

	// Add GCP-specific trace format
	if projectID != "" {
		traceStr := fmt.Sprintf("projects/%s/traces/%s", projectID, traceID)
		// Use clog/gcp's WithTrace to add trace to context for GCP handler
		ctx = gcp.WithTrace(ctx, traceStr)
		attrs = append(attrs,
			"logging.googleapis.com/trace", traceStr,
			"logging.googleapis.com/spanId", spanID,
			"logging.googleapis.com/trace_sampled", sc.TraceFlags().IsSampled(),
		)
	}

	return attrs
}

// Debug logs at debug level
func (l *Logger) Debug(msg string, args ...any) {
	l.logger.Debug(msg, args...)
}

// Info logs at info level
func (l *Logger) Info(msg string, args ...any) {
	l.logger.Info(msg, args...)
}

// Warn logs at warn level
func (l *Logger) Warn(msg string, args ...any) {
	l.logger.Warn(msg, args...)
}

// Error logs at error level
func (l *Logger) Error(msg string, args ...any) {
	l.logger.Error(msg, args...)
}

// Critical logs at critical level (GCP-specific)
func (l *Logger) Critical(msg string, args ...any) {
	l.logger.Log(context.Background(), slog.Level(LevelCritical), msg, args...)
}

// Fatal logs at error level and exits
func (l *Logger) Fatal(msg string, args ...any) {
	l.logger.Error(msg, args...)
	os.Exit(1)
}

// DebugContext logs at debug level with context
func (l *Logger) DebugContext(ctx context.Context, msg string, args ...any) {
	args = addTraceContext(ctx, l.config.ProjectID, args)
	ctx = l.enrichContext(ctx)
	l.logger.DebugContext(ctx, msg, args...)
}

// InfoContext logs at info level with context
func (l *Logger) InfoContext(ctx context.Context, msg string, args ...any) {
	args = addTraceContext(ctx, l.config.ProjectID, args)
	ctx = l.enrichContext(ctx)
	l.logger.InfoContext(ctx, msg, args...)
}

// WarnContext logs at warn level with context
func (l *Logger) WarnContext(ctx context.Context, msg string, args ...any) {
	args = addTraceContext(ctx, l.config.ProjectID, args)
	ctx = l.enrichContext(ctx)
	l.logger.WarnContext(ctx, msg, args...)
}

// ErrorContext logs at error level with context
func (l *Logger) ErrorContext(ctx context.Context, msg string, args ...any) {
	args = addTraceContext(ctx, l.config.ProjectID, args)
	ctx = l.enrichContext(ctx)
	l.logger.ErrorContext(ctx, msg, args...)
}

// CriticalContext logs at critical level with context
func (l *Logger) CriticalContext(ctx context.Context, msg string, args ...any) {
	args = addTraceContext(ctx, l.config.ProjectID, args)
	ctx = l.enrichContext(ctx)
	l.logger.Log(ctx, slog.Level(LevelCritical), msg, args...)
}

// enrichContext adds trace information to context for clog/gcp
func (l *Logger) enrichContext(ctx context.Context) context.Context {
	span := trace.SpanFromContext(ctx)
	if span == nil || !span.SpanContext().IsValid() {
		return ctx
	}

	sc := span.SpanContext()
	if l.config.ProjectID != "" {
		traceStr := fmt.Sprintf("projects/%s/traces/%s", l.config.ProjectID, sc.TraceID().String())
		ctx = gcp.WithTrace(ctx, traceStr)
	}

	return ctx
}

// DebugWithSpan logs at debug level with explicit span
func (l *Logger) DebugWithSpan(ctx context.Context, span trace.Span, msg string, args ...any) {
	l.addLogToSpan(ctx, span, LevelDebug, msg, nil, args...)
	l.DebugContext(ctx, msg, args...)
}

// InfoWithSpan logs at info level with explicit span
func (l *Logger) InfoWithSpan(ctx context.Context, span trace.Span, msg string, args ...any) {
	l.addLogToSpan(ctx, span, LevelInfo, msg, nil, args...)
	l.InfoContext(ctx, msg, args...)
}

// WarnWithSpan logs at warn level with explicit span
func (l *Logger) WarnWithSpan(ctx context.Context, span trace.Span, msg string, args ...any) {
	l.addLogToSpan(ctx, span, LevelWarn, msg, nil, args...)
	l.WarnContext(ctx, msg, args...)
}

// ErrorWithSpan logs at error level with explicit span
func (l *Logger) ErrorWithSpan(ctx context.Context, span trace.Span, msg string, args ...any) {
	l.addLogToSpan(ctx, span, LevelError, msg, nil, args...)
	l.ErrorContext(ctx, msg, args...)
}

// addLogToSpan adds log information to the span as an event
func (l *Logger) addLogToSpan(ctx context.Context, span trace.Span, level Level, msg string, err error, args ...any) {
	if span == nil || !span.SpanContext().IsValid() {
		return
	}

	// Build span attributes from log fields
	attrs := []attribute.KeyValue{
		attribute.String("log.severity", slog.Level(level).String()),
		attribute.String("log.message", msg),
	}

	// Add structured log fields as span attributes
	for i := 0; i < len(args); i += 2 {
		if i+1 < len(args) {
			if key, ok := args[i].(string); ok {
				if !isSystemLogField(key) {
					if strVal, ok := args[i+1].(string); ok {
						attrs = append(attrs, attribute.String("log."+key, strVal))
					} else {
						attrs = append(attrs, attribute.String("log."+key, fmt.Sprintf("%v", args[i+1])))
					}
				}
			}
		}
	}

	span.AddEvent("log", trace.WithAttributes(attrs...))

	// Mark span as error if this is an error log
	if level == LevelError || level == LevelCritical {
		if err != nil {
			span.RecordError(err, trace.WithAttributes(
				attribute.String("error.message", err.Error()),
				attribute.String("error.type", fmt.Sprintf("%T", err)),
			))
			span.SetStatus(codes.Error, msg)
		} else {
			span.SetStatus(codes.Error, msg)
		}
	}
}

// isSystemLogField checks if a field is a system logging field
func isSystemLogField(key string) bool {
	return key == "logging.googleapis.com/trace" ||
		key == "logging.googleapis.com/spanId" ||
		key == "logging.googleapis.com/trace_sampled" ||
		key == "trace_id" ||
		key == "span_id" ||
		key == "trace_sampled"
}

// With returns a new logger with the given attributes
func (l *Logger) With(args ...any) *Logger {
	l.mu.RLock()
	defer l.mu.RUnlock()

	return &Logger{
		logger: l.logger.With(args...),
		config: l.config,
	}
}

// WithLogName creates a new logger with a different log name in the log context
func (l *Logger) WithLogName(logName string) *Logger {
	l.mu.RLock()
	defer l.mu.RUnlock()

	newConfig := l.config
	newConfig.LogName = logName

	return &Logger{
		logger: l.logger.With("log_name", logName),
		config: newConfig,
	}
}

// WithTrace returns a logger bound to a specific trace context
func (l *Logger) WithTrace(ctx context.Context) *Logger {
	span := trace.SpanFromContext(ctx)
	if span == nil || !span.SpanContext().IsValid() {
		return l
	}

	sc := span.SpanContext()
	return l.With(
		"trace_id", sc.TraceID().String(),
		"span_id", sc.SpanID().String(),
	)
}

// LogHTTPRequest logs HTTP request details with structured fields
func (l *Logger) LogHTTPRequest(ctx context.Context, method, path string, statusCode int, duration time.Duration, args ...any) {
	level := LevelInfo
	msg := "HTTP request completed"

	// Set appropriate level and message based on status code
	if statusCode >= 500 {
		level = LevelError
		msg = "HTTP request failed"
	} else if statusCode >= 400 {
		level = LevelWarn
		msg = "HTTP request client error"
	}

	// Build log fields
	logArgs := []any{
		"http.method", method,
		"http.path", path,
		"http.status_code", statusCode,
		"duration_ms", duration.Milliseconds(),
	}

	// Add httpRequest field for GCP structured logging
	logArgs = append(logArgs, "httpRequest", map[string]any{
		"requestMethod": method,
		"requestUrl":    path,
		"status":        statusCode,
		"latency":       fmt.Sprintf("%fs", duration.Seconds()),
	})

	// Add additional fields
	logArgs = append(logArgs, args...)

	// Add trace context
	logArgs = addTraceContext(ctx, l.config.ProjectID, logArgs)
	ctx = l.enrichContext(ctx)

	// Log at appropriate level
	l.logger.Log(ctx, slog.Level(level), msg, logArgs...)
}

// LogError logs an error with stack trace and span correlation
func (l *Logger) LogError(ctx context.Context, err error, msg string, args ...any) {
	if err == nil {
		l.ErrorContext(ctx, msg, args...)
		return
	}

	// Add error fields
	errorArgs := append([]any{
		"error", err.Error(),
		"error.type", fmt.Sprintf("%T", err),
	}, args...)

	// Add trace context
	errorArgs = addTraceContext(ctx, l.config.ProjectID, errorArgs)
	ctx = l.enrichContext(ctx)

	// Record error on current span if available
	if span := trace.SpanFromContext(ctx); span != nil {
		span.RecordError(err, trace.WithAttributes(
			attribute.String("error.message", err.Error()),
			attribute.String("error.type", fmt.Sprintf("%T", err)),
		))
		span.SetStatus(codes.Error, msg)
	}

	l.logger.ErrorContext(ctx, msg, errorArgs...)
}

// LogErrorWithSpan logs an error with explicit span
func (l *Logger) LogErrorWithSpan(ctx context.Context, span trace.Span, err error, msg string, args ...any) {
	if err == nil {
		l.ErrorWithSpan(ctx, span, msg, args...)
		return
	}

	// Add error fields
	errorArgs := append([]any{
		"error", err.Error(),
		"error.type", fmt.Sprintf("%T", err),
	}, args...)

	// Record error on the provided span
	if span != nil {
		span.RecordError(err, trace.WithAttributes(
			attribute.String("error.message", err.Error()),
			attribute.String("error.type", fmt.Sprintf("%T", err)),
		))
		span.SetStatus(codes.Error, msg)
	}

	l.addLogToSpan(ctx, span, LevelError, msg, err, errorArgs...)

	// Add trace context
	errorArgs = addTraceContext(ctx, l.config.ProjectID, errorArgs)
	ctx = l.enrichContext(ctx)

	l.logger.ErrorContext(ctx, msg, errorArgs...)
}

// LogPanic recovers from panic and logs it with trace correlation
func (l *Logger) LogPanic(ctx context.Context) {
	if r := recover(); r != nil {
		stackTrace := string(debug.Stack())

		logArgs := []any{
			"panic.value", fmt.Sprintf("%v", r),
			"panic.stack_trace", stackTrace,
		}

		// Add trace context
		logArgs = addTraceContext(ctx, l.config.ProjectID, logArgs)
		ctx = l.enrichContext(ctx)

		// Record panic on current span if available
		if span := trace.SpanFromContext(ctx); span != nil {
			span.SetStatus(codes.Error, "panic recovered")
			span.RecordError(fmt.Errorf("panic: %v", r), trace.WithAttributes(
				attribute.String("panic.value", fmt.Sprintf("%v", r)),
				attribute.String("panic.stack_trace", stackTrace),
			))
		}

		l.logger.Log(ctx, slog.Level(LevelCritical), "Panic recovered", logArgs...)
		panic(r) // Re-panic after logging
	}
}

// SetLevel sets the log level
func (l *Logger) SetLevel(level Level) {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.config.Level = level
	handler := createHandler(l.config)
	l.logger = slog.New(handler)
}

// Close properly closes the logger resources
// With clog/gcp, there are no resources to close (just writing to stderr)
func (l *Logger) Close() error {
	// No resources to clean up with clog/gcp
	return nil
}

// Shutdown gracefully shuts down the logger
func Shutdown(ctx context.Context) error {
	if globalLogger != nil {
		return globalLogger.Close()
	}
	return nil
}

// captureSourceLocation captures the source location for logging
func captureSourceLocation(skip int) map[string]any {
	if pc, file, line, ok := runtime.Caller(skip); ok {
		var funcName string
		if fn := runtime.FuncForPC(pc); fn != nil {
			funcName = fn.Name()
		}
		return map[string]any{
			"file":     file,
			"line":     int64(line),
			"function": funcName,
		}
	}
	return nil
}

// Global function shortcuts using the global logger
func Debug(msg string, args ...any)    { Global().Debug(msg, args...) }
func Info(msg string, args ...any)     { Global().Info(msg, args...) }
func Warn(msg string, args ...any)     { Global().Warn(msg, args...) }
func Error(msg string, args ...any)    { Global().Error(msg, args...) }
func Critical(msg string, args ...any) { Global().Critical(msg, args...) }
func Fatal(msg string, args ...any)    { Global().Fatal(msg, args...) }

func DebugContext(ctx context.Context, msg string, args ...any) {
	Global().DebugContext(ctx, msg, args...)
}

func InfoContext(ctx context.Context, msg string, args ...any) {
	Global().InfoContext(ctx, msg, args...)
}

func WarnContext(ctx context.Context, msg string, args ...any) {
	Global().WarnContext(ctx, msg, args...)
}

func ErrorContext(ctx context.Context, msg string, args ...any) {
	Global().ErrorContext(ctx, msg, args...)
}

func CriticalContext(ctx context.Context, msg string, args ...any) {
	Global().CriticalContext(ctx, msg, args...)
}
