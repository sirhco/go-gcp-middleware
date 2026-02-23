# Examples

This directory contains examples demonstrating the capabilities of the `go-gcp-middleware` package, with a focus on showing the power of distributed tracing and nested telemetry.

## Examples

### Basic Example (`basic/`)

A simple HTTP server demonstrating basic middleware setup with logging and tracing.

**Features demonstrated:**
- Basic middleware configuration
- Simple HTTP handlers with automatic instrumentation
- Request logging and tracing
- Custom middleware chains
- Graceful shutdown

**Run:**
```bash
cd basic
GOOGLE_CLOUD_PROJECT=your-project-id go run main.go
```

### Advanced Example (`advanced/`)

A realistic e-commerce order processing service that demonstrates the full power of distributed tracing with deeply nested operations.

**Features demonstrated:**

#### 1. **Nested Workflow Tracing** (`POST /api/orders`)

Shows a complex order processing workflow with 7+ levels of nested spans:

```
HTTP Request (parent span)
├── validate-order
│   ├── validate-customer
│   ├── validate-items
│   │   ├── validate-item-0
│   │   ├── validate-item-1
│   │   └── validate-item-N
│   └── validate-payment-method
├── fraud-detection-check
│   ├── check-customer-history
│   │   └── db-query-customer-orders
│   ├── check-transaction-patterns
│   │   └── analyze-transaction-pattern
│   └── external-fraud-api
│       └── http-post-fraud-api
├── inventory-check-availability
│   ├── check-item-0
│   │   └── db-query-stock
│   ├── check-item-1
│   │   └── db-query-stock
│   └── check-item-N
│       └── db-query-stock
├── payment-gateway-charge
│   ├── http-post-payment-gateway
│   │   ├── payment-attempt-1
│   │   ├── payment-attempt-2 (if retry needed)
│   │   └── payment-attempt-3 (if retry needed)
│   └── [refund if inventory fails]
├── inventory-reserve-items
│   └── db-transaction
├── db-create-order
│   ├── db-transaction
│   │   ├── db-insert-order
│   │   ├── db-insert-order-items
│   │   └── db-insert-payment-ref
│   └── ...
└── send-order-confirmation
    ├── send-email
    ├── send-sms
    └── send-push-notification
```

#### 2. **Parallel Operations** (`POST /api/orders/batch`)

Demonstrates batch processing where multiple orders are processed in parallel, each with its own complete nested span tree. This shows how you can track multiple workflows simultaneously.

#### 3. **Multi-Service Orchestration** (`POST /api/orders/fulfill`)

Shows a fulfillment workflow that coordinates multiple services:
- Order retrieval (database)
- Inventory verification (database)
- Shipping label creation (external API with rate calculation)
- Inventory commit (database transaction)
- Order status update (database)
- Shipping notification (email, SMS, push)

#### 4. **Error Handling and Rollback Tracing**

The example includes error scenarios that show:
- Failed payment → automatic refund (traced)
- Inventory reservation failure → cleanup operations (traced)
- External API retries with exponential backoff (traced)
- Compensating transactions (traced)

#### 5. **Different Span Types**

- **Server spans**: HTTP request handlers
- **Internal spans**: Business logic operations
- **Client spans**: External HTTP/API calls
- **Producer spans**: Message queue/notification operations
- **Database spans**: Database queries and transactions

#### 6. **Rich Span Attributes**

Each span includes relevant attributes:
- `customer.id`, `order.id`, `payment.id`
- `http.method`, `http.url`, `http.status_code`
- `db.operation`, `db.table`
- `external.service` (Stripe, SendGrid, Twilio, etc.)
- `fraud.score`, `cache.hit`, `retry.attempt`

**Run:**
```bash
cd advanced
GOOGLE_CLOUD_PROJECT=your-project-id go run main.go
```

## Testing the Examples

### Sample Requests

#### Create an order:
```bash
curl -X POST http://localhost:8080/api/orders \
  -H "Content-Type: application/json" \
  -d '{
    "customer_id": "cust-12345",
    "items": [
      {"sku": "WIDGET-001", "name": "Blue Widget", "quantity": 2, "price": 29.99},
      {"sku": "GADGET-002", "name": "Red Gadget", "quantity": 1, "price": 49.99}
    ],
    "total_amount": 109.97,
    "payment_method": "credit_card"
  }'
```

#### Batch order processing:
```bash
curl -X POST http://localhost:8080/api/orders/batch \
  -H "Content-Type: application/json" \
  -d '{
    "orders": [
      {
        "customer_id": "cust-111",
        "items": [{"sku": "SKU-1", "name": "Item 1", "quantity": 1, "price": 10.00}],
        "total_amount": 10.00,
        "payment_method": "credit_card"
      },
      {
        "customer_id": "cust-222",
        "items": [{"sku": "SKU-2", "name": "Item 2", "quantity": 2, "price": 15.00}],
        "total_amount": 30.00,
        "payment_method": "paypal"
      }
    ]
  }'
```

#### Fulfill an order:
```bash
curl -X POST http://localhost:8080/api/orders/fulfill \
  -H "Content-Type: application/json" \
  -d '{
    "order_id": "ord-12345"
  }'
```

## Viewing Traces in GCP Cloud Console

1. **Go to Cloud Trace**: https://console.cloud.google.com/traces/list

2. **Find your traces**:
   - Filter by service name: `order-processing-service`
   - Look for the operation names like `process-order-request`
   - Click on a trace to see the waterfall view

3. **Explore the nested spans**:
   - See the complete call hierarchy
   - View timing for each operation
   - Identify bottlenecks (which operations take the most time)
   - See which external services were called
   - View all attributes and metadata

4. **Correlate with logs**:
   - Click on any span
   - Click "View Logs" to see correlated log entries
   - All logs within a trace are automatically linked

## What to Look For

### Performance Analysis

The traces will show you:
- **Total request time**: How long the entire operation took
- **Breakdown by operation**: Where time is spent (database, external APIs, business logic)
- **Parallelization opportunities**: Operations that could run in parallel
- **Slowest operations**: Easy to identify bottlenecks

### Error Tracking

When errors occur, traces show:
- **Where the error happened**: Exact span in the call hierarchy
- **Error propagation**: How errors bubbled up
- **Retry attempts**: How many retries were attempted
- **Rollback operations**: What cleanup happened

### External Dependencies

Traces clearly show:
- **Which external services** were called
- **How long** each external call took
- **Retry behavior** for failed external calls
- **Circuit breaker** patterns (if implemented)

### Database Performance

Database operations are traced with:
- **Query timing**: How long each query took
- **Transaction boundaries**: Start and end of transactions
- **Lock contention**: If operations are waiting
- **Slow queries**: Easy to identify

## Best Practices Demonstrated

1. **Context propagation**: Every function receives and passes `context.Context`
2. **Span hierarchy**: Parent-child relationships clearly defined
3. **Meaningful span names**: Operation names are descriptive
4. **Rich attributes**: Each span has relevant metadata
5. **Error recording**: Errors are properly recorded on spans
6. **Cleanup tracing**: Even rollback/cleanup operations are traced
7. **Log-trace correlation**: Logs automatically include trace IDs

## Environment Variables

- `GOOGLE_CLOUD_PROJECT`: Your GCP project ID (required)
- `ENVIRONMENT`: `development` or `production` (affects sampling rate)

## Sampling Rates

The examples use different sampling rates by environment:
- **Development**: 100% (all traces captured)
- **Production**: 10% (cost-effective while maintaining observability)

You can adjust these in the code or via configuration.

## Learn More

- [OpenTelemetry Documentation](https://opentelemetry.io/docs/)
- [GCP Cloud Trace](https://cloud.google.com/trace/docs)
- [GCP Cloud Logging](https://cloud.google.com/logging/docs)
- [go-gcp-middleware README](../README.md)
