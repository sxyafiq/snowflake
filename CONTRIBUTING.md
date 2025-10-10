# Contributing to Snowflake

Thank you for your interest in contributing! This document provides guidelines and instructions for contributing to this project.

## Table of Contents

- [Code of Conduct](#code-of-conduct)
- [Getting Started](#getting-started)
- [Development Setup](#development-setup)
- [Making Changes](#making-changes)
- [Commit Message Convention](#commit-message-convention)
- [Pull Request Process](#pull-request-process)
- [Code Style](#code-style)
- [Testing](#testing)

---

## Code of Conduct

This project adheres to the [Contributor Covenant Code of Conduct](CODE_OF_CONDUCT.md). By participating, you are expected to uphold this code.

---

## Getting Started

1. Fork the repository on GitHub
2. Clone your fork locally
3. Set up the development environment
4. Create a new branch for your changes
5. Make your changes
6. Submit a pull request

---

## Development Setup

### Prerequisites

- Go 1.21 or higher
- Git
- Make (optional, for convenience commands)

### Installation

```bash
# Clone your fork
git clone https://github.com/YOUR_USERNAME/snowflake.git
cd snowflake

# Add upstream remote
git remote add upstream https://github.com/sxyafiq/snowflake.git

# Install dependencies
go mod download

# Run tests to verify setup
make test
# or
go test ./...
```

---

## Making Changes

### Branch Naming

Use descriptive branch names:

```bash
git checkout -b feat/add-new-encoding
git checkout -b fix/clock-drift-issue
git checkout -b docs/update-readme
```

### Before Committing

1. **Format your code**
   ```bash
   make fmt
   # or
   go fmt ./...
   ```

2. **Run linter**
   ```bash
   make lint
   # or
   golangci-lint run
   ```

3. **Run tests**
   ```bash
   make test
   # or
   go test -v -race ./...
   ```

4. **Check test coverage**
   ```bash
   make coverage
   # or
   go test -coverprofile=coverage.out ./...
   go tool cover -html=coverage.out
   ```

---

## Commit Message Convention

This project uses [Conventional Commits](https://www.conventionalcommits.org/) for clear and consistent commit history.

### Format

```
<type>(<scope>): <subject>

<body>

<footer>
```

### Type

Must be one of the following:

- **feat**: A new feature
- **fix**: A bug fix
- **docs**: Documentation only changes
- **style**: Changes that don't affect code meaning (formatting, whitespace)
- **refactor**: Code change that neither fixes a bug nor adds a feature
- **perf**: Performance improvement
- **test**: Adding or updating tests
- **chore**: Changes to build process or auxiliary tools
- **ci**: Changes to CI configuration files and scripts

### Scope

Optional. Indicates which part of the codebase is affected:

- `generator` - ID generator core
- `encoding` - Encoding/decoding functions
- `id` - ID type and methods
- `metrics` - Metrics and observability
- `tests` - Test files
- `docs` - Documentation

### Examples

```bash
# Feature
git commit -m "feat(generator): add context support for graceful cancellation"

# Bug fix
git commit -m "fix(encoding): correct Base62 character ordering"

# Documentation
git commit -m "docs(readme): add production deployment examples"

# Performance
git commit -m "perf(encoding): optimize Base58 encoding with lookup table"

# Breaking change
git commit -m "feat(generator)!: change default epoch to 2024-01-01

BREAKING CHANGE: Default epoch changed from 2010 to 2024.
Existing IDs will have different timestamp values when decoded."
```

### Full Example

```
feat(sharding): add time-based sharding method

Add ShardByTime() method that partitions IDs based on timestamp
buckets. Useful for time-series data partitioning.

Closes #42
```

---

## Pull Request Process

### 1. Update Your Fork

```bash
git fetch upstream
git rebase upstream/main
```

### 2. Create a Pull Request

- Use a clear, descriptive title following conventional commit format
- Fill out the PR template completely
- Link related issues using keywords (Fixes #123, Closes #456)
- Ensure all CI checks pass

### 3. PR Title Format

```
feat(scope): brief description
fix(scope): brief description
docs: brief description
```

### 4. PR Description Should Include

- **What**: What changes does this PR introduce?
- **Why**: Why are these changes needed?
- **How**: How were these changes implemented?
- **Testing**: How were the changes tested?
- **Screenshots**: If applicable (UI changes, documentation)

### Example PR Description

```markdown
## What

Adds support for Base62URL encoding, an alternative URL-safe format.

## Why

Some systems prefer URL-safe encodings without padding characters.
Base62URL provides a cleaner alternative to Base64URL.

## How

- Implemented `Base62URL()` encoding method
- Implemented `ParseBase62URL()` parsing function
- Added character set: `0-9A-Za-z_-`
- Added comprehensive tests

## Testing

- Unit tests for encoding/decoding
- Edge case tests (empty, max values)
- Round-trip tests
- Benchmarks show <5% overhead vs Base62

Closes #123
```

### 5. Review Process

- Maintainers will review your PR
- Address feedback and push updates
- Once approved, a maintainer will merge

---

## Code Style

### Go Code Style

Follow standard Go conventions:

- Run `go fmt` on all code
- Follow [Effective Go](https://golang.org/doc/effective_go.html)
- Use meaningful variable names
- Write clear, concise comments
- Document all exported functions, types, and constants

### Documentation Style

- Use present tense ("Add feature" not "Added feature")
- Be concise but clear
- Include code examples for new features
- Update README.md if adding user-facing features

### Example Documentation

```go
// Base62URL returns a URL-safe Base62 encoded string.
//
// Unlike Base64URL, this encoding uses only alphanumeric characters
// without padding, making it suitable for URLs, filenames, and databases.
//
// Character set: 0-9, A-Z, a-z, _, -
//
// Performance: ~820ns per operation, 1 allocation
//
// Example:
//
//	id, _ := snowflake.GenerateID()
//	encoded := id.Base62URL() // "7n42dgm5tflk_A"
func (id ID) Base62URL() string {
	return encodeBase62URL(int64(id))
}
```

---

## Testing

### Writing Tests

- Write table-driven tests for functions with multiple cases
- Use subtests for better organization
- Test edge cases (zero, max, negative values)
- Include benchmarks for performance-critical code

### Test Coverage

- Aim for >70% coverage for new code
- All public APIs must have tests
- Critical paths (ID generation, encoding) should have 100% coverage

### Example Test

```go
func TestIDBase62URL(t *testing.T) {
	tests := []struct {
		name  string
		id    ID
		want  string
	}{
		{"zero", 0, "0"},
		{"small", 123, "1Z"},
		{"large", 9223372036854775807, "AzL8n0Y58m7"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.id.Base62URL()
			if got != tt.want {
				t.Errorf("Base62URL() = %v, want %v", got, tt.want)
			}

			// Test round-trip
			parsed, err := ParseBase62URL(got)
			if err != nil {
				t.Fatalf("ParseBase62URL() error = %v", err)
			}
			if parsed != tt.id {
				t.Errorf("Round-trip failed: got %v, want %v", parsed, tt.id)
			}
		})
	}
}
```

### Running Tests

```bash
# All tests
make test

# With race detector
make test-race

# With coverage
make coverage

# Specific package
go test ./encoding

# Specific test
go test -run TestIDBase62URL

# Benchmarks
make bench
```

---

## Questions?

- Open an issue for bugs or feature requests
- Start a [discussion](https://github.com/sxyafiq/snowflake/discussions) for questions
- Check existing issues and discussions first

---

Thank you for contributing! 🎉
