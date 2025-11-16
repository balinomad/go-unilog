# Contributing to unilog

Thank you for your interest in contributing! This document covers the GitHub workflow, code standards, and review process.

For **handler implementation**, see [docs/HANDLER_DEVELOPMENT.md](docs/HANDLER_DEVELOPMENT.md).

---

## Code of Conduct

Be respectful, constructive, and professional. We follow the [Go Community Code of Conduct](https://golang.org/conduct).

---

## Before You Start

### Good First Issues

Look for issues labeled `good first issue` or `help wanted`. These are well-scoped and suitable for new contributors.

### Discuss Major Changes

For significant changes (new features, breaking changes, architectural shifts):

1. Open a GitHub Discussion or Issue **before** coding
2. Describe the problem and proposed solution
3. Wait for maintainer feedback
4. Proceed once approach is agreed upon

This avoids wasted effort on rejected PRs.

---

## Development Setup

### Prerequisites

- Go 1.24 or later
- Git
- (Optional) `gocyclo` for complexity checks: `go install github.com/fzipp/gocyclo/cmd/gocyclo@latest`

### Clone and Setup

```bash
git clone https://github.com/balinomad/go-unilog.git
cd go-unilog

# Run tests
make test

# Run tests with coverage
make cover

# Run full test suite
make fulltest
```

### Project Structure

```
go-unilog/
├── handler/              # Handler implementations
│   ├── slog/            # Standard library adapter
│   ├── zap/             # Zap adapter
│   ├── stdlog/          # Stdlib log adapter
│   └── ...              # Other handlers
├── io/                  # I/O utilities (multi, rotating, lumberjack)
├── docs/                # Documentation
├── *.go                 # Core unilog package
└── *_test.go            # Tests
```

---

## Making Changes

### 1. Fork and Branch

```bash
# Fork on GitHub, then clone your fork
git clone https://github.com/YOUR_USERNAME/go-unilog.git
cd go-unilog

# Add upstream remote
git remote add upstream https://github.com/balinomad/go-unilog.git

# Create feature branch
git checkout -b feature/my-feature
```

### 2. Write Code

Follow [Code Standards](#code-standards) below.

### 3. Write Tests

- All new code requires tests
- Aim for 80%+ coverage on new code
- Use table-driven tests
- Test error paths and edge cases

```bash
# Run tests
go test ./...

# Run with race detector
go test -race ./...

# Check coverage
make cover
```

### 4. Update Documentation

- Add/update GoDoc comments for public APIs
- Update README.md if user-facing behavior changes
- Update docs/HANDLERS.md for new/changed handlers
- Add examples for new features

### 5. Commit

Write clear, atomic commits:

```bash
git add .
git commit -m "handler/zap: fix caller skip calculation

- Adjust skip offset to account for wrapper frames
- Add test case for nested function calls
- Fixes #123"
```

**Commit message format:**
- First line: `<scope>: <summary>` (50 chars max)
- Blank line
- Detailed explanation (72 chars per line)
- Reference issues: `Fixes #123` or `Relates to #456`

**Scopes**: `handler/zap`, `core`, `docs`, `io/rotating`, `tests`, etc.

### 6. Push and Open PR

```bash
git push origin feature/my-feature
```

Open a Pull Request on GitHub:

- Fill out the PR template
- Link related issues
- Describe what changed and why
- Include test results (if relevant)

---

## Code Standards

### General

- Follow [Effective Go](https://golang.org/doc/effective_go.html)
- Use `gofmt` (enforced by CI)
- Keep functions under 15 cyclomatic complexity (`gocyclo`)
- No trailing whitespace
- Avoid `panic()` in library code (use `error` returns)

### Naming

- **Exported**: Use descriptive names (`Logger`, `SetLevel`, `WithAttrs`)
- **Unexported**: Concise names acceptable (`h` for handler, `ctx` for context)
- **Avoid**: Stuttering (`handler.HandlerInterface` → `handler.Handler`)

### Error Handling

- **Always check errors**: Never ignore `error` returns
- **Wrap errors**: Use `fmt.Errorf("context: %w", err)` for context
- **Sentinel errors**: Use `errors.New()` for package-level sentinels
- **Return early**: Avoid deep nesting

```go
// Good
func DoWork() error {
    if err := step1(); err != nil {
        return fmt.Errorf("step1 failed: %w", err)
    }
    if err := step2(); err != nil {
        return fmt.Errorf("step2 failed: %w", err)
    }
    return nil
}

// Avoid
func DoWork() error {
    err := step1()
    if err == nil {
        err = step2()
        if err == nil {
            // ...
        }
    }
    return err
}
```

### Concurrency

- **Document**: Clearly state if type is safe for concurrent use
- **Use mutexes**: For shared mutable state
- **Use atomics**: For simple counters/flags
- **Avoid locks in hot paths**: Cache values, use atomics

```go
// Safe: mutex for configuration, atomic for level check
type handler struct {
    mu    sync.RWMutex
    level atomic.Int32

    config Config
}

func (h *handler) Enabled(level LogLevel) bool {
    return level >= LogLevel(h.level.Load()) // Lock-free
}

func (h *handler) SetConfig(c Config) {
    h.mu.Lock()
    defer h.mu.Unlock()
    h.config = c
}
```

### Testing

- **Package**: Use `*_test` package for black-box tests
- **Coverage**: Aim for 80%+ on new code
- **Table-driven**: Use for multiple test cases
- **Cleanup**: Use `t.TempDir()`, defer cleanup
- **Parallel**: Use `t.Parallel()` when safe

```go
func TestHandler_Enabled(t *testing.T) {
    t.Parallel()

    tests := []struct {
        name       string
        configured LogLevel
        tested     LogLevel
        want       bool
    }{
        {"below", InfoLevel, DebugLevel, false},
        {"at", InfoLevel, InfoLevel, true},
        {"above", InfoLevel, ErrorLevel, true},
    }

    for _, tt := range tests {
        tt := tt
        t.Run(tt.name, func(t *testing.T) {
            t.Parallel()
            h := newTestHandler(tt.configured)
            if got := h.Enabled(tt.tested); got != tt.want {
                t.Errorf("Enabled(%v) = %v, want %v", tt.tested, got, tt.want)
            }
        })
    }
}
```

### Documentation

- **GoDoc**: All exported types, functions, constants
- **End with period**: GoDoc comments end with `.`
- **Examples**: Provide `Example*` tests for key features
- **Inline comments**: Start with capital, no period (unless multi-sentence)

```go
// Logger is the main logging interface.
// It provides convenience methods for logging at specific levels.
type Logger interface {
    // Info logs a message at the info level.
    Info(ctx context.Context, msg string, keyValues ...any)
}

func (h *handler) process(r *Record) {
    // Convert level to backend format
    level := h.mapper.Map(r.Level)
}
```

---

## Pull Request Process

### Review Checklist

Before submitting, verify:

- [ ] Tests pass locally (`go test ./...`)
- [ ] Race detector clean (`go test -race ./...`)
- [ ] `gofmt` applied
- [ ] `gocyclo` under 15 for changed functions
- [ ] Coverage ≥ 80% on new code
- [ ] GoDoc comments on public APIs
- [ ] No breaking changes (or explicitly documented)
- [ ] Commit messages follow format

### CI Checks

GitHub Actions automatically runs:

- `go test` on multiple Go versions
- `go test -race` for race conditions
- Coverage reporting to Codecov
- `gofmt` verification

PRs cannot merge until CI passes.

### Review Process

1. **Maintainer reviews**: Focus on correctness, design, test coverage
2. **Feedback**: Address review comments or discuss alternatives
3. **Approval**: At least one maintainer approval required
4. **Merge**: Squash-merge to `main` (maintainers only)

### Response Time

- **Initial review**: Within 3 business days
- **Follow-up**: Within 2 business days after changes
- **Questions**: Usually within 1 business day

---

## Contribution Types

### Bug Fixes

1. Open an issue describing the bug (if none exists)
2. Include: steps to reproduce, expected vs actual behavior
3. Submit PR with fix and regression test
4. Reference issue in commit message

### New Features

1. Open a Discussion or Issue proposing the feature
2. Wait for maintainer approval before coding
3. Submit PR with feature + tests + docs
4. Update CHANGELOG.md (if exists)

### Handler Implementations

See [docs/HANDLER_DEVELOPMENT.md](docs/HANDLER_DEVELOPMENT.md) for detailed guide.

Key points:
- Implement `handler.Handler` interface
- Use `handler.ComplianceTest()` in tests
- Create handler-specific README.md
- Update docs/HANDLERS.md feature matrix

### Documentation Improvements

- Fix typos, clarify confusing sections
- Add examples for undocumented features
- Improve GoDoc comments
- No code changes needed for doc-only PRs

---

## Communication Channels

- **GitHub Issues**: Bug reports, feature requests
- **GitHub Discussions**: Questions, design discussions, RFC
- **Pull Requests**: Code review, implementation feedback

**Do not** use Issues for:
- General Go questions (use Stack Overflow)
- Support requests (use Discussions)
- Off-topic discussions

---

## Release Process

(Maintainers only)

1. Update CHANGELOG.md with release notes
2. Tag release: `git tag -a v0.x.0 -m "Release v0.x.0"`
3. Push tag: `git push origin v0.x.0`
4. GitHub Actions publishes to pkg.go.dev
5. Announce in Discussions

---

## Licensing

By contributing, you agree that your contributions will be licensed under the MIT License (same as the project).

You confirm that:
- You have the right to submit the contribution
- Your contribution is your original work
- You grant the project maintainers a perpetual, worldwide, non-exclusive license to use your contribution

No Contributor License Agreement (CLA) required - MIT License suffices.

---

## Questions?

- **Code questions**: Open a Discussion
- **Process questions**: Open an Issue
- **Security issues**: Email maintainers (see SECURITY.md, if exists)

Thank you for contributing to unilog!