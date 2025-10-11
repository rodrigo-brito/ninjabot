# Contributing to Ninjabot

First off, thank you for considering contributing to Ninjabot! It's people like you that make Ninjabot such a great tool.

## Table of Contents

- [Code of Conduct](#code-of-conduct)
- [Getting Started](#getting-started)
- [Development Setup](#development-setup)
- [How Can I Contribute?](#how-can-i-contribute)
- [Style Guidelines](#style-guidelines)
- [Commit Messages](#commit-messages)
- [Pull Request Process](#pull-request-process)
- [Testing](#testing)
- [Documentation](#documentation)

## Code of Conduct

This project and everyone participating in it is governed by our [Code of Conduct](CODE_OF_CONDUCT.md). By participating, you are expected to uphold this code. Please report unacceptable behavior to the project maintainers.

## Getting Started

- Make sure you have a [GitHub account](https://github.com/signup/free)
- Fork the repository on GitHub
- Check out the [issues](https://github.com/rodrigo-brito/ninjabot/issues) for things to work on

## Development Setup

### Prerequisites

- **Go 1.18 or higher** - [Download Go](https://golang.org/dl/)
- **Git** - [Download Git](https://git-scm.com/downloads)
- **Make** (optional, but recommended)

### Setting Up Your Development Environment

1. **Fork and clone the repository:**

   ```bash
   git clone https://github.com/YOUR-USERNAME/ninjabot.git
   cd ninjabot
   ```

2. **Add the upstream repository:**

   ```bash
   git remote add upstream https://github.com/rodrigo-brito/ninjabot.git
   ```

3. **Install dependencies:**

   ```bash
   go mod download
   ```

4. **Verify your setup:**

   ```bash
   make test
   # or
   go test -race -cover ./...
   ```

5. **Install development tools:**

   ```bash
   # Install golangci-lint for linting
   go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
   
   # Install mockery for generating mocks
   go install github.com/vektra/mockery/v2@latest
   ```

## How Can I Contribute?

### Reporting Bugs

Before creating bug reports, please check the existing issues to avoid duplicates. When you create a bug report, include as many details as possible:

- **Use a clear and descriptive title**
- **Describe the exact steps to reproduce the problem**
- **Provide specific examples** (code snippets, configuration files, etc.)
- **Describe the behavior you observed** and what you expected
- **Include logs and error messages**
- **Specify your environment** (OS, Go version, etc.)

**Bug Report Template:**

```markdown
**Description:**
A clear description of the bug.

**Steps to Reproduce:**
1. Step one
2. Step two
3. Step three

**Expected Behavior:**
What you expected to happen.

**Actual Behavior:**
What actually happened.

**Environment:**
- OS: [e.g., Ubuntu 22.04]
- Go Version: [e.g., 1.21.0]
- Ninjabot Version: [e.g., v1.0.0]

**Additional Context:**
Any other relevant information.
```

### Suggesting Enhancements

Enhancement suggestions are tracked as GitHub issues. When creating an enhancement suggestion:

- **Use a clear and descriptive title**
- **Provide a detailed description** of the suggested enhancement
- **Explain why this enhancement would be useful**
- **Include code examples** if applicable
- **List any alternatives** you've considered

### Your First Code Contribution

Unsure where to begin? Look for issues labeled:

- `good first issue` - Good for newcomers
- `help wanted` - Issues that need assistance
- `documentation` - Documentation improvements

### Pull Requests

1. **Create a branch** for your changes:

   ```bash
   git checkout -b feature/my-new-feature
   # or
   git checkout -b fix/issue-123
   ```

2. **Make your changes** following our style guidelines

3. **Write or update tests** for your changes

4. **Run tests and linting:**

   ```bash
   make test
   make lint
   # or
   go test -race -cover ./...
   golangci-lint run --fix
   ```

5. **Commit your changes** with a clear commit message

6. **Push to your fork:**

   ```bash
   git push origin feature/my-new-feature
   ```

7. **Open a Pull Request** against the `main` branch

## Style Guidelines

### Go Code Style

We follow standard Go conventions and use `golangci-lint` for enforcement.

**Key Guidelines:**

- **Follow [Effective Go](https://golang.org/doc/effective_go.html)**
- **Use `gofmt`** - All code must be formatted with `gofmt`
- **Run `golangci-lint`** - Fix all linting issues before submitting
- **Write idiomatic Go** - Prefer simplicity and clarity
- **Keep functions small** - Each function should do one thing well
- **Avoid premature optimization** - Prioritize readability

**Naming Conventions:**

- Use `camelCase` for unexported names
- Use `PascalCase` for exported names
- Use descriptive names (avoid single-letter variables except in short scopes)
- Interface names should end in `-er` when possible (e.g., `Broker`, `Notifier`)

**Error Handling:**

- Always check errors
- Wrap errors with context using `fmt.Errorf("context: %w", err)`
- Don't panic in library code
- Return errors rather than logging them (let callers decide)

**Example:**

```go
// Good
func (b *Bot) Start(ctx context.Context) error {
    if err := b.validate(); err != nil {
        return fmt.Errorf("validation failed: %w", err)
    }
    return nil
}

// Bad
func (b *Bot) Start(ctx context.Context) {
    if err := b.validate(); err != nil {
        log.Fatal(err) // Don't panic/fatal in library code
    }
}
```

### Documentation Style

- **All exported functions, types, and constants must have comments**
- **Comments should be complete sentences** starting with the name of the thing being described
- **Package comments** should describe the package's purpose
- **Use examples** where helpful

**Example:**

```go
// Strategy defines the interface that all trading strategies must implement.
// A strategy receives candle data and makes trading decisions through the broker.
type Strategy interface {
    // Timeframe returns the time interval for strategy execution (e.g., "1h", "1d").
    Timeframe() string
}
```

### Testing Style

- Write table-driven tests where appropriate
- Use meaningful test names: `TestFunctionName_Scenario_ExpectedBehavior`
- Use subtests with `t.Run()` for multiple test cases
- Mock external dependencies
- Aim for >80% code coverage

**Example:**

```go
func TestBot_Start_WithValidConfig_Succeeds(t *testing.T) {
    tests := []struct {
        name    string
        config  Config
        wantErr bool
    }{
        {
            name:    "valid config",
            config:  validConfig(),
            wantErr: false,
        },
        // more test cases...
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // test implementation
        })
    }
}
```

## Commit Messages

We follow the [Conventional Commits](https://www.conventionalcommits.org/) specification.

**Format:**

```
<type>(<scope>): <subject>

<body>

<footer>
```

**Types:**

- `feat`: A new feature
- `fix`: A bug fix
- `docs`: Documentation only changes
- `style`: Code style changes (formatting, missing semicolons, etc.)
- `refactor`: Code refactoring without changing functionality
- `perf`: Performance improvements
- `test`: Adding or updating tests
- `chore`: Maintenance tasks, dependency updates, etc.
- `ci`: CI/CD changes

**Examples:**

```
feat(strategy): add support for custom indicators

Add ability to register custom indicators in strategies.
This allows users to implement their own technical indicators
beyond the built-in ones.

Closes #123
```

```
fix(exchange): handle rate limiting correctly

Previously, rate limit errors were not properly handled,
causing the bot to crash. Now we retry with exponential backoff.

Fixes #456
```

```
docs(readme): update installation instructions

Add prerequisites section and clarify Go version requirements.
```

**Guidelines:**

- Use the imperative mood ("add feature" not "added feature")
- Keep the subject line under 50 characters
- Capitalize the subject line
- Don't end the subject line with a period
- Separate subject from body with a blank line
- Wrap the body at 72 characters
- Reference issues and pull requests in the footer

## Pull Request Process

1. **Ensure your PR:**
   - Follows the style guidelines
   - Includes tests for new functionality
   - Updates documentation as needed
   - Passes all CI checks
   - Has a clear description of changes

2. **Fill out the PR template** completely

3. **Link related issues** using keywords (Fixes #123, Closes #456)

4. **Request review** from maintainers

5. **Address review feedback** promptly

6. **Squash commits** if requested (we may squash on merge)

7. **Wait for approval** - At least one maintainer must approve

8. **Merge** - Maintainers will merge once approved

**PR Title Format:**

Follow the same format as commit messages:

```
feat(exchange): add support for Coinbase
fix(bot): prevent race condition in order processing
docs(contributing): add commit message guidelines
```

## Testing

### Running Tests

```bash
# Run all tests
make test

# Run tests with coverage
go test -race -cover ./...

# Run tests for a specific package
go test ./exchange/...

# Run a specific test
go test -run TestBotStart ./...

# Generate coverage report
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

### Writing Tests

- **Unit tests** - Test individual functions in isolation
- **Integration tests** - Test component interactions
- **Backtesting** - Validate strategies with historical data

**Test Requirements:**

- All new features must include tests
- Bug fixes should include regression tests
- Maintain or improve code coverage
- Tests must pass on all supported Go versions

### Mocking

We use [mockery](https://github.com/vektra/mockery) to generate mocks:

```bash
# Generate mocks
make generate
# or
go generate ./...
```

Mocks are stored in `testdata/mocks/`.

## Documentation

### Code Documentation

- Document all exported identifiers
- Use godoc format
- Include examples where helpful
- Keep documentation up-to-date with code changes

### User Documentation

- Update README.md for user-facing changes
- Add examples for new features
- Update external documentation site if needed

### Generating Documentation

```bash
# View godoc locally
godoc -http=:6060
# Then visit http://localhost:6060/pkg/github.com/rodrigo-brito/ninjabot/
```

## Questions?

- **Open an issue** for questions about contributing
- **Join our Discord** - [Discord Link](https://discord.gg/TGCrUH972E)
- **Check existing issues and PRs** - Your question may already be answered

## Recognition

Contributors will be recognized in:

- GitHub contributors list
- Release notes (for significant contributions)
- Project README (for major contributions)

Thank you for contributing to Ninjabot! 🥷
