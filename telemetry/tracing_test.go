package telemetry

import (
	"context"
	"errors"
	"testing"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

func TestStartSpan(t *testing.T) {
	ctx := context.Background()

	ctx, span := StartSpan(ctx, "test-tracer", "test-span")
	if span == nil {
		t.Fatal("Expected non-nil span")
	}
	defer span.End()

	// Note: Span context may not be valid when tracing is not enabled
	// Just verify we got a span object
}

func TestStartSpanWithOptions(t *testing.T) {
	ctx := context.Background()

	opts := SpanOptions{
		Kind: trace.SpanKindClient,
		Attributes: []attribute.KeyValue{
			attribute.String("key", "value"),
		},
	}

	ctx, span := StartSpan(ctx, "test-tracer", "test-span", opts)
	if span == nil {
		t.Fatal("Expected non-nil span")
	}
	defer span.End()
}

func TestStartSpanFromContext(t *testing.T) {
	ctx := context.Background()

	// Initialize global provider
	config := Config{
		ServiceName:   "test",
		ProjectID:     "test-project",
		EnableTracing: false,
	}
	InitGlobal(ctx, config)
	defer ShutdownGlobal(ctx)

	ctx, span := StartSpanFromContext(ctx, "test-span")
	if span == nil {
		t.Fatal("Expected non-nil span")
	}
	defer span.End()
}

func TestStartServerSpan(t *testing.T) {
	ctx := context.Background()

	ctx, span := StartServerSpan(ctx, "server-span")
	if span == nil {
		t.Fatal("Expected non-nil span")
	}
	defer span.End()
}

func TestStartClientSpan(t *testing.T) {
	ctx := context.Background()

	ctx, span := StartClientSpan(ctx, "client-span")
	if span == nil {
		t.Fatal("Expected non-nil span")
	}
	defer span.End()
}

func TestStartProducerSpan(t *testing.T) {
	ctx := context.Background()

	ctx, span := StartProducerSpan(ctx, "producer-span")
	if span == nil {
		t.Fatal("Expected non-nil span")
	}
	defer span.End()
}

func TestStartConsumerSpan(t *testing.T) {
	ctx := context.Background()

	ctx, span := StartConsumerSpan(ctx, "consumer-span")
	if span == nil {
		t.Fatal("Expected non-nil span")
	}
	defer span.End()
}

func TestRecordError(t *testing.T) {
	ctx := context.Background()
	ctx, span := StartSpanFromContext(ctx, "test-span")
	defer span.End()

	err := errors.New("test error")
	RecordError(span, err, "operation failed")

	// Verify span status is set to error
	// Note: We can't directly verify this without more complex mocking
}

func TestRecordErrorContext(t *testing.T) {
	ctx := context.Background()
	ctx, span := StartSpanFromContext(ctx, "test-span")
	defer span.End()

	err := errors.New("test error")
	RecordErrorContext(ctx, err, "operation failed")
}

func TestAddSpanAttributes(t *testing.T) {
	ctx := context.Background()
	ctx, span := StartSpanFromContext(ctx, "test-span")
	defer span.End()

	AddSpanAttributes(span,
		attribute.String("key", "value"),
		attribute.Int("number", 42),
	)
}

func TestAddSpanAttributesContext(t *testing.T) {
	ctx := context.Background()
	ctx, span := StartSpanFromContext(ctx, "test-span")
	defer span.End()

	AddSpanAttributesContext(ctx,
		attribute.String("key", "value"),
	)
}

func TestAddSpanEvent(t *testing.T) {
	ctx := context.Background()
	ctx, span := StartSpanFromContext(ctx, "test-span")
	defer span.End()

	AddSpanEvent(span, "test-event",
		attribute.String("event.key", "value"),
	)
}

func TestAddSpanEventContext(t *testing.T) {
	ctx := context.Background()
	ctx, span := StartSpanFromContext(ctx, "test-span")
	defer span.End()

	AddSpanEventContext(ctx, "test-event")
}

func TestSetSpanStatus(t *testing.T) {
	ctx := context.Background()
	ctx, span := StartSpanFromContext(ctx, "test-span")
	defer span.End()

	SetSpanStatus(span, codes.Ok, "success")
	SetSpanStatus(span, codes.Error, "failed")
}

func TestSetSpanStatusContext(t *testing.T) {
	ctx := context.Background()
	ctx, span := StartSpanFromContext(ctx, "test-span")
	defer span.End()

	SetSpanStatusContext(ctx, codes.Ok, "success")
}

func TestAttributeHelpers(t *testing.T) {
	t.Run("WithSpanAttribute", func(t *testing.T) {
		attr := WithSpanAttribute("key", "value")
		if attr.Key != "key" {
			t.Errorf("Expected key 'key', got %s", attr.Key)
		}
	})

	t.Run("WithSpanIntAttribute", func(t *testing.T) {
		attr := WithSpanIntAttribute("count", 42)
		if attr.Key != "count" {
			t.Errorf("Expected key 'count', got %s", attr.Key)
		}
	})

	t.Run("WithSpanInt64Attribute", func(t *testing.T) {
		attr := WithSpanInt64Attribute("count", int64(42))
		if attr.Key != "count" {
			t.Errorf("Expected key 'count', got %s", attr.Key)
		}
	})

	t.Run("WithSpanBoolAttribute", func(t *testing.T) {
		attr := WithSpanBoolAttribute("flag", true)
		if attr.Key != "flag" {
			t.Errorf("Expected key 'flag', got %s", attr.Key)
		}
	})

	t.Run("WithSpanFloat64Attribute", func(t *testing.T) {
		attr := WithSpanFloat64Attribute("price", 19.99)
		if attr.Key != "price" {
			t.Errorf("Expected key 'price', got %s", attr.Key)
		}
	})
}

func TestTraceHTTPRequest(t *testing.T) {
	attrs := TraceHTTPRequest("GET", "http://example.com", "curl/7.0", 200)
	if len(attrs) != 4 {
		t.Errorf("Expected 4 attributes, got %d", len(attrs))
	}
}

func TestTraceHTTPServer(t *testing.T) {
	attrs := TraceHTTPServer("GET", "/api/users", "https", "example.com", "/api/users?page=1")
	if len(attrs) != 5 {
		t.Errorf("Expected 5 attributes, got %d", len(attrs))
	}
}

func TestTraceHTTPClient(t *testing.T) {
	attrs := TraceHTTPClient("POST", "http://api.example.com", 201)
	if len(attrs) != 3 {
		t.Errorf("Expected 3 attributes, got %d", len(attrs))
	}
}

func TestTraceDBOperation(t *testing.T) {
	attrs := TraceDBOperation("SELECT", "users", "mydb")
	if len(attrs) != 4 {
		t.Errorf("Expected 4 attributes, got %d", len(attrs))
	}
}

func TestTraceDBQuery(t *testing.T) {
	attrs := TraceDBQuery("SELECT * FROM users", "mydb")
	if len(attrs) != 3 {
		t.Errorf("Expected 3 attributes, got %d", len(attrs))
	}
}

func TestTraceServiceCall(t *testing.T) {
	attrs := TraceServiceCall("payment-service", "processPayment", "http://payment.local")
	if len(attrs) != 3 {
		t.Errorf("Expected 3 attributes, got %d", len(attrs))
	}
}

func TestTraceGRPCCall(t *testing.T) {
	attrs := TraceGRPCCall("UserService", "GetUser", 0)
	if len(attrs) != 4 {
		t.Errorf("Expected 4 attributes, got %d", len(attrs))
	}
}

func TestTraceMessageQueue(t *testing.T) {
	attrs := TraceMessageQueue("send", "rabbitmq", "user-events")
	if len(attrs) != 4 {
		t.Errorf("Expected 4 attributes, got %d", len(attrs))
	}
}

func TestTracePubSubMessage(t *testing.T) {
	t.Run("with topic", func(t *testing.T) {
		attrs := TracePubSubMessage("publish", "user-events", "")
		if len(attrs) < 2 {
			t.Errorf("Expected at least 2 attributes, got %d", len(attrs))
		}
	})

	t.Run("with subscription", func(t *testing.T) {
		attrs := TracePubSubMessage("receive", "", "user-events-sub")
		if len(attrs) < 2 {
			t.Errorf("Expected at least 2 attributes, got %d", len(attrs))
		}
	})
}

func TestTraceGCSOperation(t *testing.T) {
	attrs := TraceGCSOperation("upload", "my-bucket", "file.txt")
	if len(attrs) != 3 {
		t.Errorf("Expected 3 attributes, got %d", len(attrs))
	}
}

func TestWithTimeout(t *testing.T) {
	ctx := context.Background()

	t.Run("completes before timeout", func(t *testing.T) {
		err := WithTimeout(ctx, 100*time.Millisecond, "test-op", func(ctx context.Context) error {
			time.Sleep(10 * time.Millisecond)
			return nil
		})

		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
	})

	t.Run("times out", func(t *testing.T) {
		err := WithTimeout(ctx, 50*time.Millisecond, "test-op", func(ctx context.Context) error {
			time.Sleep(200 * time.Millisecond)
			return nil
		})

		if err == nil {
			t.Error("Expected timeout error")
		}
	})

	t.Run("function returns error", func(t *testing.T) {
		expectedErr := errors.New("operation failed")
		err := WithTimeout(ctx, 100*time.Millisecond, "test-op", func(ctx context.Context) error {
			return expectedErr
		})

		if err != expectedErr {
			t.Errorf("Expected error %v, got %v", expectedErr, err)
		}
	})
}

func TestTraceFunction(t *testing.T) {
	ctx := context.Background()

	t.Run("successful execution", func(t *testing.T) {
		err := TraceFunction(ctx, "test-func", func(ctx context.Context) error {
			return nil
		})

		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
	})

	t.Run("function returns error", func(t *testing.T) {
		expectedErr := errors.New("test error")
		err := TraceFunction(ctx, "test-func", func(ctx context.Context) error {
			return expectedErr
		})

		if err != expectedErr {
			t.Errorf("Expected error %v, got %v", expectedErr, err)
		}
	})
}

func TestTraceFunctionWithResult(t *testing.T) {
	ctx := context.Background()

	t.Run("successful execution", func(t *testing.T) {
		result, err := TraceFunctionWithResult(ctx, "test-func", func(ctx context.Context) (string, error) {
			return "success", nil
		})

		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
		if result != "success" {
			t.Errorf("Expected result 'success', got %s", result)
		}
	})

	t.Run("function returns error", func(t *testing.T) {
		expectedErr := errors.New("test error")
		result, err := TraceFunctionWithResult(ctx, "test-func", func(ctx context.Context) (string, error) {
			return "", expectedErr
		})

		if err != expectedErr {
			t.Errorf("Expected error %v, got %v", expectedErr, err)
		}
		if result != "" {
			t.Errorf("Expected empty result, got %s", result)
		}
	})
}

func TestGetTraceID(t *testing.T) {
	ctx := context.Background()
	ctx, span := StartSpanFromContext(ctx, "test-span")
	defer span.End()

	traceID := GetTraceID(ctx)
	// Note: Trace ID may be empty when tracing is not enabled
	// Just verify function doesn't panic
	_ = traceID
}

func TestGetSpanID(t *testing.T) {
	ctx := context.Background()
	ctx, span := StartSpanFromContext(ctx, "test-span")
	defer span.End()

	spanID := GetSpanID(ctx)
	// Note: Span ID may be empty when tracing is not enabled
	// Just verify function doesn't panic
	_ = spanID
}

func TestGetTraceContext(t *testing.T) {
	ctx := context.Background()
	ctx, span := StartSpanFromContext(ctx, "test-span")
	defer span.End()

	traceID, spanID, sampled := GetTraceContext(ctx)
	// Note: IDs may be empty when tracing is not enabled
	// Just verify function doesn't panic
	_, _, _ = traceID, spanID, sampled
}

func TestIsTraceEnabled(t *testing.T) {
	ctx := context.Background()

	t.Run("without span", func(t *testing.T) {
		if IsTraceEnabled(ctx) {
			t.Error("Expected trace to be disabled without span")
		}
	})

	t.Run("with span", func(t *testing.T) {
		ctx, span := StartSpanFromContext(ctx, "test-span")
		defer span.End()

		// Note: Trace may not be enabled when tracing is disabled in tests
		// Just verify function doesn't panic
		_ = IsTraceEnabled(ctx)
	})
}

func TestIsSampled(t *testing.T) {
	ctx := context.Background()
	ctx, span := StartSpanFromContext(ctx, "test-span")
	defer span.End()

	_ = IsSampled(ctx) // Result depends on sampling configuration
}

func TestGetSpanFromContext(t *testing.T) {
	ctx := context.Background()
	ctx, originalSpan := StartSpanFromContext(ctx, "test-span")
	defer originalSpan.End()

	span := GetSpanFromContext(ctx)
	if span == nil {
		t.Error("Expected non-nil span")
	}
}

func TestEndSpan(t *testing.T) {
	ctx := context.Background()
	_, span := StartSpanFromContext(ctx, "test-span")

	// Should not panic
	EndSpan(span)
}

func TestEndSpanWithError(t *testing.T) {
	ctx := context.Background()
	_, span := StartSpanFromContext(ctx, "test-span")

	err := errors.New("test error")
	EndSpanWithError(span, err, "operation failed")
}

func TestEndSpanWithStatus(t *testing.T) {
	ctx := context.Background()
	_, span := StartSpanFromContext(ctx, "test-span")

	EndSpanWithStatus(span, codes.Ok, "success")
}

func TestMeasureLatency(t *testing.T) {
	ctx := context.Background()
	_, span := StartSpanFromContext(ctx, "test-span")

	start := time.Now()
	time.Sleep(10 * time.Millisecond)

	MeasureLatency(span, start)
	span.End()
}

func TestMeasureLatencyContext(t *testing.T) {
	ctx := context.Background()
	ctx, span := StartSpanFromContext(ctx, "test-span")
	defer span.End()

	start := time.Now()
	time.Sleep(10 * time.Millisecond)

	MeasureLatencyContext(ctx, start)
}

// Benchmark tests

func BenchmarkStartSpan(b *testing.B) {
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, span := StartSpanFromContext(ctx, "bench-span")
		span.End()
	}
}

func BenchmarkAddSpanAttributes(b *testing.B) {
	ctx := context.Background()
	_, span := StartSpanFromContext(ctx, "bench-span")
	defer span.End()

	attrs := []attribute.KeyValue{
		attribute.String("key1", "value1"),
		attribute.Int("key2", 42),
		attribute.Bool("key3", true),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		AddSpanAttributes(span, attrs...)
	}
}

func BenchmarkRecordError(b *testing.B) {
	ctx := context.Background()
	_, span := StartSpanFromContext(ctx, "bench-span")
	defer span.End()

	err := errors.New("benchmark error")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		RecordError(span, err, "operation failed")
	}
}
