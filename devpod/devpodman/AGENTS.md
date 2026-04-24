# Devpodman — Agent Guide

## Personality

* You like to solve and analyze problems first and foremost, you are not an entertainer.
* You do not "sugarcoat" uncomfortable truths or opinions that might antagonize the user.

## Project Overview

CLI tool that reads `devcontainer.json` and starts Podman pods. Schema-driven design using Pkl for type generation.

## Commands

All `go test` and `go build` commands require the `-tags containers_image_openpgp` build tag (from `.goreleaser.yaml`). Omitting it will cause build failures.

### Taskfile shortcuts

The project has a `Taskfile.yaml`. The task runner binary is `go-task` (not `task`, which is Taskwarrior).

```bash
# Run all tests
go-task test

# Verbose test output
go-task test:verbose

# Single package
go-task test:package PACKAGE=./devcontainer/...

# Single test by name
go-task test:run TEST=TestLoad

# Build binary
go-task build

# Build with version ldflags (like goreleaser)
go-task build:ldflags VERSION=1.0.0 COMMIT=abc123 DATE=2026-01-01

# Format code
go-task fmt

# Regenerate model types from Pkl
go-task pkl:generate

# Validate a devcontainer.json
go-task validate PATH=path/to/devcontainer.json
```

### Raw equivalents

```bash
# Run all tests
go test -tags containers_image_openpgp ./...

# Run tests for a single package
go test -tags containers_image_openpgp ./podman/...

# Run a single test
go test -tags containers_image_openpgp ./devcontainer/... -run TestLoad/loads_image-based_devcontainer -v

# Build
go build -tags containers_image_openpgp ./cmd/devpodman/

# Run CLI
go run -tags containers_image_openpgp ./cmd/devpodman/ debug --validate path/to/devcontainer.json

# Regenerate model types from Pkl
pkl run @go/gen.pkl pkl/Schema.pkl
```

## Architecture

```
devpodman/
├── cmd/devpodman/       # CLI entry point (urfave/cli/v3)
├── devcontainer/        # Parses/validates devcontainer.json
├── effects/             # Command pattern for side effects (FileWrite, FileDelete, etc.)
├── model/               # Auto-generated from Pkl — DO NOT EDIT
│   ├── build/           #   BuildProperties, BuildProps
│   ├── common/          #   CommonProperties, Mount, MountType
│   └── image/           #   ImageProperties
├── podman/              # Podman client + XDG-aware config loading
└── pkl/                 # Pkl schema source files (source of truth)
```

- `model/` files are generated from `pkl/` — never hand-edit them
- `devcontainer/` is the domain layer: `Load(data []byte)` returns typed build/image/common configs
- `podman/` owns connection management: `Config` via koanf + env vars, `Client` wraps podman bindings
- `effects/` provides composable side-effect operations with fail-fast and error collection

## Methodology
- Use Test Driven Development

## Code Style

### Imports
- Standard library first, blank line, third-party, blank line, local imports

### Formatting
- `gofmt` always. No exceptions. Tabs for indentation.

### Types
- Use model types from `model/` directly — do not duplicate structs
- Pointer fields for optional values, value types for required fields
- Do not use `any` when a concrete type exists

### Naming
- Packages: short, lowercase, no underscores (e.g., `devcontainer`, not `dev_container`)
- Variables: `camelCase`, descriptive but concise (`commonProps`, not `cp`)
- Test subtest names: lowercase with spaces (`"loads image-based devcontainer"`)
- Test table entries: `tt` (e.g., `for _, tt := range tests`)

### Error Handling
- **Prefer separate variable assignment over inline `if err :=`:**

```go
// ✅ Good — easier to debug, inspect values in debugger
val, err := someFunction()
if err != nil {
    return fmt.Errorf("failed to do thing: %w", err)
}

// ❌ Avoid — harder to inspect `val` when error occurs
if val, err := someFunction(); err != nil {
    return err
}
```

- Wrap errors with context: `fmt.Errorf("failed to parse devcontainer.json: %w", err)`
- Validate at the boundary — return errors early, don't pass invalid state deeper
- Error messages: lowercase, no trailing punctuation, describe what went wrong

### Testing
- Standard library `testing` only — no assertion libraries
- Table-driven tests via `t.Run()` subtests
- `t.Fatalf()` for failures, `t.Errorf()` for non-fatal mismatches
- Use `t.TempDir()` for temporary file tests
- Use `t.Skipf()` for tests requiring external resources (e.g., podman socket)
- Clean up env vars with `defer os.Unsetenv(...)` after `os.Setenv`
- Test happy paths, error paths, and edge cases
- Run `go test ./...` before considering work complete

## Dependency Guidelines

| Module | Purpose |
|--------|---------|
| `github.com/urfave/cli/v3` | CLI framework |
| `github.com/knadh/koanf/v2` | Layered config loading (env vars, defaults) |
| `github.com/adrg/xdg` | XDG Base Directory spec (podman socket discovery) |
| `github.com/containers/podman/v5` | Podman Go bindings |
| `github.com/apple/pkl-go` | Pkl schema evaluation and code generation |

## Pkl Workflow

Pkl schema files in `pkl/` are the source of truth. After modifying them, regenerate Go types into `model/`. The generated files have both `pkl:` and `json:` struct tags.

