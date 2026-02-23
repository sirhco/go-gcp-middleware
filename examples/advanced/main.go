package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"slices"
	"syscall"
	"time"

	middleware "github.com/sirhco/go-gcp-middleware"
	"github.com/sirhco/go-gcp-middleware/logger"
	"github.com/sirhco/go-gcp-middleware/telemetry"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
)

func main() {
	ctx := context.Background()

	projectID := os.Getenv("GOOGLE_CLOUD_PROJECT")
	if projectID == "" {
		log.Fatal("GOOGLE_CLOUD_PROJECT environment variable required")
	}

	// Advanced configuration showing nested telemetry
	config := middleware.Config{
		ServiceName:    "order-processing-service",
		ServiceVersion: "2.0.0",
		Environment:    getEnv("ENVIRONMENT", "production"),
		ProjectID:      projectID,

		EnableConsole: true,
		EnableGCP:     true,
		LogLevel:      logger.LevelDebug,
		PrettyLog:     getEnv("ENVIRONMENT", "production") == "development",

		LogName:       "order-service",
		EnableTracing: true,
		TraceRatio:    getTraceRatio(),

		Attributes: map[string]string{
			"team":       "ecommerce",
			"component":  "order-processor",
			"deployment": "cloudrun",
		},
	}

	client, err := middleware.NewClient(ctx, config)
	if err != nil {
		log.Fatalf("Failed to initialize middleware: %v", err)
	}
	defer gracefulShutdown(client)

	// Initialize application services with nested dependencies
	app := &Application{
		client:           client,
		inventoryService: NewInventoryService(client),
		paymentService:   NewPaymentService(client),
		notificationSvc:  NewNotificationService(client),
		orderRepository:  NewOrderRepository(client),
		fraudDetection:   NewFraudDetectionService(client),
		shippingService:  NewShippingService(client),
	}

	mux := setupRoutes(app)

	server := &http.Server{
		Addr:              ":8080",
		Handler:           mux,
		ReadTimeout:       15 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	// Start server
	go func() {
		client.Logger().InfoContext(ctx, "Order Processing Service starting",
			"addr", server.Addr,
			"environment", config.Environment)

		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	// Wait for shutdown signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit

	client.Logger().InfoContext(ctx, "Shutdown signal received", "signal", sig.String())

	shutdownCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		client.Logger().ErrorContext(ctx, "Server shutdown error", "error", err)
	}

	client.Logger().InfoContext(ctx, "Server stopped gracefully")
}

type Application struct {
	client           *middleware.Client
	inventoryService *InventoryService
	paymentService   *PaymentService
	notificationSvc  *NotificationService
	orderRepository  *OrderRepository
	fraudDetection   *FraudDetectionService
	shippingService  *ShippingService
}

func setupRoutes(app *Application) *http.ServeMux {
	mux := http.NewServeMux()

	// Health check
	mux.Handle("/health", app.client.StandardChain("health").ThenFunc(app.healthHandler))

	// Order processing endpoints - these demonstrate deep nested telemetry
	apiChain := app.client.APIChain("api", 30*time.Second)

	// Single order - shows nested workflow
	mux.Handle("/api/orders", apiChain.ThenFunc(app.createOrderHandler))

	// Batch order processing - shows parallel nested operations
	mux.Handle("/api/orders/batch", apiChain.ThenFunc(app.batchOrderHandler))

	// Order fulfillment - shows multi-service orchestration
	mux.Handle("/api/orders/fulfill", apiChain.ThenFunc(app.fulfillOrderHandler))

	return mux
}

func (app *Application) healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "healthy"})
}

// createOrderHandler demonstrates a complex nested workflow:
// HTTP Request -> Validation -> Fraud Check -> Inventory Check -> Payment -> Order Creation -> Notification
func (app *Application) createOrderHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := app.client.Logger()

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Start main operation span
	ctx, span := app.client.Telemetry().GetTracer("order-service").Start(ctx, "process-order-request")
	defer span.End()

	var req CreateOrderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		telemetry.RecordErrorContext(ctx, err, "Invalid request body")
		log.WarnContext(ctx, "Failed to decode request", "error", err)
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	span.SetAttributes(
		attribute.String("order.customer_id", req.CustomerID),
		attribute.Int("order.item_count", len(req.Items)),
		attribute.Float64("order.total_amount", req.TotalAmount),
	)

	log.InfoContext(ctx, "Processing order request",
		"customer_id", req.CustomerID,
		"items", len(req.Items),
		"total", req.TotalAmount)

	// Step 1: Validate order (child span)
	if err := app.validateOrder(ctx, &req); err != nil {
		telemetry.RecordErrorContext(ctx, err, "Order validation failed")
		log.WarnContext(ctx, "Order validation failed", "error", err)
		http.Error(w, fmt.Sprintf("Validation failed: %v", err), http.StatusBadRequest)
		return
	}

	// Step 2: Fraud detection (child span with external API call)
	fraudScore, err := app.fraudDetection.CheckOrder(ctx, &req)
	if err != nil {
		telemetry.RecordErrorContext(ctx, err, "Fraud check failed")
		log.ErrorContext(ctx, "Fraud detection failed", "error", err)
		http.Error(w, "Service temporarily unavailable", http.StatusServiceUnavailable)
		return
	}

	if fraudScore > 0.8 {
		span.SetStatus(codes.Error, "High fraud risk detected")
		log.WarnContext(ctx, "Order blocked due to fraud risk",
			"fraud_score", fraudScore,
			"customer_id", req.CustomerID)
		http.Error(w, "Order cannot be processed", http.StatusForbidden)
		return
	}

	// Step 3: Check inventory (child span with database calls)
	available, err := app.inventoryService.CheckAvailability(ctx, req.Items)
	if err != nil {
		telemetry.RecordErrorContext(ctx, err, "Inventory check failed")
		log.ErrorContext(ctx, "Inventory check failed", "error", err)
		http.Error(w, "Service temporarily unavailable", http.StatusServiceUnavailable)
		return
	}

	if !available {
		log.WarnContext(ctx, "Insufficient inventory", "items", req.Items)
		http.Error(w, "Some items are out of stock", http.StatusConflict)
		return
	}

	// Step 4: Process payment (child span with external payment gateway)
	paymentID, err := app.paymentService.ProcessPayment(ctx, req.CustomerID, req.TotalAmount, req.PaymentMethod)
	if err != nil {
		telemetry.RecordErrorContext(ctx, err, "Payment processing failed")
		log.ErrorContext(ctx, "Payment failed", "error", err, "customer_id", req.CustomerID)
		http.Error(w, "Payment failed", http.StatusPaymentRequired)
		return
	}

	span.SetAttributes(attribute.String("payment.id", paymentID))

	// Step 5: Reserve inventory (child span with database transaction)
	reservationID, err := app.inventoryService.ReserveItems(ctx, req.Items)
	if err != nil {
		// Payment succeeded but inventory reservation failed - need to refund
		log.ErrorContext(ctx, "Inventory reservation failed, initiating refund",
			"error", err,
			"payment_id", paymentID)

		// Nested refund operation
		if refundErr := app.paymentService.RefundPayment(ctx, paymentID); refundErr != nil {
			log.ErrorContext(ctx, "CRITICAL: Refund failed after inventory failure",
				"payment_id", paymentID,
				"refund_error", refundErr)
		}

		http.Error(w, "Order processing failed", http.StatusInternalServerError)
		return
	}

	// Step 6: Create order record (child span with database)
	order, err := app.orderRepository.CreateOrder(ctx, &req, paymentID, reservationID)
	if err != nil {
		telemetry.RecordErrorContext(ctx, err, "Order creation failed")
		log.ErrorContext(ctx, "Failed to create order", "error", err)

		// Cleanup: release inventory and refund payment
		app.cleanupFailedOrder(ctx, paymentID, reservationID)

		http.Error(w, "Order processing failed", http.StatusInternalServerError)
		return
	}

	span.SetAttributes(attribute.String("order.id", order.ID))

	// Step 7: Send confirmation notifications (child span with parallel operations)
	if err := app.notificationSvc.SendOrderConfirmation(ctx, order); err != nil {
		// Non-critical - log but don't fail the order
		log.WarnContext(ctx, "Failed to send notification", "error", err, "order_id", order.ID)
	}

	log.InfoContext(ctx, "Order created successfully",
		"order_id", order.ID,
		"customer_id", req.CustomerID,
		"payment_id", paymentID,
		"total", req.TotalAmount)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(order)
}

// batchOrderHandler demonstrates parallel nested operations
func (app *Application) batchOrderHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := app.client.Logger()

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ctx, span := app.client.Telemetry().GetTracer("order-service").Start(ctx, "process-batch-orders")
	defer span.End()

	var batchReq BatchOrderRequest
	if err := json.NewDecoder(r.Body).Decode(&batchReq); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	span.SetAttributes(attribute.Int("batch.size", len(batchReq.Orders)))
	log.InfoContext(ctx, "Processing batch orders", "count", len(batchReq.Orders))

	// Process orders in parallel (each with its own nested span tree)
	results := app.processBatchOrders(ctx, batchReq.Orders)

	successCount := 0
	for _, result := range results {
		if result.Success {
			successCount++
		}
	}

	span.SetAttributes(
		attribute.Int("batch.success_count", successCount),
		attribute.Int("batch.failure_count", len(results)-successCount),
	)

	log.InfoContext(ctx, "Batch processing complete",
		"total", len(results),
		"success", successCount,
		"failed", len(results)-successCount)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"total":   len(results),
		"success": successCount,
		"failed":  len(results) - successCount,
		"results": results,
	})
}

// fulfillOrderHandler demonstrates multi-service orchestration
func (app *Application) fulfillOrderHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := app.client.Logger()

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ctx, span := app.client.Telemetry().GetTracer("order-service").Start(ctx, "fulfill-order")
	defer span.End()

	var req FulfillOrderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	span.SetAttributes(attribute.String("order.id", req.OrderID))
	log.InfoContext(ctx, "Starting order fulfillment", "order_id", req.OrderID)

	// Step 1: Get order details (database query)
	order, err := app.orderRepository.GetOrder(ctx, req.OrderID)
	if err != nil {
		log.ErrorContext(ctx, "Failed to fetch order", "error", err)
		http.Error(w, "Order not found", http.StatusNotFound)
		return
	}

	// Step 2: Verify inventory is still reserved
	if err := app.inventoryService.VerifyReservation(ctx, order.ReservationID); err != nil {
		log.ErrorContext(ctx, "Inventory reservation invalid", "error", err)
		http.Error(w, "Cannot fulfill order", http.StatusConflict)
		return
	}

	// Step 3: Calculate shipping (external shipping API)
	shippingLabel, err := app.shippingService.CreateShippingLabel(ctx, order)
	if err != nil {
		log.ErrorContext(ctx, "Failed to create shipping label", "error", err)
		http.Error(w, "Shipping service unavailable", http.StatusServiceUnavailable)
		return
	}

	// Step 4: Update inventory (commit the reservation)
	if err := app.inventoryService.CommitReservation(ctx, order.ReservationID); err != nil {
		log.ErrorContext(ctx, "Failed to commit inventory", "error", err)
		http.Error(w, "Fulfillment failed", http.StatusInternalServerError)
		return
	}

	// Step 5: Update order status
	if err := app.orderRepository.UpdateOrderStatus(ctx, req.OrderID, "fulfilled"); err != nil {
		log.ErrorContext(ctx, "Failed to update order status", "error", err)
		// Continue - order is actually fulfilled
	}

	// Step 6: Send shipping notification
	if err := app.notificationSvc.SendShippingNotification(ctx, order, shippingLabel); err != nil {
		log.WarnContext(ctx, "Failed to send shipping notification", "error", err)
	}

	log.InfoContext(ctx, "Order fulfilled successfully",
		"order_id", req.OrderID,
		"tracking_number", shippingLabel.TrackingNumber)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]any{
		"order_id":        order.ID,
		"status":          "fulfilled",
		"tracking_number": shippingLabel.TrackingNumber,
		"carrier":         shippingLabel.Carrier,
	})
}

// Helper methods showing nested operations

func (app *Application) validateOrder(ctx context.Context, req *CreateOrderRequest) error {
	ctx, span := telemetry.StartSpanFromContext(ctx, "validate-order")
	defer span.End()

	log := app.client.Logger()

	// Nested validation steps
	if err := app.validateCustomer(ctx, req.CustomerID); err != nil {
		return fmt.Errorf("customer validation failed: %w", err)
	}

	if err := app.validateItems(ctx, req.Items); err != nil {
		return fmt.Errorf("item validation failed: %w", err)
	}

	if err := app.validatePaymentMethod(ctx, req.PaymentMethod); err != nil {
		return fmt.Errorf("payment method validation failed: %w", err)
	}

	log.DebugContext(ctx, "Order validation passed", "customer_id", req.CustomerID)
	return nil
}

func (app *Application) validateCustomer(ctx context.Context, customerID string) error {
	ctx, span := telemetry.StartSpanFromContext(ctx, "validate-customer")
	defer span.End()

	span.SetAttributes(attribute.String("customer.id", customerID))

	// Simulate customer lookup
	time.Sleep(20 * time.Millisecond)

	if customerID == "" {
		return fmt.Errorf("customer ID required")
	}

	return nil
}

func (app *Application) validateItems(ctx context.Context, items []OrderItem) error {
	ctx, span := telemetry.StartSpanFromContext(ctx, "validate-items")
	defer span.End()

	span.SetAttributes(attribute.Int("items.count", len(items)))

	if len(items) == 0 {
		return fmt.Errorf("no items in order")
	}

	// Validate each item (nested loop with spans)
	for i, item := range items {
		_, itemSpan := telemetry.StartSpanFromContext(ctx, fmt.Sprintf("validate-item-%d", i))
		itemSpan.SetAttributes(
			attribute.String("item.sku", item.SKU),
			attribute.Int("item.quantity", item.Quantity),
		)

		time.Sleep(5 * time.Millisecond)

		if item.Quantity <= 0 {
			itemSpan.SetStatus(codes.Error, "Invalid quantity")
			itemSpan.End()
			return fmt.Errorf("invalid quantity for item %s", item.SKU)
		}

		itemSpan.End()
	}

	return nil
}

func (app *Application) validatePaymentMethod(ctx context.Context, method string) error {
	ctx, span := telemetry.StartSpanFromContext(ctx, "validate-payment-method")
	defer span.End()

	span.SetAttributes(attribute.String("payment.method", method))

	time.Sleep(10 * time.Millisecond)

	validMethods := []string{"credit_card", "debit_card", "paypal", "apple_pay"}
	if slices.Contains(validMethods, method) {
		return nil
	}

	return fmt.Errorf("unsupported payment method: %s", method)
}

func (app *Application) cleanupFailedOrder(ctx context.Context, paymentID, reservationID string) {
	ctx, span := telemetry.StartSpanFromContext(ctx, "cleanup-failed-order")
	defer span.End()

	log := app.client.Logger()
	log.InfoContext(ctx, "Cleaning up failed order",
		"payment_id", paymentID,
		"reservation_id", reservationID)

	// Release inventory (nested span)
	if err := app.inventoryService.ReleaseReservation(ctx, reservationID); err != nil {
		log.ErrorContext(ctx, "Failed to release inventory", "error", err)
	}

	// Refund payment (nested span)
	if err := app.paymentService.RefundPayment(ctx, paymentID); err != nil {
		log.ErrorContext(ctx, "Failed to refund payment", "error", err)
	}
}

func (app *Application) processBatchOrders(ctx context.Context, orders []CreateOrderRequest) []BatchOrderResult {
	ctx, span := telemetry.StartSpanFromContext(ctx, "process-all-batch-orders")
	defer span.End()

	results := make([]BatchOrderResult, len(orders))

	// Process each order (each creates its own nested span tree)
	for i, order := range orders {
		results[i] = app.processSingleBatchOrder(ctx, &order, i)
	}

	return results
}

func (app *Application) processSingleBatchOrder(ctx context.Context, order *CreateOrderRequest, index int) BatchOrderResult {
	ctx, span := telemetry.StartSpanFromContext(ctx, fmt.Sprintf("process-batch-order-%d", index))
	defer span.End()

	log := app.client.Logger()

	span.SetAttributes(
		attribute.Int("batch.index", index),
		attribute.String("order.customer_id", order.CustomerID),
	)

	// Each order goes through the same nested workflow
	// (simplified version - in reality would call the same methods as createOrderHandler)

	// Validate
	if err := app.validateOrder(ctx, order); err != nil {
		log.WarnContext(ctx, "Batch order validation failed",
			"index", index,
			"error", err)
		return BatchOrderResult{Success: false, Error: err.Error()}
	}

	// Check inventory
	available, err := app.inventoryService.CheckAvailability(ctx, order.Items)
	if err != nil || !available {
		return BatchOrderResult{Success: false, Error: "inventory unavailable"}
	}

	// Simulate success
	orderID := fmt.Sprintf("batch-order-%d-%d", time.Now().Unix(), index)

	log.InfoContext(ctx, "Batch order processed",
		"index", index,
		"order_id", orderID)

	return BatchOrderResult{
		Success: true,
		OrderID: orderID,
	}
}

// Service implementations (each with nested operations)

type InventoryService struct {
	logger *logger.Logger
}

func NewInventoryService(client *middleware.Client) *InventoryService {
	return &InventoryService{logger: client.Logger().WithLogName("inventory-service")}
}

func (s *InventoryService) CheckAvailability(ctx context.Context, items []OrderItem) (bool, error) {
	ctx, span := telemetry.StartSpanFromContext(ctx, "inventory-check-availability")
	defer span.End()

	span.SetAttributes(attribute.Int("items.count", len(items)))
	s.logger.DebugContext(ctx, "Checking inventory availability", "items", len(items))

	// Simulate checking each item (nested spans)
	for i, item := range items {
		ctx, itemSpan := telemetry.StartSpanFromContext(ctx, fmt.Sprintf("check-item-%d", i))
		itemSpan.SetAttributes(
			attribute.String("item.sku", item.SKU),
			attribute.Int("item.quantity", item.Quantity),
		)

		// Simulate database query
		time.Sleep(15 * time.Millisecond)

		// Simulate stock check
		available := s.checkItemStock(ctx, item.SKU, item.Quantity)
		itemSpan.SetAttributes(attribute.Bool("item.available", available))

		if !available {
			itemSpan.SetStatus(codes.Error, "Insufficient stock")
			s.logger.WarnContext(ctx, "Insufficient stock", "sku", item.SKU)
			itemSpan.End()
			return false, nil
		}

		itemSpan.End()
	}

	return true, nil
}

func (s *InventoryService) checkItemStock(ctx context.Context, sku string, quantity int) bool {
	_, span := telemetry.StartSpanFromContext(ctx, "db-query-stock")
	defer span.End()

	span.SetAttributes(
		attribute.String("db.operation", "SELECT"),
		attribute.String("db.table", "inventory"),
	)

	time.Sleep(10 * time.Millisecond)

	// Simulate random availability (90% available)
	return rand.Float32() > 0.1
}

func (s *InventoryService) ReserveItems(ctx context.Context, items []OrderItem) (string, error) {
	ctx, span := telemetry.StartSpanFromContext(ctx, "inventory-reserve-items")
	defer span.End()

	s.logger.InfoContext(ctx, "Reserving inventory items", "count", len(items))

	// Simulate database transaction
	time.Sleep(30 * time.Millisecond)

	reservationID := fmt.Sprintf("res-%d", time.Now().UnixNano())
	span.SetAttributes(attribute.String("reservation.id", reservationID))

	return reservationID, nil
}

func (s *InventoryService) VerifyReservation(ctx context.Context, reservationID string) error {
	ctx, span := telemetry.StartSpanFromContext(ctx, "inventory-verify-reservation")
	defer span.End()

	span.SetAttributes(attribute.String("reservation.id", reservationID))
	time.Sleep(10 * time.Millisecond)

	return nil
}

func (s *InventoryService) CommitReservation(ctx context.Context, reservationID string) error {
	ctx, span := telemetry.StartSpanFromContext(ctx, "inventory-commit-reservation")
	defer span.End()

	span.SetAttributes(attribute.String("reservation.id", reservationID))
	s.logger.InfoContext(ctx, "Committing inventory reservation", "reservation_id", reservationID)

	time.Sleep(25 * time.Millisecond)

	return nil
}

func (s *InventoryService) ReleaseReservation(ctx context.Context, reservationID string) error {
	ctx, span := telemetry.StartSpanFromContext(ctx, "inventory-release-reservation")
	defer span.End()

	span.SetAttributes(attribute.String("reservation.id", reservationID))
	time.Sleep(20 * time.Millisecond)

	return nil
}

type PaymentService struct {
	logger *logger.Logger
}

func NewPaymentService(client *middleware.Client) *PaymentService {
	return &PaymentService{logger: client.Logger().WithLogName("payment-service")}
}

func (s *PaymentService) ProcessPayment(ctx context.Context, customerID string, amount float64, method string) (string, error) {
	ctx, span := telemetry.StartClientSpan(ctx, "payment-gateway-charge")
	defer span.End()

	span.SetAttributes(
		attribute.String("customer.id", customerID),
		attribute.Float64("payment.amount", amount),
		attribute.String("payment.method", method),
		attribute.String("external.service", "stripe-api"),
	)

	s.logger.InfoContext(ctx, "Processing payment",
		"customer_id", customerID,
		"amount", amount,
		"method", method)

	// Simulate external API call with retries
	paymentID, err := s.callPaymentGateway(ctx, customerID, amount, method)
	if err != nil {
		span.SetStatus(codes.Error, "Payment failed")
		telemetry.RecordError(span, err, "Payment gateway error")
		return "", err
	}

	span.SetAttributes(attribute.String("payment.id", paymentID))
	s.logger.InfoContext(ctx, "Payment processed successfully",
		"payment_id", paymentID,
		"amount", amount)

	return paymentID, nil
}

func (s *PaymentService) callPaymentGateway(ctx context.Context, customerID string, amount float64, method string) (string, error) {
	ctx, span := telemetry.StartSpanFromContext(ctx, "http-post-payment-gateway")
	defer span.End()

	span.SetAttributes(
		attribute.String("http.method", "POST"),
		attribute.String("http.url", "https://api.stripe.com/v1/charges"),
	)

	// Simulate API latency with retries
	maxRetries := 3
	for attempt := 1; attempt <= maxRetries; attempt++ {
		_, attemptSpan := telemetry.StartSpanFromContext(ctx, fmt.Sprintf("payment-attempt-%d", attempt))
		attemptSpan.SetAttributes(attribute.Int("retry.attempt", attempt))

		time.Sleep(time.Duration(50+rand.Intn(100)) * time.Millisecond)

		// Simulate success rate (95%)
		if rand.Float32() > 0.05 {
			paymentID := fmt.Sprintf("pay_%d", time.Now().UnixNano())
			attemptSpan.SetAttributes(attribute.String("payment.id", paymentID))
			attemptSpan.End()
			return paymentID, nil
		}

		attemptSpan.SetStatus(codes.Error, "Gateway timeout")
		attemptSpan.End()

		if attempt == maxRetries {
			return "", fmt.Errorf("payment gateway timeout after %d attempts", maxRetries)
		}
	}

	return "", fmt.Errorf("payment failed")
}

func (s *PaymentService) RefundPayment(ctx context.Context, paymentID string) error {
	ctx, span := telemetry.StartClientSpan(ctx, "payment-gateway-refund")
	defer span.End()

	span.SetAttributes(
		attribute.String("payment.id", paymentID),
		attribute.String("external.service", "stripe-api"),
	)

	s.logger.InfoContext(ctx, "Refunding payment", "payment_id", paymentID)

	time.Sleep(60 * time.Millisecond)

	return nil
}

type FraudDetectionService struct {
	logger *logger.Logger
}

func NewFraudDetectionService(client *middleware.Client) *FraudDetectionService {
	return &FraudDetectionService{logger: client.Logger().WithLogName("fraud-detection")}
}

func (s *FraudDetectionService) CheckOrder(ctx context.Context, order *CreateOrderRequest) (float64, error) {
	ctx, span := telemetry.StartSpanFromContext(ctx, "fraud-detection-check")
	defer span.End()

	span.SetAttributes(
		attribute.String("customer.id", order.CustomerID),
		attribute.Float64("order.amount", order.TotalAmount),
	)

	s.logger.DebugContext(ctx, "Running fraud detection",
		"customer_id", order.CustomerID,
		"amount", order.TotalAmount)

	// Step 1: Check customer history
	ctx, historySpan := telemetry.StartSpanFromContext(ctx, "check-customer-history")
	historyScore := s.checkCustomerHistory(ctx, order.CustomerID)
	historySpan.SetAttributes(attribute.Float64("fraud.history_score", historyScore))
	historySpan.End()

	// Step 2: Check transaction patterns
	ctx, patternSpan := telemetry.StartSpanFromContext(ctx, "check-transaction-patterns")
	patternScore := s.checkTransactionPatterns(ctx, order)
	patternSpan.SetAttributes(attribute.Float64("fraud.pattern_score", patternScore))
	patternSpan.End()

	// Step 3: Call external fraud detection API
	ctx, externalSpan := telemetry.StartClientSpan(ctx, "external-fraud-api")
	externalSpan.SetAttributes(attribute.String("external.service", "fraud-detection-api"))
	externalScore := s.callFraudAPI(ctx, order)
	externalSpan.SetAttributes(attribute.Float64("fraud.external_score", externalScore))
	externalSpan.End()

	// Combine scores
	finalScore := (historyScore + patternScore + externalScore) / 3.0
	span.SetAttributes(attribute.Float64("fraud.final_score", finalScore))

	s.logger.InfoContext(ctx, "Fraud check complete",
		"customer_id", order.CustomerID,
		"fraud_score", finalScore)

	return finalScore, nil
}

func (s *FraudDetectionService) checkCustomerHistory(ctx context.Context, customerID string) float64 {
	ctx, span := telemetry.StartSpanFromContext(ctx, "db-query-customer-orders")
	defer span.End()

	time.Sleep(20 * time.Millisecond)
	return rand.Float64() * 0.3 // Low risk from history
}

func (s *FraudDetectionService) checkTransactionPatterns(ctx context.Context, order *CreateOrderRequest) float64 {
	ctx, span := telemetry.StartSpanFromContext(ctx, "analyze-transaction-pattern")
	defer span.End()

	time.Sleep(15 * time.Millisecond)
	return rand.Float64() * 0.2 // Low risk from patterns
}

func (s *FraudDetectionService) callFraudAPI(ctx context.Context, order *CreateOrderRequest) float64 {
	ctx, span := telemetry.StartSpanFromContext(ctx, "http-post-fraud-api")
	defer span.End()

	span.SetAttributes(
		attribute.String("http.method", "POST"),
		attribute.String("http.url", "https://api.frauddetection.com/v1/check"),
	)

	time.Sleep(80 * time.Millisecond)
	return rand.Float64() * 0.3
}

type NotificationService struct {
	logger *logger.Logger
}

func NewNotificationService(client *middleware.Client) *NotificationService {
	return &NotificationService{logger: client.Logger().WithLogName("notification-service")}
}

func (s *NotificationService) SendOrderConfirmation(ctx context.Context, order *Order) error {
	ctx, span := telemetry.StartProducerSpan(ctx, "send-order-confirmation")
	defer span.End()

	span.SetAttributes(
		attribute.String("order.id", order.ID),
		attribute.String("customer.id", order.CustomerID),
	)

	s.logger.InfoContext(ctx, "Sending order confirmation",
		"order_id", order.ID,
		"customer_id", order.CustomerID)

	// Send email (nested span)
	if err := s.sendEmail(ctx, order.CustomerID, "order_confirmation", order); err != nil {
		s.logger.WarnContext(ctx, "Failed to send email", "error", err)
	}

	// Send SMS (nested span)
	if err := s.sendSMS(ctx, order.CustomerID, fmt.Sprintf("Order %s confirmed", order.ID)); err != nil {
		s.logger.WarnContext(ctx, "Failed to send SMS", "error", err)
	}

	// Send push notification (nested span)
	if err := s.sendPushNotification(ctx, order.CustomerID, "Order Confirmed", order); err != nil {
		s.logger.WarnContext(ctx, "Failed to send push notification", "error", err)
	}

	return nil
}

func (s *NotificationService) SendShippingNotification(ctx context.Context, order *Order, label *ShippingLabel) error {
	ctx, span := telemetry.StartProducerSpan(ctx, "send-shipping-notification")
	defer span.End()

	span.SetAttributes(
		attribute.String("order.id", order.ID),
		attribute.String("tracking.number", label.TrackingNumber),
	)

	s.logger.InfoContext(ctx, "Sending shipping notification",
		"order_id", order.ID,
		"tracking", label.TrackingNumber)

	data := map[string]string{
		"order_id":        order.ID,
		"tracking_number": label.TrackingNumber,
		"carrier":         label.Carrier,
	}

	s.sendEmail(ctx, order.CustomerID, "order_shipped", data)
	s.sendSMS(ctx, order.CustomerID, fmt.Sprintf("Order shipped! Track: %s", label.TrackingNumber))

	return nil
}

func (s *NotificationService) sendEmail(ctx context.Context, customerID, template string, data any) error {
	ctx, span := telemetry.StartClientSpan(ctx, "send-email")
	defer span.End()

	span.SetAttributes(
		attribute.String("email.template", template),
		attribute.String("customer.id", customerID),
		attribute.String("external.service", "sendgrid"),
	)

	time.Sleep(40 * time.Millisecond)
	return nil
}

func (s *NotificationService) sendSMS(ctx context.Context, customerID, message string) error {
	ctx, span := telemetry.StartClientSpan(ctx, "send-sms")
	defer span.End()

	span.SetAttributes(
		attribute.String("sms.message_length", fmt.Sprintf("%d", len(message))),
		attribute.String("customer.id", customerID),
		attribute.String("external.service", "twilio"),
	)

	time.Sleep(30 * time.Millisecond)
	return nil
}

func (s *NotificationService) sendPushNotification(ctx context.Context, customerID, title string, data any) error {
	ctx, span := telemetry.StartClientSpan(ctx, "send-push-notification")
	defer span.End()

	span.SetAttributes(
		attribute.String("push.title", title),
		attribute.String("customer.id", customerID),
		attribute.String("external.service", "firebase-cm"),
	)

	time.Sleep(25 * time.Millisecond)
	return nil
}

type OrderRepository struct {
	logger *logger.Logger
}

func NewOrderRepository(client *middleware.Client) *OrderRepository {
	return &OrderRepository{logger: client.Logger().WithLogName("order-repository")}
}

func (r *OrderRepository) CreateOrder(ctx context.Context, req *CreateOrderRequest, paymentID, reservationID string) (*Order, error) {
	ctx, span := telemetry.StartSpanFromContext(ctx, "db-create-order")
	defer span.End()

	span.SetAttributes(
		attribute.String("db.operation", "INSERT"),
		attribute.String("db.table", "orders"),
	)

	r.logger.InfoContext(ctx, "Creating order record",
		"customer_id", req.CustomerID,
		"payment_id", paymentID)

	// Simulate database transaction with multiple operations
	ctx, txSpan := telemetry.StartSpanFromContext(ctx, "db-transaction")

	// Insert order
	time.Sleep(20 * time.Millisecond)
	orderID := fmt.Sprintf("ord_%d", time.Now().UnixNano())

	// Insert order items
	ctx, itemsSpan := telemetry.StartSpanFromContext(ctx, "db-insert-order-items")
	itemsSpan.SetAttributes(attribute.Int("items.count", len(req.Items)))
	time.Sleep(15 * time.Millisecond)
	itemsSpan.End()

	// Insert payment reference
	ctx, paySpan := telemetry.StartSpanFromContext(ctx, "db-insert-payment-ref")
	time.Sleep(10 * time.Millisecond)
	paySpan.End()

	txSpan.End()

	order := &Order{
		ID:            orderID,
		CustomerID:    req.CustomerID,
		Items:         req.Items,
		TotalAmount:   req.TotalAmount,
		PaymentID:     paymentID,
		ReservationID: reservationID,
		Status:        "pending",
		CreatedAt:     time.Now(),
	}

	return order, nil
}

func (r *OrderRepository) GetOrder(ctx context.Context, orderID string) (*Order, error) {
	ctx, span := telemetry.StartSpanFromContext(ctx, "db-get-order")
	defer span.End()

	span.SetAttributes(
		attribute.String("db.operation", "SELECT"),
		attribute.String("order.id", orderID),
	)

	time.Sleep(25 * time.Millisecond)

	return &Order{
		ID:            orderID,
		CustomerID:    "cust-123",
		ReservationID: "res-456",
		Status:        "pending",
	}, nil
}

func (r *OrderRepository) UpdateOrderStatus(ctx context.Context, orderID, status string) error {
	ctx, span := telemetry.StartSpanFromContext(ctx, "db-update-order-status")
	defer span.End()

	span.SetAttributes(
		attribute.String("db.operation", "UPDATE"),
		attribute.String("order.id", orderID),
		attribute.String("order.status", status),
	)

	r.logger.InfoContext(ctx, "Updating order status",
		"order_id", orderID,
		"status", status)

	time.Sleep(15 * time.Millisecond)

	return nil
}

type ShippingService struct {
	logger *logger.Logger
}

func NewShippingService(client *middleware.Client) *ShippingService {
	return &ShippingService{logger: client.Logger().WithLogName("shipping-service")}
}

func (s *ShippingService) CreateShippingLabel(ctx context.Context, order *Order) (*ShippingLabel, error) {
	ctx, span := telemetry.StartClientSpan(ctx, "shipping-create-label")
	defer span.End()

	span.SetAttributes(
		attribute.String("order.id", order.ID),
		attribute.String("external.service", "shipstation-api"),
	)

	s.logger.InfoContext(ctx, "Creating shipping label", "order_id", order.ID)

	// Step 1: Calculate shipping rates
	ctx, ratesSpan := telemetry.StartSpanFromContext(ctx, "calculate-shipping-rates")
	rates := s.calculateRates(ctx, order)
	ratesSpan.SetAttributes(attribute.Int("rates.count", len(rates)))
	ratesSpan.End()

	// Step 2: Select best rate
	selectedRate := rates[0]

	// Step 3: Create label via API
	ctx, apiSpan := telemetry.StartSpanFromContext(ctx, "http-post-create-label")
	apiSpan.SetAttributes(
		attribute.String("http.method", "POST"),
		attribute.String("http.url", "https://api.shipstation.com/v1/labels"),
	)
	time.Sleep(100 * time.Millisecond)
	apiSpan.End()

	label := &ShippingLabel{
		TrackingNumber: fmt.Sprintf("TRACK%d", time.Now().UnixNano()),
		Carrier:        selectedRate.Carrier,
		Service:        selectedRate.Service,
		Cost:           selectedRate.Cost,
	}

	span.SetAttributes(attribute.String("tracking.number", label.TrackingNumber))

	return label, nil
}

func (s *ShippingService) calculateRates(ctx context.Context, order *Order) []ShippingRate {
	_, span := telemetry.StartSpanFromContext(ctx, "http-get-shipping-rates")
	defer span.End()

	time.Sleep(50 * time.Millisecond)

	return []ShippingRate{
		{Carrier: "USPS", Service: "Priority", Cost: 8.99},
		{Carrier: "FedEx", Service: "Ground", Cost: 12.50},
	}
}

// Models

type CreateOrderRequest struct {
	CustomerID    string      `json:"customer_id"`
	Items         []OrderItem `json:"items"`
	TotalAmount   float64     `json:"total_amount"`
	PaymentMethod string      `json:"payment_method"`
}

type OrderItem struct {
	SKU      string  `json:"sku"`
	Name     string  `json:"name"`
	Quantity int     `json:"quantity"`
	Price    float64 `json:"price"`
}

type Order struct {
	ID            string      `json:"id"`
	CustomerID    string      `json:"customer_id"`
	Items         []OrderItem `json:"items"`
	TotalAmount   float64     `json:"total_amount"`
	PaymentID     string      `json:"payment_id"`
	ReservationID string      `json:"reservation_id"`
	Status        string      `json:"status"`
	CreatedAt     time.Time   `json:"created_at"`
}

type BatchOrderRequest struct {
	Orders []CreateOrderRequest `json:"orders"`
}

type BatchOrderResult struct {
	Success bool   `json:"success"`
	OrderID string `json:"order_id,omitempty"`
	Error   string `json:"error,omitempty"`
}

type FulfillOrderRequest struct {
	OrderID string `json:"order_id"`
}

type ShippingLabel struct {
	TrackingNumber string  `json:"tracking_number"`
	Carrier        string  `json:"carrier"`
	Service        string  `json:"service"`
	Cost           float64 `json:"cost"`
}

type ShippingRate struct {
	Carrier string
	Service string
	Cost    float64
}

// Helper functions

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getTraceRatio() float64 {
	env := getEnv("ENVIRONMENT", "production")
	if env == "development" {
		return 1.0 // 100% sampling in dev
	}
	return 0.1 // 10% sampling in production
}

func gracefulShutdown(client *middleware.Client) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := client.Shutdown(ctx); err != nil {
		log.Printf("Shutdown error: %v", err)
	}
}
