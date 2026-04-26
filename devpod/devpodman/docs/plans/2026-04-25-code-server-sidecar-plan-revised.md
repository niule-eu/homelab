# Code-Server Sidecar Implementation Plan (Revised)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement `devpodman play` and `devpodman down` commands with a clean architecture: `pkg/engine` as the public API returning effect sequences, `internal/podman` as a thin bindings wrapper, `internal/cli` for urfave command factories, following golang-standards/project-layout.

**Architecture:** `pkg/engine` defines the public API (`Engine.Play`, `Engine.Down`) that accepts an `EngineConnection` (alias for `context.Context`) and returns `effects.Compound` sequences. The engine imports podman bindings directly — no custom podman client abstraction. All state-changing operations (build images, create pods, create volumes, start containers) are expressed as effects. The CLI layer in `internal/cli` creates the podman connection context, calls engine methods, and applies the returned effects.

**Tech Stack:** Go 1.25, `urfave/cli/v3`, `containers/podman/v5` bindings, embedded Containerfile via `//go:embed`, effects command pattern

---

## Progress Status

**Completed:** Tasks 1-5 ✅
**Remaining:** Tasks 6-9

---

## Key Design Decisions (deviated from original plan)

1. **Connection validation** moved from `internal/cli` into `internal/podman.NewClient()` — fails fast on creation
2. **Engine effects** use constructor pattern: `NewXxxEffect(conn, ...) effects.Effect` — connection passed in constructor, not to `Apply()`
3. **No custom `Compound` in engine** — uses `pkg/effects.Compound` directly
4. **`pkg/engine/connection.go`** — only contains the `EngineConnection` type alias, no `ValidateConnection` (validation is in podman package)
5. **Effect tests** use real podman integration tests with `testConn()` helper
6. **Pod name** in `SpecGenerator.Pod` field uses plain name (e.g., `"my-pod"`), NOT `"pod:my-pod"` prefix
7. **Effect `Apply()` returns `error` only** — no typed reports returned from effects (matches existing `pkg/effects.Effect` interface)
8. **`DerivePodName` is exported** from `pkg/engine` — CLI computes pod name independently; `Play` returns `(effects.Compound, error)`
9. **Containerfile is fully Go-templated** — no ARG directives; all variables (versions, UID, GID) substituted at render time
10. **Connections config** uses a simple template function `RenderConnectionsConfig(socketPath string)` — no `podName` parameter
11. **Image cleanup in `Down`** omitted for now; `--delete-images` flag accepted but no-op
12. **`cmd/devpodman/debug.go`** is stale dead code — deleted in Task 9; debug logic lives in `internal/cli/commands.go`

---

## Current File Structure

```
devpodman/
├── cmd/devpodman/
│   ├── main.go             # MODIFY: register play/down commands
│   └── debug.go            # DELETE: stale duplicate of internal/cli/commands.go
├── internal/
│   ├── cli/
│   │   ├── commands.go      # MODIFY: add NewPlayCommand, NewDownCommand
│   │   ├── commands_test.go # MODIFY: add tests for play/down commands
│   │   └── connections.go   # EXISTING: NewEngineConnection(ctx, cfg)
│   └── podman/
│       ├── client.go        # EXISTING (thin wrapper)
│       ├── client_test.go   # EXISTING
│       ├── config.go        # EXISTING (koanf + XDG)
│       ├── config_test.go   # EXISTING
│       ├── helpers.go       # EXISTING
│       ├── pods.go          # EXISTING
│       └── pods_test.go     # EXISTING
├── pkg/
│   ├── engine/
│   │   ├── engine.go        # NEW: Engine struct, Play, Down, DerivePodName
│   │   ├── engine_test.go   # NEW
│   │   ├── effects.go       # EXISTING: all effect types with constructor pattern
│   │   ├── effects_test.go  # EXISTING (fix stale volume test)
│   │   ├── sidecar.go       # NEW: sidecar Containerfile template, ImageTag, RenderConnectionsConfig
│   │   ├── sidecar_test.go  # NEW
│   │   ├── connection.go    # EXISTING: EngineConnection alias
│   │   ├── connection_test.go # EXISTING
│   │   └── assets/
│   │       ├── Containerfile    # NEW: go:embed (fully Go-templated, no ARG)
│   │       └── devpodman-shell  # NEW: go:embed (shell wrapper)
│   ├── devcontainer/
│   │   ├── devcontainer.go  # EXISTING
│   │   └── devcontainer_test.go # EXISTING
│   ├── effects/
│   │   ├── effects.go       # EXISTING: Effect interface, Compound, FileWrite, etc.
│   │   └── effects_test.go  # EXISTING
│   └── model/
│       ├── devcontainer.cue            # EXISTING
│       ├── schema.go                   # EXISTING
│       ├── schema_test.go              # EXISTING
│       └── cue_types_model_gen.go      # EXISTING
└── docs/
    └── plans/
        └── 2026-04-25-code-server-sidecar-plan-revised.md # THIS FILE
```

---

### Task 1: Move packages to new layout

**Files:**
- Move: `devcontainer/` → `pkg/devcontainer/`
- Move: `effects/` → `pkg/effects/`
- Move: `model/` → `pkg/model/`
- Move: `podman/` → `internal/podman/`

- [ ] **Step 1: Move directories**

```bash
mkdir -p pkg devpodman/internal
mv devcontainer pkg/devcontainer
mv effects pkg/effects
mv model pkg/model
mkdir -p internal/podman
mv podman/* internal/podman/
rmdir podman
```

- [ ] **Step 2: Update import paths in all moved files**

All imports of `github.com/niule-eu/devpodman/devcontainer` → `github.com/niule-eu/devpodman/pkg/devcontainer`
All imports of `github.com/niule-eu/devpodman/effects` → `github.com/niule-eu/devpodman/pkg/effects`
All imports of `github.com/niule-eu/devpodman/model` → `github.com/niule-eu/devpodman/pkg/model`
All imports of `github.com/niule-eu/devpodman/podman` → `github.com/niule-eu/devpodman/internal/podman`

- [ ] **Step 3: Update `cmd/devpodman/main.go`**

```go
package main

import (
	"context"
	"log"
	"os"

	"github.com/niule-eu/devpodman/internal/cli"
	"github.com/urfave/cli/v3"
)

func main() {
	app := &cli.Command{
		Name:  "podmandev",
		Usage: "devcontainers for podman",
		Commands: []*cli.Command{
			cli.NewDebugCommand(),
		},
	}
	if err := app.Run(context.Background(), os.Args); err != nil {
		log.Fatal(err)
	}
}
```

- [ ] **Step 4: Run tests and build to verify**

```bash
go build -tags containers_image_openpgp ./cmd/devpodman/
go test -tags containers_image_openpgp ./...
```

Expected: PASS, zero errors.

- [ ] **Step 5: Commit**

```bash
git add -A
git commit -m "refactor: restructure to golang-standards/project-layout"
```

---

### Task 2: Create `internal/cli/commands.go` with command factories

**Files:**
- Create: `internal/cli/commands.go`

- [ ] **Step 1: Write the failing test**

```go
package cli

import (
	"testing"
)

func TestNewDebugCommand(t *testing.T) {
	cmd := NewDebugCommand()
	if cmd == nil {
		t.Fatal("NewDebugCommand returned nil")
	}
	if cmd.Name != "debug" {
		t.Fatalf("expected command name 'debug', got %q", cmd.Name)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test -tags containers_image_openpgp ./internal/cli/... -run TestNewDebugCommand -v
```

Expected: FAIL with "undefined: NewDebugCommand"

- [ ] **Step 3: Write minimal implementation**

```go
package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/niule-eu/devpodman/internal/podman"
	"github.com/niule-eu/devpodman/pkg/devcontainer"
	"github.com/urfave/cli/v3"
)

// NewDebugCommand creates the debug subcommand.
func NewDebugCommand() *cli.Command {
	return &cli.Command{
		Name:  "debug",
		Usage: "debug and validate devcontainer configuration",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "validate",
				Aliases: []string{"v"},
			},
			&cli.BoolFlag{
				Name:    "print-podman-config",
				Aliases: []string{"p"},
				Usage:   "load and print podman connection config to stdout",
			},
			&cli.BoolFlag{
				Name:    "list-containers",
				Aliases: []string{"l"},
				Usage:   "list all podman containers",
			},
		},
		Action: debugAction,
	}
}

func debugAction(ctx context.Context, c *cli.Command) error {
	if c.Bool("list-containers") {
		cfg, err := podman.LoadConfig()
		if err != nil {
			return err
		}
		client, err := podman.NewClient(ctx, cfg)
		if err != nil {
			return err
		}
		cts, err := client.ListContainers()
		if err != nil {
			return err
		}
		for _, ct := range cts {
			fmt.Fprintf(os.Stdout, "%s\t%s\t%s\t%s\n", ct.ID, ct.Image, ct.State, ct.Names)
		}
		return nil
	}

	if c.Bool("print-podman-config") {
		cfg, err := podman.LoadConfig()
		if err != nil {
			return err
		}
		fmt.Fprintf(os.Stdout, "socket_path: %s\n", cfg.SocketPath)
		fmt.Fprintf(os.Stdout, "timeout: %s\n", cfg.Timeout)
		fmt.Fprintf(os.Stdout, "connection_uri: %s\n", cfg.ConnectionURI())
		return nil
	}

	validatePath := c.String("validate")
	if validatePath == "" {
		dir, err := os.Getwd()
		if err != nil {
			return err
		}
		fmt.Fprintf(os.Stdout, "Current working directory: %s\n", dir)
		return nil
	}

	fileInfo, err := os.Stat(validatePath)
	if err != nil {
		return fmt.Errorf("failed to stat %s: %w", validatePath, err)
	}
	fmt.Fprintf(os.Stdout, "%s\n", fileInfo)

	data, err := os.ReadFile(validatePath)
	if err != nil {
		return fmt.Errorf("failed to read %s: %w", validatePath, err)
	}

	cfg, err := devcontainer.Load(data)
	if err != nil {
		return err
	}

	if cfg.Build != nil {
		fmt.Fprintf(os.Stdout, "Successfully loaded build config: %+v\n", cfg.Build.Build.Args)
	}
	if cfg.Image != nil {
		fmt.Fprintf(os.Stdout, "Successfully loaded image config: %s\n", cfg.Image.Image)
	}
	if cfg.Common != nil {
		fmt.Fprintf(os.Stdout, "Successfully loaded common config: %+v\n", cfg.Common)
	}

	return nil
}
```

- [ ] **Step 4: Run test to verify it passes**

```bash
go test -tags containers_image_openpgp ./internal/cli/... -run TestNewDebugCommand -v
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/cli/commands.go cmd/devpodman/main.go
git commit -m "feat(cli): move debug command to internal/cli"
```

---

### Task 3: Create `internal/cli/connections.go` with EngineConnection factory

**Files:**
- Create: `internal/cli/connections.go`

- [ ] **Step 1: Write the failing test**

```go
package cli

import (
	"context"
	"testing"

	"github.com/niule-eu/devpodman/internal/podman"
)

func TestNewEngineConnection(t *testing.T) {
	cfg, err := podman.LoadConfig()
	if err != nil {
		t.Skipf("podman not available: %v", err)
	}

	conn, err := NewEngineConnection(context.Background(), cfg)
	if err != nil {
		t.Fatalf("NewEngineConnection failed: %v", err)
	}
	if conn == nil {
		t.Fatal("expected non-nil connection")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test -tags containers_image_openpgp ./internal/cli/... -run TestNewEngineConnection -v
```

Expected: FAIL with "undefined: NewEngineConnection"

- [ ] **Step 3: Write minimal implementation**

```go
package cli

import (
	"context"
	"fmt"

	"github.com/containers/podman/v5/pkg/bindings/containers"
	"github.com/niule-eu/devpodman/internal/podman"
)

// NewEngineConnection creates a validated podman connection context
// for use with pkg/engine. It verifies the connection works by
// attempting a lightweight API call.
func NewEngineConnection(ctx context.Context, cfg *podman.Config) (context.Context, error) {
	client, err := podman.NewClient(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to podman: %w", err)
	}

	connCtx := client.Ctx()

	// Validate connection with a lightweight API call
	trueVal := true
	_, err = containers.List(connCtx, &containers.ListOptions{
		All:   &trueVal,
		Limit: ptrInt(1),
	})
	if err != nil {
		return nil, fmt.Errorf("podman connection validation failed: %w", err)
	}

	return connCtx, nil
}

func ptrInt(i int) *int {
	return &i
}
```

- [ ] **Step 4: Run test to verify it passes**

```bash
go test -tags containers_image_openpgp ./internal/cli/... -run TestNewEngineConnection -v
```

Expected: PASS (or SKIP if podman not available)

- [ ] **Step 5: Commit**

```bash
git add internal/cli/connections.go
git commit -m "feat(cli): add NewEngineConnection with validation"
```

---

### Task 4: Create `pkg/engine/connection.go` with EngineConnection alias and validation

**Files:**
- Create: `pkg/engine/connection.go`
- Create: `pkg/engine/connection_test.go`

- [ ] **Step 1: Write the failing test**

```go
package engine

import (
	"context"
	"testing"
)

func TestEngineConnectionType(t *testing.T) {
	var conn EngineConnection = context.Background()
	if conn == nil {
		t.Fatal("expected non-nil EngineConnection")
	}
}

func TestValidateConnection(t *testing.T) {
	// With a plain context (no podman connection), validation should fail
	ctx := context.Background()
	err := ValidateConnection(ctx)
	if err == nil {
		t.Fatal("expected error for non-podman context")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test -tags containers_image_openpgp ./pkg/engine/... -run TestValidateConnection -v
```

Expected: FAIL with "undefined: EngineConnection" and "undefined: ValidateConnection"

- [ ] **Step 3: Write minimal implementation**

```go
package engine

import (
	"context"
	"fmt"

	"github.com/containers/podman/v5/pkg/bindings/containers"
)

// EngineConnection is an alias for context.Context.
// The podman bindings embed their HTTP client in a context
// returned by bindings.NewConnection(). This context must be
// passed to all podman API calls.
type EngineConnection = context.Context

// ValidateConnection checks that the provided context contains
// a working podman connection by attempting a lightweight API call.
func ValidateConnection(conn EngineConnection) error {
	trueVal := true
	_, err := containers.List(conn, &containers.ListOptions{
		All:   &trueVal,
		Limit: ptrInt(1),
	})
	if err != nil {
		return fmt.Errorf("podman connection validation failed: %w", err)
	}
	return nil
}

func ptrInt(i int) *int {
	return &i
}
```

- [ ] **Step 4: Run test to verify it passes**

```bash
go test -tags containers_image_openpgp ./pkg/engine/... -run TestValidateConnection -v
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add pkg/engine/connection.go pkg/engine/connection_test.go
git commit -m "feat(engine): add EngineConnection alias and ValidateConnection"
```

---

### Task 5: Create `pkg/engine/effects.go` with engine-specific effect types ✅ DONE

**Files:**
- Created: `pkg/engine/effects.go`
- Created: `pkg/engine/effects_test.go`

**Design decisions:**
- Effects implement `pkg/effects.Effect` interface (`Apply() error`)
- Connection passed via constructor: `NewXxxEffect(conn EngineConnection, ...) effects.Effect`
- No custom `Compound` in engine — uses `pkg/effects.Compound` directly
- Integration tests with real podman via `testConn()` helper

**Effect types:**
- `BuildImageEffect` — `NewBuildImageEffect(conn, contextDir, containerfile, tag, buildArgs)`
- `CreatePodEffect` — `NewCreatePodEffect(conn, name, annotations, labels)`
- `CreateContainerEffect` — `NewCreateContainerEffect(conn, spec)`
- `StartContainerEffect` — `NewStartContainerEffect(conn, name)`
- `StartPodEffect` — `NewStartPodEffect(conn, name)`
- `RemovePodEffect` — `NewRemovePodEffect(conn, name)`
- `VolumeImportEffect` — `NewVolumeImportEffect(conn, name, tarData)`
- `RemoveVolumeEffect` — `NewRemoveVolumeEffect(conn, name)`

---

### Task 6: Create `pkg/engine/sidecar.go` with sidecar image building logic

**Files:**
- Create: `pkg/engine/sidecar.go`
- Create: `pkg/engine/sidecar_test.go`
- Create: `pkg/engine/assets/Containerfile`
- Create: `pkg/engine/assets/devpodman-shell`

- [ ] **Step 1: Write the failing test**

```go
package engine

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"strings"
	"testing"
	"text/template"
)

func TestImageTag(t *testing.T) {
	tag := ImageTag(1000)
	expected := "devpodman-code-server:4.98.2-1000"
	if tag != expected {
		t.Errorf("expected %q, got %q", expected, tag)
	}
}

func TestContainerfileTemplate(t *testing.T) {
	rendered, err := RenderContainerfile(1000, 1000)
	if err != nil {
		t.Fatalf("RenderContainerfile failed: %v", err)
	}

	// Verify versions are substituted
	if !strings.Contains(rendered, "ghcr.io/mgoltzsche/podman:5.8.2-remote") {
		t.Error("expected podman remote version substituted")
	}
	if !strings.Contains(rendered, "docker.io/codercom/code-server:4.98.2") {
		t.Error("expected code-server version substituted")
	}

	// Verify no ARG directives remain
	if strings.Contains(rendered, "ARG ") {
		t.Error("expected no ARG directives in rendered template")
	}

	// Verify UID/GID values are inlined
	if !strings.Contains(rendered, "groupadd -g 1000 devpodman") {
		t.Error("expected USER_GID inlined in groupadd")
	}
	if !strings.Contains(rendered, "useradd -m -u 1000 -g 1000") {
		t.Error("expected USER_UID/USER_GID inlined in useradd")
	}
}

func TestContainerfileTemplate_Parse(t *testing.T) {
	// Verify the raw embedded template parses correctly
	tmpl, err := template.New("Containerfile").Parse(containerfileTemplate)
	if err != nil {
		t.Fatalf("failed to parse Containerfile template: %v", err)
	}

	data := ContainerfileTemplateData{
		PodmanRemoteVersion: "5.8.2",
		CodeServerVersion:   "4.98.2",
		UserUID:             1000,
		UserGID:             1000,
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		t.Fatalf("failed to render template: %v", err)
	}
}

func TestRenderConnectionsConfig(t *testing.T) {
	cfg := RenderConnectionsConfig("/run/user/1000/podman/podman.sock")
	if cfg == "" {
		t.Fatal("expected non-empty config")
	}

	// Verify it's valid JSON
	var v map[string]any
	if err := json.Unmarshal([]byte(cfg), &v); err != nil {
		t.Fatalf("config is not valid JSON: %v", err)
	}

	// Verify socket path is present
	if !strings.Contains(cfg, "unix:///run/user/1000/podman/podman.sock") {
		t.Error("expected socket URI in config")
	}
}

func TestDevpodmanShellEmbedded(t *testing.T) {
	if devpodmanShellScript == "" {
		t.Fatal("expected non-empty devpodman-shell script")
	}
	if !strings.HasPrefix(devpodmanShellScript, "#!/bin/bash") {
		t.Error("expected shebang line in devpodman-shell")
	}
	if !strings.Contains(devpodmanShellScript, "MAIN_CONTAINER_NAME") {
		t.Error("expected MAIN_CONTAINER_NAME reference in devpodman-shell")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test -tags containers_image_openpgp ./pkg/engine/... -run TestImageTag -v
```

Expected: FAIL with "undefined: ImageTag"

- [ ] **Step 3: Create the Containerfile template**

Fully Go-templated — no ARG directives. All variables (versions, UID, GID) substituted at render time.

```dockerfile
FROM ghcr.io/mgoltzsche/podman:{{.PodmanRemoteVersion}}-remote AS podman-remote
FROM docker.io/codercom/code-server:{{.CodeServerVersion}}

COPY --from=podman-remote /usr/bin/podman-remote /usr/local/bin/podman

RUN groupadd -g {{.UserGID}} devpodman && \
    useradd -m -u {{.UserUID}} -g {{.UserGID}} -s /bin/bash devpodman

COPY devpodman-shell /usr/local/bin/devpodman-shell
RUN chmod +x /usr/local/bin/devpodman-shell

USER devpodman
WORKDIR /workspace

EXPOSE 8080

CMD ["code-server", "--bind-addr", "0.0.0.0:8080", "/workspace"]
```

- [ ] **Step 4: Create the devpodman-shell wrapper**

```bash
#!/bin/bash
exec podman exec -it "${MAIN_CONTAINER_NAME}" "$@"
```

- [ ] **Step 5: Write minimal implementation**

```go
package engine

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"text/template"
)

const (
	// CodeServerPort is the port code-server listens on inside the sidecar.
	CodeServerPort = 8080

	// CodeServerHostPort is the port mapped to the host.
	CodeServerHostPort = 8090

	// PodmanRemoteVersion is the version of podman-remote to bundle.
	PodmanRemoteVersion = "5.8.2"

	// CodeServerVersion is the version of code-server to use.
	CodeServerVersion = "4.98.2"
)

//go:embed assets/Containerfile
var containerfileTemplate string

//go:embed assets/devpodman-shell
var devpodmanShellScript string

// ContainerfileTemplateData holds the variables for the Containerfile template.
type ContainerfileTemplateData struct {
	PodmanRemoteVersion string
	CodeServerVersion   string
	UserUID             int
	UserGID             int
}

// ImageTag returns the tag for the sidecar image given the host UID.
func ImageTag(uid int) string {
	return fmt.Sprintf("devpodman-code-server:%s-%d", CodeServerVersion, uid)
}

// RenderContainerfile renders the Containerfile template with the given UID/GID.
func RenderContainerfile(uid, gid int) (string, error) {
	tmpl, err := template.New("Containerfile").Parse(containerfileTemplate)
	if err != nil {
		return "", fmt.Errorf("failed to parse Containerfile template: %w", err)
	}

	data := ContainerfileTemplateData{
		PodmanRemoteVersion: PodmanRemoteVersion,
		CodeServerVersion:   CodeServerVersion,
		UserUID:             uid,
		UserGID:             gid,
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to render Containerfile: %w", err)
	}

	return buf.String(), nil
}

// RenderConnectionsConfig generates the podman-connections.json content
// for the sidecar to connect back to the host podman socket.
func RenderConnectionsConfig(socketPath string) string {
	cfg := map[string]any{
		"connection": map[string]string{
			"default": "host",
		},
		"Engines": map[string]any{
			"host": map[string]any{
				"URI":   "unix://" + socketPath,
				"Root":  false,
			},
		},
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return "{}"
	}

	return string(data)
}
```

- [ ] **Step 6: Run test to verify it passes**

```bash
go test -tags containers_image_openpgp ./pkg/engine/... -run "TestImageTag|TestContainerfileTemplate|TestRenderConnectionsConfig|TestDevpodmanShellEmbedded" -v
```

Expected: PASS

- [ ] **Step 7: Commit**

```bash
git add pkg/engine/sidecar.go pkg/engine/sidecar_test.go pkg/engine/assets/
git commit -m "feat(engine): add sidecar image building logic"
```

---

### Task 7: Create `pkg/engine/engine.go` with Play and Down methods

**Files:**
- Create: `pkg/engine/engine.go`
- Create: `pkg/engine/engine_test.go`

**Note:** Use the actual effect constructor pattern: `NewXxxEffect(conn, ...)`. Use `pkg/effects.Compound`, not a custom engine Compound.

- [ ] **Step 1: Write the failing test**

```go
package engine

import (
	"context"
	"testing"

	"github.com/niule-eu/devpodman/pkg/devcontainer"
)

func TestNewEngine(t *testing.T) {
	engine := New()
	if engine == nil {
		t.Fatal("New returned nil")
	}
}

func TestPlay_ReturnsCompound(t *testing.T) {
	engine := New()

	// Minimal image-based devcontainer config
	cfg := &devcontainer.ResolvedConfig{
		Image: &devcontainer.ImageContainer{Image: "alpine:latest"},
		Common: &devcontainer.DevContainerCommon{
			Name: "test-project",
		},
	}

	compound, err := engine.Play(context.Background(), cfg, "/tmp/test")
	if err != nil {
		t.Fatalf("Play returned error: %v", err)
	}
	if len(compound.Effects) == 0 {
		t.Fatal("expected non-empty effects list")
	}
}

func TestDown_ReturnsCompound(t *testing.T) {
	engine := New()

	compound, err := engine.Down(context.Background(), "test-pod", false)
	if err != nil {
		t.Fatalf("Down returned error: %v", err)
	}
	if len(compound.Effects) == 0 {
		t.Fatal("expected non-empty effects list")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test -tags containers_image_openpgp ./pkg/engine/... -run TestNewEngine -v
```

Expected: FAIL with "undefined: New"

- [ ] **Step 3: Write minimal implementation**

```go
package engine

import (
	"archive/tar"
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/containers/podman/v5/pkg/specgen"
	"github.com/niule-eu/devpodman/pkg/devcontainer"
	"github.com/niule-eu/devpodman/pkg/effects"
)

// Engine orchestrates devcontainer pod lifecycle.
type Engine struct{}

// New creates a new Engine.
func New() *Engine {
	return &Engine{}
}

// Play returns a Compound effect that creates a devcontainer pod
// with a main container and code-server sidecar.
func (e *Engine) Play(conn EngineConnection, cfg *devcontainer.ResolvedConfig, workspaceDir string) (effects.Compound, error) {
	podName := DerivePodName(cfg, workspaceDir)
	mainContainerName := podName + "-main"
	sidecarName := podName + "-code-server"

	uid := os.Getuid()
	gid := os.Getgid()
	sidecarTag := ImageTag(uid)

	// Resolve main image
	mainImage, err := resolveMainImage(cfg, podName)
	if err != nil {
		return effects.Compound{}, err
	}

	// Render Containerfile
	renderedContainerfile, err := RenderContainerfile(uid, gid)
	if err != nil {
		return effects.Compound{}, fmt.Errorf("failed to render Containerfile: %w", err)
	}

	// Generate connections config
	socketPath := detectSocketPath()
	connectionsJSON := RenderConnectionsConfig(socketPath)

	var effs []effects.Effect

	// Effect 1: Write Containerfile and shell script to temp dir
	tmpDir := filepath.Join(os.TempDir(), "devpodman-"+podName)
	containerfilePath := filepath.Join(tmpDir, "Containerfile")
	shellPath := filepath.Join(tmpDir, "devpodman-shell")

	effs = append(effs, effects.FileWrite{
		Path:        containerfilePath,
		Content:     []byte(renderedContainerfile),
		Permissions: 0644,
	})
	effs = append(effs, effects.FileWrite{
		Path:        shellPath,
		Content:     []byte(devpodmanShellScript),
		Permissions: 0755,
	})
	effs = append(effs, NewBuildImageEffect(conn, tmpDir, "Containerfile", sidecarTag, nil))

	// Effect 2: Create volumes for sidecar assets
	connectionsVolumeName := podName + "-connections"
	effs = append(effs, NewVolumeImportEffect(conn, connectionsVolumeName, createTarForFile("podman-connections.json", []byte(connectionsJSON))))

	// Effect 3: Create pod
	effs = append(effs, NewCreatePodEffect(conn, podName, map[string]string{
		"io.podman.annotations.userns": "keep-id",
	}, map[string]string{
		"devpodman/managed": "true",
	}))

	// Main container spec
	mainSpec := buildMainContainerSpec(cfg, mainImage, mainContainerName, podName, workspaceDir)
	effs = append(effs, NewCreateContainerEffect(conn, mainSpec))

	// Sidecar container spec
	sidecarSpec := buildSidecarContainerSpec(sidecarTag, sidecarName, podName, mainContainerName, connectionsVolumeName, workspaceDir)
	effs = append(effs, NewCreateContainerEffect(conn, sidecarSpec))

	// Effect 4: Start pod (starts all containers in the pod)
	effs = append(effs, NewStartPodEffect(conn, podName))

	return effects.Compound{Effects: effs, FailFast: true}, nil
}

// Down returns a Compound effect that stops and removes a devcontainer pod.
func (e *Engine) Down(conn EngineConnection, podName string, deleteImages bool) (effects.Compound, error) {
	var effs []effects.Effect

	// Effect 1: Remove pod (force, includes containers)
	effs = append(effs, NewRemovePodEffect(conn, podName))

	// Effect 2: Remove associated volumes
	connectionsVolumeName := podName + "-connections"
	effs = append(effs, NewRemoveVolumeEffect(conn, connectionsVolumeName))

	// Image cleanup: omitted for now; flag accepted but no-op

	return effects.Compound{Effects: effs, FailFast: true}, nil
}

// DerivePodName returns the deterministic pod name for a config + workspace dir.
// Exported so the CLI can compute the pod name independently.
func DerivePodName(cfg *devcontainer.ResolvedConfig, workspaceDir string) string {
	if cfg.Common != nil && cfg.Common.Name != "" {
		return "devpodman-" + sanitizeName(cfg.Common.Name)
	}
	return "devpodman-" + sanitizeName(filepath.Base(workspaceDir))
}

func sanitizeName(name string) string {
	result := make([]rune, 0, len(name))
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			result = append(result, r)
		} else {
			result = append(result, '-')
		}
	}
	trimmed := string(result)
	trimmed = trim(trimmed, '-')
	return trimmed
}

func trim(s string, cut rune) string {
	start := 0
	for start < len(s) && rune(s[start]) == cut {
		start++
	}
	end := len(s)
	for end > start && rune(s[end-1]) == cut {
		end--
	}
	return s[start:end]
}

func resolveMainImage(cfg *devcontainer.ResolvedConfig, podName string) (string, error) {
	if cfg.Image != nil {
		return cfg.Image.Image, nil
	}
	if cfg.Build != nil {
		return podName + "-main", nil
	}
	return "", fmt.Errorf("devcontainer.json must specify 'image' or 'build'")
}

func detectSocketPath() string {
	if path := os.Getenv("XDG_RUNTIME_DIR"); path != "" {
		return filepath.Join(path, "podman", "podman.sock")
	}
	return "/run/podman/podman.sock"
}

func buildMainContainerSpec(cfg *devcontainer.ResolvedConfig, image, name, podName, dir string) *specgen.SpecGenerator {
	spec := specgen.NewSpecGenerator(image, false)
	spec.Name = name
	spec.Pod = podName
	spec.Command = []string{"sleep", "infinity"}
	spec.WorkDir = "/workspace"

	// Workspace mount
	spec.Mounts = append(spec.Mounts, specgen.Mount{
		Source:      dir,
		Destination: "/workspace",
		Type:        "bind",
	})

	// Additional mounts from devcontainer.json
	if cfg.NonCompose != nil {
		for _, m := range cfg.NonCompose.Mounts {
			spec.Mounts = append(spec.Mounts, specgen.Mount{
				Source:      m.Source,
				Destination: m.Target,
				Type:        m.Type,
			})
		}
	}

	// Environment
	env := make(map[string]string)
	if cfg.NonCompose != nil {
		for k, v := range cfg.NonCompose.ContainerEnv {
			env[k] = v
		}
	}
	if cfg.Common != nil {
		for k, v := range cfg.Common.RemoteEnv {
			env[k] = v
		}
	}
	spec.Env = env

	// User
	if cfg.Common != nil && cfg.Common.RemoteUser != "" {
		spec.User = cfg.Common.RemoteUser
	} else if cfg.NonCompose != nil && cfg.NonCompose.ContainerUser != "" {
		spec.User = cfg.NonCompose.ContainerUser
	}

	// Working directory override
	if cfg.NonCompose != nil && cfg.NonCompose.WorkspaceFolder != "" {
		spec.WorkDir = cfg.NonCompose.WorkspaceFolder
	}

	return spec
}

func buildSidecarContainerSpec(image, name, podName, mainContainerName, connectionsVolumeName, workspaceDir string) *specgen.SpecGenerator {
	spec := specgen.NewSpecGenerator(image, false)
	spec.Name = name
	spec.Pod = podName
	spec.Command = []string{"code-server", "--bind-addr", "0.0.0.0:8080", "/workspace"}
	spec.Env = map[string]string{
		"MAIN_CONTAINER_NAME": mainContainerName,
	}
	spec.WorkDir = "/workspace"

	spec.Mounts = []specgen.Mount{
		{
			Source:      workspaceDir,
			Destination: "/workspace",
			Type:        "bind",
		},
		{
			Source:      connectionsVolumeName,
			Destination: "/home/devpodman/.config/containers",
			Type:        "volume",
		},
	}

	spec.PortMappings = []specgen.PortMapping{
		{
			ContainerPort: 8080,
			HostPort:      8090,
			Protocol:      "tcp",
		},
	}

	return spec
}

func createTarForFile(name string, data []byte) []byte {
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	hdr := &tar.Header{
		Name: name,
		Mode: 0644,
		Size: int64(len(data)),
	}
	tw.WriteHeader(hdr)
	tw.Write(data)
	tw.Close()
	return buf.Bytes()
}
```

- [ ] **Step 4: Run test to verify it passes**

```bash
go test -tags containers_image_openpgp ./pkg/engine/... -run "TestNewEngine|TestPlay_ReturnsCompound|TestDown_ReturnsCompound" -v
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add pkg/engine/engine.go pkg/engine/engine_test.go
git commit -m "feat(engine): add Play and Down methods returning effect compounds"
```

---

### Task 8: Wire `play` and `down` commands in `internal/cli/commands.go`

**Files:**
- Modify: `internal/cli/commands.go`

**Note:** `Engine.Play` takes `EngineConnection` (not `context.Context`). Use `pkg/effects.Compound`. Effects use constructor pattern.

- [ ] **Step 1: Add Play and Down command factories**

Add to `internal/cli/commands.go`:

```go
import (
	"github.com/niule-eu/devpodman/pkg/engine"
)

// NewPlayCommand creates the play subcommand.
func NewPlayCommand() *cli.Command {
	return &cli.Command{
		Name:      "play",
		Usage:     "create and start a devcontainer pod with code-server sidecar",
		ArgsUsage: "<path-to-devcontainer.json>",
		Action:    playAction,
	}
}

func playAction(ctx context.Context, c *cli.Command) error {
	path := c.Args().First()
	if path == "" {
		return fmt.Errorf("path to devcontainer.json is required")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read %s: %w", path, err)
	}

	cfg, err := devcontainer.Load(data)
	if err != nil {
		return fmt.Errorf("failed to load devcontainer config: %w", err)
	}

	workspaceDir := filepath.Dir(path)

	podCfg, err := podman.LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load podman config: %w", err)
	}

	conn, err := NewEngineConnection(ctx, podCfg)
	if err != nil {
		return err
	}

	eng := engine.New()
	compound, err := eng.Play(conn, cfg, workspaceDir)
	if err != nil {
		return err
	}

	if err := compound.Apply(); err != nil {
		return fmt.Errorf("failed to apply play effects: %w", err)
	}

	fmt.Fprintf(os.Stdout, "Pod %q started\n", engine.DerivePodName(cfg, workspaceDir))
	fmt.Fprintf(os.Stdout, "code-server: http://localhost:%d\n", engine.CodeServerHostPort)

	return nil
}

// NewDownCommand creates the down subcommand.
func NewDownCommand() *cli.Command {
	return &cli.Command{
		Name:      "down",
		Usage:     "stop and remove a devcontainer pod",
		ArgsUsage: "<pod-name>",
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:  "delete-images",
				Usage: "remove images built by devpodman",
			},
		},
		Action: downAction,
	}
}

func downAction(ctx context.Context, c *cli.Command) error {
	podName := c.Args().First()
	if podName == "" {
		return fmt.Errorf("pod name is required")
	}

	podCfg, err := podman.LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load podman config: %w", err)
	}

	conn, err := NewEngineConnection(ctx, podCfg)
	if err != nil {
		return err
	}

	eng := engine.New()
	compound, err := eng.Down(conn, podName, c.Bool("delete-images"))
	if err != nil {
		return err
	}

	if err := compound.Apply(); err != nil {
		return fmt.Errorf("failed to apply down effects: %w", err)
	}

	fmt.Fprintf(os.Stdout, "Pod %q removed\n", podName)
	return nil
}
```

- [ ] **Step 2: Update `cmd/devpodman/main.go` to register new commands**

```go
package main

import (
	"context"
	"log"
	"os"

	"github.com/niule-eu/devpodman/internal/cli"
	"github.com/urfave/cli/v3"
)

func main() {
	app := &cli.Command{
		Name:  "podmandev",
		Usage: "devcontainers for podman",
		Commands: []*cli.Command{
			cli.NewDebugCommand(),
			cli.NewPlayCommand(),
			cli.NewDownCommand(),
		},
	}
	if err := app.Run(context.Background(), os.Args); err != nil {
		log.Fatal(err)
	}
}
```

- [ ] **Step 3: Verify compilation**

```bash
go build -tags containers_image_openpgp ./cmd/devpodman/
```

Expected: PASS, zero errors.

- [ ] **Step 4: Run all tests**

```bash
go test -tags containers_image_openpgp ./...
```

Expected: All tests pass (integration tests may skip if podman not available)

- [ ] **Step 5: Commit**

```bash
git add internal/cli/commands.go cmd/devpodman/main.go
git commit -m "feat(cli): add play and down commands wired to engine"
```

---

### Task 9: Clean up stale files, fix test, final validation

**Files:**
- Delete: `cmd/devpodman/debug.go` (stale duplicate of `internal/cli/commands.go`)
- Fix: `pkg/engine/effects_test.go` (stale volume name collision)

- [ ] **Step 1: Delete stale `cmd/devpodman/debug.go`**

This file duplicates the debug command that now lives in `internal/cli/commands.go`.
`main.go` imports `cli.NewDebugCommand()` from `internal/cli`, making this dead code.

```bash
rm cmd/devpodman/debug.go
```

- [ ] **Step 2: Fix stale volume test in `pkg/engine/effects_test.go`**

The `TestRemoveVolumeEffect_Apply/removes_existing_volume` subtest uses a fixed volume name
`test-remove-vol` that can collide across test runs. Use a unique suffix:

```go
func TestRemoveVolumeEffect_Apply(t *testing.T) {
	conn := testConn(t)

	t.Run("removes existing volume", func(t *testing.T) {
		volName := "test-remove-vol-" + t.Name()
		tarData := makeTarFile("test.txt", []byte("hello"))
		importEff := NewVolumeImportEffect(conn, volName, tarData)
		if err := importEff.Apply(); err != nil {
			t.Fatalf("failed to create test volume: %v", err)
		}
		t.Cleanup(func() { _ = NewRemoveVolumeEffect(conn, volName).Apply() })

		eff := NewRemoveVolumeEffect(conn, volName)
		err := eff.Apply()
		if err != nil {
			t.Fatalf("Apply() failed: %v", err)
		}
	})

	t.Run("returns nil for nonexistent volume", func(t *testing.T) {
		eff := NewRemoveVolumeEffect(conn, "test-nonexistent-vol-xyz")
		err := eff.Apply()
		if err != nil {
			t.Fatalf("expected nil for nonexistent volume, got: %v", err)
		}
	})
}
```

- [ ] **Step 3: Verify no stale imports**

```bash
rg "github.com/niule-eu/devpodman/(devcontainer|effects|model|podman)[^/]" --type go
```

Expected: No matches (all imports should use `pkg/` or `internal/` paths)

- [ ] **Step 4: Run full build and test suite**

```bash
go build -tags containers_image_openpgp ./cmd/devpodman/
go test -tags containers_image_openpgp ./...
```

Expected: All tests pass (integration tests may skip if podman not available)

- [ ] **Step 5: Commit**

```bash
git add -A
git commit -m "chore: remove stale debug.go, fix volume test collision"
```

---

## Post-Implementation Validation

Manual smoke test on a machine with podman:

```bash
go-task build
./devpodman play ./testdata/devcontainer.json   # or an image-based config
./devpodman debug -l                            # verify containers visible
./devpodman down devpodman-<name>
```

---

## Self-Review Checklist

1. **Spec coverage:** All elements from the revised design are covered: engine API returning effects, EngineConnection alias with validation (in podman package), separate effect types for each operation using constructor pattern, sidecar image building with fully Go-templated Containerfile (no ARG directives), volume import for connections config, play/down lifecycle, golang-standards/project-layout structure.

2. **Placeholder scan:** No TBD, TODO, or fill-in-the-details markers. The `createTarForFile` function is fully implemented using `archive/tar`. Image cleanup in `Down` is explicitly documented as omitted.

3. **Type consistency:** `EngineConnection` alias used consistently. Effect types use constructor pattern (`NewXxxEffect(conn, ...)`) returning `effects.Effect` with `Apply() error`. `pkg/effects.Compound` used directly. Sidecar constants (`CodeServerPort`, `CodeServerHostPort`) defined in `sidecar.go`. `DerivePodName` is exported from `pkg/engine` — CLI uses it directly, no duplicate sanitization logic. `SpecGenerator.Pod` uses plain pod name (no `"pod:"` prefix).

4. **Deviation from original plan:**
   - Connection validation moved to `internal/podman.NewClient()` (Task 3)
   - `pkg/engine/connection.go` has only the type alias, no `ValidateConnection` (Task 4)
   - Effects use constructor pattern, not struct literals with `Apply(conn)` (Task 5)
   - No custom `Compound` in engine — uses `pkg/effects.Compound` directly (Task 5)
   - `Engine.Play` returns `(effects.Compound, error)` — pod name computed via exported `DerivePodName`
   - `ConnectionsConfig` replaced with `RenderConnectionsConfig(socketPath string)` — no `podName` param
   - Containerfile fully Go-templated — no ARG directives remain
   - Image cleanup in `Down` omitted for now
   - `cmd/devpodman/debug.go` deleted as stale dead code
   - Volume test fixed with unique names to prevent stale collision
