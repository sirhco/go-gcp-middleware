# Contributing to go-gcp-middleware

Thank you for your interest in contributing! This document provides guidelines for contributing to the project.

## Development Setup

1. **Clone the repository**
   ```bash
   git clone https://github.com/sirch/go-gcp-middleware
   cd go-gcp-middleware
   ```

2. **Install dependencies**
   ```bash
   go mod download
   ```

3. **Set up GCP credentials**
   ```bash
   export GOOGLE_CLOUD_PROJECT=your-project-id
   # Or authenticate with gcloud
   gcloud auth application-default login
   ```

## Code Guidelines

### Go Standards

- Follow the [Effective Go](https://golang.org/doc/effective_go) guidelines
- Use `gofmt` to format code
- Run `go vet` to catch common issues
- Use `golangci-lint` for comprehensive linting

### Package Structure

```
go-gcp-middleware/
├── client.go          # Main client and configuration
├── middleware.go      # HTTP middleware implementations
├── logger/           # Logging package
├── telemetry/        # OpenTelemetry and tracing
├── helpers/          # Helper utilities
└── examples/         # Usage examples
```

### Naming Conventions

- **Packages**: Short, lowercase, single-word names
- **Exported functions**: PascalCase (e.g., `NewClient`, `HTTPHandler`)
- **Private functions**: camelCase (e.g., `setupTracing`, `validateConfig`)
- **Constants**: PascalCase or ALL_CAPS for package-level constants

### Error Handling

- Always return errors, don't panic (except in init or fatal situations)
- Wrap errors with context: `fmt.Errorf("operation failed: %w", err)`
- Log errors with structured fields
- Use appropriate log levels (Debug, Info, Warn, Error, Critical)

### Testing

- Write tests for all new functionality
- Aim for >80% code coverage
- Use table-driven tests where appropriate
- Mock external dependencies (GCP services, databases)

Example test:
```go
func TestNewClient(t *testing.T) {
    tests := []struct {
        name    string
        config  Config
        wantErr bool
    }{
        {
            name: "valid config",
            config: Config{
                ServiceName: "test",
                ProjectID:   "test-project",
            },
            wantErr: false,
        },
        {
            name: "missing project id",
            config: Config{
                ServiceName: "test",
            },
            wantErr: true,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            _, err := NewClient(context.Background(), tt.config)
            if (err != nil) != tt.wantErr {
                t.Errorf("NewClient() error = %v, wantErr %v", err, tt.wantErr)
            }
        })
    }
}
```

### Documentation

- Document all exported types, functions, and constants
- Use godoc format for documentation
- Include usage examples in documentation
- Update README.md for new features

### Commit Messages

Follow the [Conventional Commits](https://www.conventionalcommits.org/) specification:

- `feat:` New features
- `fix:` Bug fixes
- `docs:` Documentation changes
- `refactor:` Code refactoring
- `test:` Test additions or changes
- `chore:` Maintenance tasks

Examples:
```
feat: add support for custom CORS configuration
fix: correct trace ID propagation in nested spans
docs: update README with new configuration options
```

## Pull Request Process

1. **Create a feature branch**
   ```bash
   git checkout -b feature/your-feature-name
   ```

2. **Make your changes**
   - Write code following the guidelines above
   - Add or update tests
   - Update documentation

3. **Test your changes**
   ```bash
   go test ./...
   go test -race ./...
   go test -cover ./...
   ```

4. **Lint your code**
   ```bash
   gofmt -w .
   go vet ./...
   golangci-lint run
   ```

5. **Commit and push**
   ```bash
   git add .
   git commit -m "feat: add new feature"
   git push origin feature/your-feature-name
   ```

6. **Create a Pull Request**
   - Provide a clear description of the changes
   - Reference any related issues
   - Ensure all CI checks pass

## Adding New Features

When adding new features:

1. **Discuss first** - For major features, open an issue first to discuss
2. **Keep it focused** - One feature per PR
3. **Maintain backward compatibility** - Don't break existing APIs
4. **Add tests** - New features must have tests
5. **Update examples** - Add usage examples if applicable
6. **Document** - Update README and godoc comments

## Testing

### Running Tests

```bash
# Run all tests
go test ./...

# Run with coverage
go test -cover ./...

# Run with race detection
go test -race ./...

# Run specific package
go test ./logger

# Run with verbose output
go test -v ./...
```

### Integration Tests

Integration tests require GCP credentials:

```bash
export GOOGLE_CLOUD_PROJECT=your-test-project
go test -tags=integration ./...
```

## Code Review

All contributions require code review. Reviewers will check for:

- Code quality and adherence to guidelines
- Test coverage
- Documentation completeness
- Performance implications
- Security considerations
- Backward compatibility
