# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

#### Core Features
- Comprehensive middleware client with `NewClient()` for unified initialization
- Automatic configuration validation with `SetDefaults()` and `Validate()` methods
- Support for environment variable configuration (`GOOGLE_CLOUD_PROJECT`, `ENVIRONMENT`)
- Graceful shutdown with proper resource cleanup via `Client.Shutdown()`

#### Logging
- Structured logging using Go's `slog` with GCP Cloud Logging integration
- Multi-handler pattern supporting both console and GCP log outputs
- Automatic trace correlation between logs and traces
- Context-aware logging methods (`InfoContext`, `ErrorContext`, etc.)
- Support for multiple log names via `WithLogName()` for component-specific logging
- Custom log levels: Debug, Info, Warn, Error, Critical
- HTTP request/response logging with automatic metrics
- Thread-safe logger implementation with `sync.RWMutex`
- Pretty-print option for development console logs
- Source location tracking (file, line, function)

#### Distributed Tracing
- OpenTelemetry integration with Google Cloud Trace exporter
- Automatic span creation for HTTP requests
- W3C Trace Context propagation
- Configurable sampling ratio (0.0-1.0)
- Batch processing for efficient trace export (default: 512 spans per batch)
- Span helper functions:
  - `StartServerSpan()` for server-side operations
  - `StartClientSpan()` for external API calls
  - `StartProducerSpan()` for message producers
  - `StartConsumerSpan()` for message consumers
  - `StartInternalSpan()` for internal operations
- Span event recording with `AddSpanEvent()` and `AddSpanEventContext()`
- Error recording with `RecordError()` and `RecordErrorContext()`
- Trace ID and Span ID extraction utilities
- Automatic GCP resource detection (Cloud Run, GCE, GKE)

#### HTTP Middleware
- **CORS Middleware**: Configurable cross-origin resource sharing with sensible defaults
- **Recovery Middleware**: Panic recovery with detailed logging and trace recording
- **Request ID Middleware**: Automatic request ID generation and propagation
- **Timeout Middleware**: Configurable per-request timeouts
- **Logging Middleware**: Automatic HTTP request/response logging
- **OpenTelemetry Middleware**: Automatic span creation and context propagation

#### Middleware Chains
- `HTTPHandler()` - Quick start with automatic middleware stack
- `HTTPHandlerWithTimeout()` - Handler with timeout support
- `StandardChain()` - Pre-configured standard middleware stack
- `APIChain()` - Optimized middleware stack for REST APIs
- `NewChain()` - Fully customizable middleware chain builder
- Support for custom middleware composition

#### GCP Integration
- Native GCP Cloud Logging format with `clog/gcp`
- Automatic GCP resource metadata detection
- Cloud Trace export with batch processing
- Support for custom resource attributes
- Automatic severity level mapping
- Log-trace correlation in GCP Cloud Console

#### Development Tools
- Task-based build system using [Taskfile.dev](https://taskfile.dev/)
- Comprehensive task definitions:
  - Build, test, and coverage tasks
  - Linting with golangci-lint
  - Code formatting and vetting
  - Integration test support
  - Example runners
  - Documentation server
  - Git hooks installer
  - Automated tagging
- CI/CD pipeline tasks for Azure DevOps
- Pre-commit git hooks support

#### Examples
- Basic example: Simple HTTP server with standard middleware
- Advanced example: Complex e-commerce order processing service demonstrating:
  - Deep nested telemetry (7+ levels of spans)
  - Multi-service orchestration
  - Error handling and rollback patterns
  - External API calls with retries
  - Parallel batch operations
  - Database and cache operations
  - Different span types and patterns
- Comprehensive examples README with:
  - Usage instructions
  - Sample requests and responses
  - Trace visualization guide
  - GCP Cloud Console navigation tips

#### Documentation
- Comprehensive README with quick start guide
- CLAUDE.md with architectural decisions and design patterns
- Detailed configuration documentation
- Best practices guide
- Performance considerations and optimization tips
- Security guidelines
- Troubleshooting section
- Examples with inline documentation

### Changed
- Migrated from Make to Task for build automation
- Enhanced error handling to never panic in production code
- Optimized middleware ordering for performance and correctness

### Security
- Implemented proper credential handling with Application Default Credentials (ADC)
- Added guidelines for avoiding sensitive data in logs
- CORS configuration recommendations for production deployments
- Secure default configurations

## [1.0.0] - 02-22-2026

Initial release - placeholder for first tagged version.

[1.0.0]: https://github.com/sirhco/go-gcp-middleware/tags

### Changed

[1.0.1] - 02-22-2026

Updated package name and versioned correctly

### Changed
[1.0.2] - 02-22-2026

Fixed naming of files

### Changed
[1.1.0] - 02-22-2026

Downgrade some packages for compatability

### Changed
[1.2.0] - 02-23-2026

Upgrade packages to see if semconv upgrade will fix compatability
