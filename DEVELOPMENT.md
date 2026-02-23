# Development Guide

This guide covers development workflows, tools, and best practices for contributing to go-gcp-middleware.

## Prerequisites

- **Go**: [Install Go](https://go.dev/doc/install)
- **Task**: [Install Task](https://taskfile.dev/installation/) (replaces Make)
- **GCP Project**: For integration tests (optional)

## Quick Start

1. **Install Task** (if not already installed):
   ```bash
   # macOS
   brew install go-task/tap/go-task

   # Linux
   sh -c "$(curl --location https://taskfile.dev/install.sh)" -- -d -b /usr/local/bin

   # Windows (Chocolatey)
   choco install go-task

   # Or using Go
   go install github.com/go-task/task/v3/cmd/task@latest
   ```

2. **Install development tools**:
   ```bash
   task install-tools
   ```

3. **Install git hooks** (optional but recommended):
   ```bash
   task hooks
   ```
   This will run `task check` before each commit.

## Available Tasks

View all available tasks:
```bash
task --list
```

### Development Tasks

| Task | Description |
|------|-------------|
| `task deps` | Download and tidy Go dependencies |
| `task build` | Build the package |
| `task fmt` | Format code with `go fmt` |
| `task vet` | Run `go vet` static analysis |
| `task lint` | Run golangci-lint (requires install-tools) |
| `task check` | Run fmt, vet, and test (recommended before commit) |
| `task clean` | Remove build artifacts and test outputs |

### Testing Tasks

| Task | Description |
|------|-------------|
| `task test` | Run unit tests |
| `task test:coverage` | Run tests with coverage report (generates coverage.html) |
| `task test:race` | Run tests with race detector |
| `task test:integration` | Run integration tests (requires GCP credentials) |

### Example Tasks

| Task | Description |
|------|-------------|
| `task examples` | Build example binaries to bin/ |
| `task run:basic` | Build and run basic example |
| `task run:advanced` | Build and run advanced example |

### Documentation Tasks

| Task | Description |
|------|-------------|
| `task doc` | Serve Go documentation at http://localhost:6060 |

### Git Tasks

| Task | Description |
|------|-------------|
| `task hooks` | Install git pre-commit hooks |
| `task tag VERSION=v1.0.0` | Create and push a git tag |

### CI Tasks

| Task | Description |
|------|-------------|
| `task ci:test` | Run tests as in CI (with race detector and coverage) |
| `task ci:lint` | Run linting as in CI (with GitHub Actions format) |

## Development Workflow

### Before You Start

1. Create a new branch:
   ```bash
   git checkout -b feature/my-new-feature
   ```

2. Ensure dependencies are up to date:
   ```bash
   task deps
   ```

### During Development

1. **Make your changes**

2. **Format and check your code**:
   ```bash
   task check
   ```
   This runs:
   - `go fmt` to format code
   - `go vet` to check for issues
   - `go test` to run tests

3. **Run additional checks** (optional but recommended):
   ```bash
   task lint           # Run golangci-lint
   task test:race      # Check for race conditions
   task test:coverage  # Generate coverage report
   ```

### Testing

#### Unit Tests

Run all unit tests:
```bash
task test
```

Run tests with coverage:
```bash
task test:coverage
# Open coverage.html in browser to see coverage report
```

Run tests with race detector:
```bash
task test:race
```

#### Integration Tests

Integration tests require GCP credentials and a GCP project.

1. **Set up GCP credentials**:
   ```bash
   # Using Application Default Credentials
   gcloud auth application-default login

   # Or using a service account key file
   export GOOGLE_APPLICATION_CREDENTIALS=/path/to/keyfile.json
   ```

2. **Set GCP project**:
   ```bash
   export GOOGLE_CLOUD_PROJECT=your-project-id
   ```

3. **Run integration tests**:
   ```bash
   task test:integration
   ```

#### Testing Examples

Test the example applications:

1. **Set environment variables**:
   ```bash
   export GOOGLE_CLOUD_PROJECT=your-project-id
   export GOOGLE_APPLICATION_CREDENTIALS=/path/to/keyfile.json  # Optional
   ```

2. **Run examples**:
   ```bash
   # Basic example
   task run:basic

   # Advanced example
   task run:advanced
   ```

3. **Test the endpoints**:
   ```bash
   # In another terminal
   curl http://localhost:8080/health
   curl http://localhost:8080/api/hello
   ```

4. **View traces in GCP**:
   - Go to [GCP Console > Trace](https://console.cloud.google.com/traces)
   - Select your project
   - View recent traces

### Code Style

- **Follow Go conventions**: Use `go fmt`, `go vet`, and `golangci-lint`
- **Write tests**: All new features should have tests
- **Document exports**: All exported types, functions, and methods should have doc comments
- **Use structured logging**: Use key-value pairs with slog
- **Pass context**: Always pass `context.Context` for cancellation and tracing

### Commit Messages

Follow conventional commit format:

```
type(scope): subject

body (optional)

footer (optional)
```

Types:
- `feat`: New feature
- `fix`: Bug fix
- `docs`: Documentation changes
- `style`: Code style changes (formatting)
- `refactor`: Code refactoring
- `test`: Test changes
- `chore`: Build/tooling changes

Examples:
```
feat(logger): add support for custom log names

fix(middleware): correct CORS header handling

docs(readme): update installation instructions

test(telemetry): add integration tests for trace export
```

### Before Committing

If you installed git hooks:
```bash
task hooks
```

The pre-commit hook will automatically run `task check` before each commit.

To run manually:
```bash
task check
```

### Creating a Pull Request

1. **Ensure all tests pass**:
   ```bash
   task ci:test
   task ci:lint
   ```

2. **Update documentation** if needed:
   - Update README.md for user-facing changes
   - Update CLAUDE.md for architectural decisions
   - Update examples if APIs changed

3. **Push your branch**:
   ```bash
   git push origin feature/my-new-feature
   ```

4. **Create pull request** in Azure DevOps

## Project Structure

```
.
├── Taskfile.yml          # Task runner configuration
├── DEVELOPMENT.md        # This file
├── CLAUDE.md            # Project documentation for Claude Code
├── README.md            # User documentation
├── client.go            # Main client and configuration
├── middleware.go        # HTTP middleware implementations
├── logger/              # Logging package
│   ├── logger.go
│   └── logger_test.go
├── telemetry/           # OpenTelemetry and tracing
│   ├── telemetry.go
│   └── tracing.go
├── helpers/             # Helper utilities
│   └── http.go
└── examples/            # Usage examples
    ├── README.md
    ├── basic/
    └── advanced/
```

## Troubleshooting

### Task not found

If you get "task: command not found":

1. **Verify installation**:
   ```bash
   which task
   ```

2. **Reinstall using Go**:
   ```bash
   go install github.com/go-task/task/v3/cmd/task@latest
   ```

3. **Ensure `$GOPATH/bin` is in your PATH**:
   ```bash
   export PATH=$PATH:$(go env GOPATH)/bin
   ```

### golangci-lint not found

If you get "golangci-lint not found":

```bash
task install-tools
```

Or install manually:
```bash
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
```

### Integration tests fail

Common issues:

1. **Missing GCP credentials**:
   ```bash
   gcloud auth application-default login
   ```

2. **Wrong project**:
   ```bash
   export GOOGLE_CLOUD_PROJECT=your-project-id
   ```

3. **Missing IAM permissions**:
   - Ensure the service account has "Cloud Trace Agent" role
   - Ensure the service account has "Logs Writer" role

### Examples don't show traces

1. **Wait a few minutes** - Traces can take time to appear in GCP

2. **Check sampling ratio** - Default is 10%, try 100% for testing:
   ```go
   config.TraceRatio = 1.0
   ```

3. **Verify project ID**:
   ```bash
   echo $GOOGLE_CLOUD_PROJECT
   ```

4. **Check GCP Console**:
   - Go to Trace > Trace List
   - Ensure you're viewing the correct project
   - Adjust the time range

## Resources

- [Task Documentation](https://taskfile.dev/)
- [Go Documentation](https://go.dev/doc/)
- [OpenTelemetry Go](https://opentelemetry.io/docs/instrumentation/go/)
- [GCP Cloud Logging](https://cloud.google.com/logging/docs)
- [GCP Cloud Trace](https://cloud.google.com/trace/docs)
- [golangci-lint](https://golangci-lint.run/)

## Getting Help

- Check the [README.md](./README.md) for usage documentation
- Check the [CLAUDE.md](./CLAUDE.md) for architectural decisions
- Check the [examples/](./examples/) directory for working code
- Review the [OpenTelemetry documentation](https://opentelemetry.io/docs/instrumentation/go/)
- Review the [GCP documentation](https://cloud.google.com/docs)
