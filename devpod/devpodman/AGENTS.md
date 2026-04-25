# Devpodman — Agent Guide

## Personality

* You like to solve and analyze problems first and foremost, you are not an entertainer.
* You do not "sugarcoat" uncomfortable truths or opinions that might antagonize the user.

## Project Overview

CLI tool that reads `devcontainer.json` and starts Podman pods. Schema-driven design using CUE for type generation.

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
go-task test:package PACKAGE=./pkg/devcontainer/...

# Single test by name
go-task test:run TEST=TestLoad

# Build binary
go-task build

# Build with version ldflags (like goreleaser)
go-task build:ldflags VERSION=1.0.0 COMMIT=abc123 DATE=2026-01-01

# Format code
go-task fmt

# Regenerate model types from CUE
go-task cue:generate

# Validate a devcontainer.json
go-task validate PATH=path/to/devcontainer.json
```

### Raw equivalents

```bash
# Run all tests
go test -tags containers_image_openpgp ./...

# Run tests for a single package
go test -tags containers_image_openpgp ./internal/podman/...

# Run a single test
go test -tags containers_image_openpgp ./pkg/devcontainer/... -run TestLoad/loads_image-based_devcontainer -v

# Build
go build -tags containers_image_openpgp ./cmd/devpodman/

# Run CLI
go run -tags containers_image_openpgp ./cmd/devpodman/ debug --validate path/to/devcontainer.json

# Regenerate model types from CUE (must run from pkg/model/ directory)
cd pkg/model && cue exp gengotypes .
```

## Architecture

```
devpodman/
├── cmd/devpodman/       # CLI entry point (urfave/cli/v3)
├── internal/
│   ├── cli/             # urfave command factories + actions
│   └── podman/          # Podman client + XDG-aware config loading
├── pkg/
│   ├── engine/          # Public API: Play, Down, orchestration
│   ├── devcontainer/    # Parses/validates devcontainer.json
│   ├── effects/         # Command pattern for side effects
│   └── model/           # CUE schema + auto-generated Go types
│       ├── devcontainer.cue            # Source of truth — CUE schema
│       ├── schema.go                   # Embeds CUE schema for runtime
│       └── cue_types_model_gen.go      # Generated Go types (DO NOT EDIT)
```

- `model/` files are generated from `model/devcontainer.cue` — never hand-edit `cue_types_model_gen.go`
- `devcontainer/` is the domain layer: `Load(data []byte)` returns `*ResolvedConfig` with CUE-validated build/image/common/noncompose configs
- `podman/` owns connection management: `Config` via koanf + env vars, `Client` wraps podman bindings, validates connection on creation
- `engine/` is the public API: accepts `EngineConnection` (context alias), returns `effects.Compound` sequences
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

### Pointer Helpers for Primitives

When passing primitive values as pointers to function calls, use helper functions
instead of creating temporary variables:

```go
// ✅ Good
options := &SomeOptions{
    Enabled: ptrBool(true),
    Count:   ptrInt(5),
}

// ❌ Avoid — creates unnecessary intermediate variables
enabled := true
count := 5
options := &SomeOptions{
    Enabled: &enabled,
    Count:   &count,
}
```

Define helpers in a `helpers.go` file within the package:

```go
func ptrBool(b bool) *bool { return &b }
func ptrInt(i int) *int    { return &i }
```

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
| `cuelang.org/go` | CUE schema evaluation and runtime validation |

## CUE Workflow

The CUE schema at `model/devcontainer.cue` is the source of truth for devcontainer configuration types.
After modifying it, regenerate Go types into `model/cue_types_model_gen.go`:

```bash
cd model && cue exp gengotypes .
```

Generated types support both `json:` struct tags for JSON (un)marshalling.
The schema is embedded at build time via `//go:embed` and used at runtime by `cuelang.org/go` for validation.
Go parses JSON into the generated structs, then CUE validates each struct against its corresponding definition
(`#dockerfileContainer`, `#imageContainer`, `#devContainerCommon`, `#nonComposeBase`).

