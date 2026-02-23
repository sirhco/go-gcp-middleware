package telemetry

import (
	"context"
	"fmt"
	"runtime"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// SpanOptions contains options for creating spans
type SpanOptions struct {
	Attributes []attribute.KeyValue
	Kind       trace.SpanKind
	Links      []trace.Link
}

// StartSpan is a helper function to start a new span with common attributes
func StartSpan(ctx context.Context, tracerName, spanName string, opts ...SpanOptions) (context.Context, trace.Span) {
	tracer := otel.Tracer(tracerName)

	// Default options
	var options []trace.SpanStartOption
	options = append(options, trace.WithSpanKind(trace.SpanKindInternal))

	// Add caller information
	if pc, file, line, ok := runtime.Caller(1); ok {
		if fn := runtime.FuncForPC(pc); fn != nil {
			options = append(options, trace.WithAttributes(
				attribute.String("code.function", fn.Name()),
				attribute.String("code.filepath", file),
				attribute.Int("code.lineno", line),
			))
		}
	}

	// Apply custom options
	if len(opts) > 0 {
		opt := opts[0]
		if len(opt.Attributes) > 0 {
			options = append(options, trace.WithAttributes(opt.Attributes...))
		}
		if opt.Kind != trace.SpanKindUnspecified {
			options = append(options, trace.WithSpanKind(opt.Kind))
		}
		if len(opt.Links) > 0 {
			options = append(options, trace.WithLinks(opt.Links...))
		}
	}

	return tracer.Start(ctx, spanName, options...)
}

// StartSpanFromContext starts a span using the tracer from the global provider
func StartSpanFromContext(ctx context.Context, spanName string, opts ...SpanOptions) (context.Context, trace.Span) {
	tracerName := "default"
	if globalProvider != nil {
		tracerName = globalProvider.config.ServiceName
	}
	return StartSpan(ctx, tracerName, spanName, opts...)
}

// StartServerSpan starts a span for a server request
func StartServerSpan(ctx context.Context, spanName string, opts ...SpanOptions) (context.Context, trace.Span) {
	if len(opts) > 0 {
		opts[0].Kind = trace.SpanKindServer
	} else {
		opts = []SpanOptions{{Kind: trace.SpanKindServer}}
	}
	return StartSpanFromContext(ctx, spanName, opts...)
}

// StartClientSpan starts a span for a client request
func StartClientSpan(ctx context.Context, spanName string, opts ...SpanOptions) (context.Context, trace.Span) {
	if len(opts) > 0 {
		opts[0].Kind = trace.SpanKindClient
	} else {
		opts = []SpanOptions{{Kind: trace.SpanKindClient}}
	}
	return StartSpanFromContext(ctx, spanName, opts...)
}

// StartProducerSpan starts a span for a message producer
func StartProducerSpan(ctx context.Context, spanName string, opts ...SpanOptions) (context.Context, trace.Span) {
	if len(opts) > 0 {
		opts[0].Kind = trace.SpanKindProducer
	} else {
		opts = []SpanOptions{{Kind: trace.SpanKindProducer}}
	}
	return StartSpanFromContext(ctx, spanName, opts...)
}

// StartConsumerSpan starts a span for a message consumer
func StartConsumerSpan(ctx context.Context, spanName string, opts ...SpanOptions) (context.Context, trace.Span) {
	if len(opts) > 0 {
		opts[0].Kind = trace.SpanKindConsumer
	} else {
		opts = []SpanOptions{{Kind: trace.SpanKindConsumer}}
	}
	return StartSpanFromContext(ctx, spanName, opts...)
}

// RecordError records an error on the span with additional context
func RecordError(span trace.Span, err error, description string, attrs ...attribute.KeyValue) {
	if span == nil || err == nil {
		return
	}

	// Mark span as error
	span.SetStatus(codes.Error, description)

	// Record the error
	allAttrs := []attribute.KeyValue{
		attribute.String("error.type", fmt.Sprintf("%T", err)),
		attribute.String("error.message", err.Error()),
	}
	allAttrs = append(allAttrs, attrs...)

	span.RecordError(err, trace.WithAttributes(allAttrs...))
}

// RecordErrorContext records an error on the current span from context
func RecordErrorContext(ctx context.Context, err error, description string, attrs ...attribute.KeyValue) {
	span := trace.SpanFromContext(ctx)
	RecordError(span, err, description, attrs...)
}

// AddSpanAttributes adds attributes to a span safely
func AddSpanAttributes(span trace.Span, attrs ...attribute.KeyValue) {
	if span != nil && len(attrs) > 0 {
		span.SetAttributes(attrs...)
	}
}

// AddSpanAttributesContext adds attributes to the current span from context
func AddSpanAttributesContext(ctx context.Context, attrs ...attribute.KeyValue) {
	span := trace.SpanFromContext(ctx)
	AddSpanAttributes(span, attrs...)
}

// AddSpanEvent adds an event to a span with timestamp
func AddSpanEvent(span trace.Span, name string, attrs ...attribute.KeyValue) {
	if span != nil {
		span.AddEvent(name, trace.WithAttributes(attrs...), trace.WithTimestamp(time.Now()))
	}
}

// AddSpanEventContext adds an event to the current span from context
func AddSpanEventContext(ctx context.Context, name string, attrs ...attribute.KeyValue) {
	span := trace.SpanFromContext(ctx)
	AddSpanEvent(span, name, attrs...)
}

// SetSpanStatus sets the status of a span
func SetSpanStatus(span trace.Span, code codes.Code, description string) {
	if span != nil {
		span.SetStatus(code, description)
	}
}

// SetSpanStatusContext sets the status of the current span from context
func SetSpanStatusContext(ctx context.Context, code codes.Code, description string) {
	span := trace.SpanFromContext(ctx)
	SetSpanStatus(span, code, description)
}

// WithSpanAttribute is a helper to create attribute key-value pairs
func WithSpanAttribute(key, value string) attribute.KeyValue {
	return attribute.String(key, value)
}

// WithSpanIntAttribute is a helper to create integer attribute key-value pairs
func WithSpanIntAttribute(key string, value int) attribute.KeyValue {
	return attribute.Int(key, value)
}

// WithSpanInt64Attribute is a helper to create int64 attribute key-value pairs
func WithSpanInt64Attribute(key string, value int64) attribute.KeyValue {
	return attribute.Int64(key, value)
}

// WithSpanBoolAttribute is a helper to create boolean attribute key-value pairs
func WithSpanBoolAttribute(key string, value bool) attribute.KeyValue {
	return attribute.Bool(key, value)
}

// WithSpanFloat64Attribute is a helper to create float64 attribute key-value pairs
func WithSpanFloat64Attribute(key string, value float64) attribute.KeyValue {
	return attribute.Float64(key, value)
}

// TraceHTTPRequest creates common HTTP request attributes
func TraceHTTPRequest(method, url, userAgent string, statusCode int) []attribute.KeyValue {
	return []attribute.KeyValue{
		attribute.String("http.method", method),
		attribute.String("http.url", url),
		attribute.String("http.user_agent", userAgent),
		attribute.Int("http.status_code", statusCode),
	}
}

// TraceHTTPServer creates common HTTP server attributes
func TraceHTTPServer(method, route, scheme, host, target string) []attribute.KeyValue {
	return []attribute.KeyValue{
		attribute.String("http.method", method),
		attribute.String("http.route", route),
		attribute.String("http.scheme", scheme),
		attribute.String("http.host", host),
		attribute.String("http.target", target),
	}
}

// TraceHTTPClient creates common HTTP client attributes
func TraceHTTPClient(method, url string, statusCode int) []attribute.KeyValue {
	return []attribute.KeyValue{
		attribute.String("http.method", method),
		attribute.String("http.url", url),
		attribute.Int("http.status_code", statusCode),
	}
}

// TraceDBOperation creates common database operation attributes
func TraceDBOperation(operation, table, database string) []attribute.KeyValue {
	return []attribute.KeyValue{
		attribute.String("db.operation", operation),
		attribute.String("db.sql.table", table),
		attribute.String("db.name", database),
		attribute.String("db.system", "sql"),
	}
}

// TraceDBQuery creates attributes for a database query
func TraceDBQuery(statement, database string) []attribute.KeyValue {
	return []attribute.KeyValue{
		attribute.String("db.statement", statement),
		attribute.String("db.name", database),
		attribute.String("db.system", "sql"),
	}
}

// TraceServiceCall creates common service call attributes
func TraceServiceCall(service, method, endpoint string) []attribute.KeyValue {
	return []attribute.KeyValue{
		attribute.String("service.name", service),
		attribute.String("service.method", method),
		attribute.String("service.endpoint", endpoint),
	}
}

// TraceGRPCCall creates common gRPC call attributes
func TraceGRPCCall(service, method string, statusCode int) []attribute.KeyValue {
	return []attribute.KeyValue{
		attribute.String("rpc.service", service),
		attribute.String("rpc.method", method),
		attribute.String("rpc.system", "grpc"),
		attribute.Int("rpc.grpc.status_code", statusCode),
	}
}

// TraceMessageQueue creates attributes for message queue operations
func TraceMessageQueue(operation, queueName, destination string) []attribute.KeyValue {
	return []attribute.KeyValue{
		attribute.String("messaging.operation", operation),
		attribute.String("messaging.destination", destination),
		attribute.String("messaging.destination_kind", "queue"),
		attribute.String("messaging.system", queueName),
	}
}

// TracePubSubMessage creates attributes for pub/sub operations
func TracePubSubMessage(operation, topic, subscription string) []attribute.KeyValue {
	attrs := []attribute.KeyValue{
		attribute.String("messaging.operation", operation),
		attribute.String("messaging.system", "pubsub"),
	}
	if topic != "" {
		attrs = append(attrs, attribute.String("messaging.destination", topic))
		attrs = append(attrs, attribute.String("messaging.destination_kind", "topic"))
	}
	if subscription != "" {
		attrs = append(attrs, attribute.String("messaging.source", subscription))
	}
	return attrs
}

// TraceGCSOperation creates attributes for Google Cloud Storage operations
func TraceGCSOperation(operation, bucket, object string) []attribute.KeyValue {
	return []attribute.KeyValue{
		attribute.String("gcs.operation", operation),
		attribute.String("gcs.bucket", bucket),
		attribute.String("gcs.object", object),
	}
}

// WithTimeout wraps a function with a timeout and tracing
func WithTimeout(ctx context.Context, timeout time.Duration, spanName string, fn func(context.Context) error) error {
	ctx, span := StartSpanFromContext(ctx, spanName)
	defer span.End()

	// Create timeout context
	timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Add timeout attribute
	span.SetAttributes(attribute.String("timeout", timeout.String()))

	// Execute function
	done := make(chan error, 1)
	go func() {
		done <- fn(timeoutCtx)
	}()

	select {
	case err := <-done:
		if err != nil {
			RecordError(span, err, "operation failed")
		}
		return err
	case <-timeoutCtx.Done():
		err := timeoutCtx.Err()
		RecordError(span, err, "operation timed out")
		return err
	}
}

// TraceFunction is a decorator that adds tracing to any function
func TraceFunction(ctx context.Context, spanName string, fn func(context.Context) error) error {
	ctx, span := StartSpanFromContext(ctx, spanName)
	defer span.End()

	start := time.Now()
	err := fn(ctx)
	duration := time.Since(start)

	// Add duration attribute
	span.SetAttributes(attribute.Int64("duration_ms", duration.Milliseconds()))

	if err != nil {
		RecordError(span, err, "function execution failed")
	}

	return err
}

// TraceFunctionWithResult is a decorator that adds tracing to functions with results
func TraceFunctionWithResult[T any](ctx context.Context, spanName string, fn func(context.Context) (T, error)) (T, error) {
	ctx, span := StartSpanFromContext(ctx, spanName)
	defer span.End()

	start := time.Now()
	result, err := fn(ctx)
	duration := time.Since(start)

	// Add duration attribute
	span.SetAttributes(attribute.Int64("duration_ms", duration.Milliseconds()))

	if err != nil {
		RecordError(span, err, "function execution failed")
	}

	return result, err
}

// GetTraceID extracts the trace ID from context
func GetTraceID(ctx context.Context) string {
	span := trace.SpanFromContext(ctx)
	if span != nil {
		spanContext := span.SpanContext()
		if spanContext.IsValid() {
			return spanContext.TraceID().String()
		}
	}
	return ""
}

// GetSpanID extracts the span ID from context
func GetSpanID(ctx context.Context) string {
	span := trace.SpanFromContext(ctx)
	if span != nil {
		spanContext := span.SpanContext()
		if spanContext.IsValid() {
			return spanContext.SpanID().String()
		}
	}
	return ""
}

// GetTraceContext extracts both trace ID and span ID from context
func GetTraceContext(ctx context.Context) (traceID, spanID string, sampled bool) {
	span := trace.SpanFromContext(ctx)
	if span != nil {
		spanContext := span.SpanContext()
		if spanContext.IsValid() {
			return spanContext.TraceID().String(),
				spanContext.SpanID().String(),
				spanContext.IsSampled()
		}
	}
	return "", "", false
}

// IsTraceEnabled checks if tracing is enabled for the current context
func IsTraceEnabled(ctx context.Context) bool {
	span := trace.SpanFromContext(ctx)
	if span != nil {
		return span.SpanContext().IsValid()
	}
	return false
}

// IsSampled checks if the current trace is sampled
func IsSampled(ctx context.Context) bool {
	span := trace.SpanFromContext(ctx)
	if span != nil {
		return span.SpanContext().IsSampled()
	}
	return false
}

// GetSpanFromContext safely retrieves a span from context
func GetSpanFromContext(ctx context.Context) trace.Span {
	return trace.SpanFromContext(ctx)
}

// EndSpan safely ends a span
func EndSpan(span trace.Span) {
	if span != nil {
		span.End()
	}
}

// EndSpanWithError ends a span and records an error if present
func EndSpanWithError(span trace.Span, err error, description string) {
	if span != nil {
		if err != nil {
			RecordError(span, err, description)
		}
		span.End()
	}
}

// EndSpanWithStatus ends a span with a specific status
func EndSpanWithStatus(span trace.Span, code codes.Code, description string) {
	if span != nil {
		span.SetStatus(code, description)
		span.End()
	}
}

// MeasureLatency measures the latency of an operation and adds it as a span attribute
func MeasureLatency(span trace.Span, start time.Time) {
	if span != nil {
		duration := time.Since(start)
		span.SetAttributes(
			attribute.Int64("latency_ms", duration.Milliseconds()),
			attribute.Float64("latency_s", duration.Seconds()),
		)
	}
}

// MeasureLatencyContext measures the latency and adds it to the current span from context
func MeasureLatencyContext(ctx context.Context, start time.Time) {
	span := trace.SpanFromContext(ctx)
	MeasureLatency(span, start)
}
