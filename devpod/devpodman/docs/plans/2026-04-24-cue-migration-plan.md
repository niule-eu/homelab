# Migrate devpodman from Pkl to CUE config backend — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace PKL schema-based type generation with CUE. Use CUE runtime for individual definition validation, Go for priority-based conflict resolution.

**Architecture:** CUE schema lives at `model/devcontainer.cue` with `package model`. `cue exp gengotypes` generates Go structs into `model/cue_types_model_gen.go`. `devcontainer.go` uses the embedded CUE schema at runtime via `cuelang.org/go` to decode each definition independently, then applies Go priority logic (dockerfile > image) to resolve conflicts into a `ResolvedConfig` struct.

**Tech Stack:** Go 1.25, `cuelang.org/go`, `embed`, `distribution/reference`

> **Note:** Schema location is `model/devcontainer.cue` (not `cue/` as in the design doc) because `cue exp gengotypes` generates files into the CUE package directory. Keeping schema + generated code together avoids file-copy indirection.

---

### Task 1: Write the CUE schema

**Files:**
- Create: `devpodman/model/devcontainer.cue`

- [ ] **Step 1: Write the CUE schema file**

Write `devpodman/model/devcontainer.cue`:

```cue
package model

// Mount definition for devpodman
#Mount: {
	source!: string
	target!: string
	type!:   "volume" | "bind"
}

// Build configuration options
#buildOptions: {
	// Target stage in a multi-stage build.
	target?: string

	// Build arguments.
	args?: [string]: string

	// The image to consider as a cache.
	cacheFrom?: [...string]

	// The location of the Dockerfile. Must be a relative path.
	dockerfile!: string & =~"^[^/]"

	// The location of the context folder. Must be a relative path if set.
	context?: string & =~"^[^/]"
}

// Properties common to all dev container configurations
#devContainerCommon: {
	// A name for the dev container.
	name?: string

	// Features to add to the dev container.
	features?: {
		...
	}

	// Array of Feature ids in install order.
	overrideFeatureInstallOrder?: [...string]

	// Forwarded ports.
	forwardPorts?: [..._]

	// Port-specific attributes.
	portsAttributes?: {
		...
	}

	// Default port attributes.
	otherPortsAttributes?: close({
		onAutoForward?:    "notify" | "openBrowser" | "openPreview" | "silent" | "ignore"
		elevateIfNeeded?:  bool
		label?:            string
		requireLocalPort?: bool
		protocol?:         "http" | "https"
	})

	// Update remote user UID/GID.
	updateRemoteUserUID?: bool

	// Remote environment variables.
	remoteEnv?: [string]: string

	// Username for processes in container.
	remoteUser?: string

	// Host initialization command.
	initializeCommand?: [...string]

	// Container creation lifecycle commands.
	onCreateCommand?:      { [string]: [...string] }
	updateContentCommand?: { [string]: [...string] }
	postCreateCommand?:    { [string]: [...string] }
	postStartCommand?:     { [string]: [...string] }
	postAttachCommand?:    { [string]: [...string] }

	// Command to wait for before background execution.
	waitFor?: "initializeCommand" | "onCreateCommand" | "updateContentCommand" | "postCreateCommand" | "postStartCommand"

	// User environment probe.
	userEnvProbe?: "none" | "loginShell" | "loginInteractiveShell" | "interactiveShell"

	// Host hardware requirements.
	hostRequirements?: {
		cpus?:    int & >=1
		memory?:  =~"^\\d+([tgmk]b)?$"
		storage?: =~"^\\d+([tgmk]b)?$"
		gpu?:     _
	}

	// Tool-specific configuration.
	customizations?: {
		...
	}

	additionalProperties?: {
		...
	}

	// devpodman extensions
	privileged?:    bool
	containerEnv?:  [string]: string
	containerUser?: string
	runArgs?:       [...string]
	mounts?:        [...#Mount]

	// Workspace configuration with path constraints
	workspaceFolder?: string & =~"^/"
	workspaceMount?:  #Mount
}

// Container defined by a Dockerfile build
#dockerfileContainer: {
	build!: #buildOptions
}

// Container defined by a pre-built image
#imageContainer: {
	image!: string & =~"^.+$"
}

// Non-Compose base properties
#nonComposeBase: {
	appPort?:         [...string]
	shutdownAction?:  "none" | "stopContainer"
	overrideCommand?: bool
	workspaceFolder?: string
	workspaceMount?:  #Mount
}
```

- [ ] **Step 2: Verify the schema compiles**

```bash
cue eval devpodman/model/devcontainer.cue
```

Expected: no output (success). If errors, fix and re-run.

- [ ] **Step 3: Commit**

```bash
git add devpodman/model/devcontainer.cue
git commit -m "feat: add CUE schema for devcontainer validation"
```

---

### Task 2: Generate Go types from CUE

**Files:**
- Create: `devpodman/model/cue_types_model_gen.go` (generated)

- [ ] **Step 1: Generate Go types**

```bash
cue exp gengotypes devpodman/model/devcontainer.cue
```

Expected: creates `devpodman/model/cue_types_model_gen.go` alongside the CUE file.

- [ ] **Step 2: Verify the generated file compiles**

```bash
go build -tags containers_image_openpgp ./devpodman/model/...
```

Expected: compilation succeeds. If not, inspect generated file for issues.

- [ ] **Step 3: Commit**

```bash
git add devpodman/model/cue_types_model_gen.go
git commit -m "feat: add CUE-generated Go types for devcontainer model"
```

---

### Task 3: Remove PKL artifacts

**Files:**
- Remove: `devpodman/pkl/` (entire directory)
- Remove: `devpodman/PklProject`
- Remove: `devpodman/PklProject.deps.json`
- Remove: `devpodman/model/DevpodmanSchema.pkl.go`
- Remove: `devpodman/model/init.pkl.go`
- Remove: `devpodman/model/build/` (entire directory)
- Remove: `devpodman/model/common/` (entire directory)
- Remove: `devpodman/model/image/` (entire directory)

- [ ] **Step 1: Delete all PKL files**

```bash
rm -rf devpodman/pkl
rm -f devpodman/PklProject
rm -f devpodman/PklProject.deps.json
rm -f devpodman/model/DevpodmanSchema.pkl.go
rm -f devpodman/model/init.pkl.go
rm -rf devpodman/model/build
rm -rf devpodman/model/common
rm -rf devpodman/model/image
```

- [ ] **Step 2: Verify remaining model files**

```bash
ls devpodman/model/
```

Expected: only `devcontainer.cue` and `cue_types_model_gen.go`.

- [ ] **Step 3: Commit**

```bash
git add -A devpodman/model devpodman/pkl devpodman/PklProject devpodman/PklProject.deps.json
git commit -m "refactor: remove PKL schema artifacts"
```

---

### Task 4: Update Go dependencies

**Files:**
- Modify: `devpodman/go.mod`
- Modify: `devpodman/go.sum`

- [ ] **Step 1: Add cuelang.org/go**

```bash
go get cuelang.org/go@latest
```

Workdir: `devpodman`.

- [ ] **Step 2: Remove pkl-go**

```bash
go get github.com/apple/pkl-go@none
```

Workdir: `devpodman`.

- [ ] **Step 3: Tidy**

```bash
go mod tidy
```

Workdir: `devpodman`.

- [ ] **Step 4: Verify go.mod clean**

Expected: `github.com/apple/pkl-go` absent from `go.mod`. `cuelang.org/go` present.

- [ ] **Step 5: Commit**

```bash
git add devpodman/go.mod devpodman/go.sum
git commit -m "build: replace pkl-go with cuelang.org/go"
```

---

### Task 5: Write ResolvedConfig type and embed wiring

**Files:**
- Create: `devpodman/model/schema.go`
- Create: `devpodman/model/schema_test.go`
- Modify: `devpodman/devcontainer/devcontainer.go`

- [ ] **Step 1: Add embed to model package**

Create `devpodman/model/schema.go`:

```go
package model

import _ "embed"

//go:embed devcontainer.cue
var Schema string
```

- [ ] **Step 2: Test embed works**

Create `devpodman/model/schema_test.go`:

```go
package model

import "testing"

func TestSchemaNotEmpty(t *testing.T) {
	if Schema == "" {
		t.Fatal("embedded schema is empty")
	}
}
```

Run: `go test -tags containers_image_openpgp ./devpodman/model/... -run TestSchemaNotEmpty -v`

Expected: PASS.

- [ ] **Step 3: Write the ResolvedConfig struct and verify imports compile**

Replace `devpodman/devcontainer/devcontainer.go` with the scaffolding. Replace the exact type names with what `cue_types_model_gen.go` actually exports (verify by reading the file):

```go
package devcontainer

import (
	"github.com/niule-eu/devpodman/model"
)

// ResolvedConfig holds the parsed devcontainer configuration after
// CUE validation and Go priority resolution.
type ResolvedConfig struct {
	Build      *model.DockerfileContainer
	Image      *model.ImageContainer
	Common     *model.DevContainerCommon
	NonCompose *model.NonComposeBase
}
```

Run: `go build -tags containers_image_openpgp ./devpodman/devcontainer/...`

Expected: compiles.

- [ ] **Step 4: Commit**

```bash
git add devpodman/model/schema.go devpodman/model/schema_test.go devpodman/devcontainer/devcontainer.go
git commit -m "feat: add ResolvedConfig type and embedded CUE schema"
```

---

### Task 6: Implement Load() with CUE multi-def decode and priority

**Files:**
- Modify: `devpodman/devcontainer/devcontainer.go`

- [ ] **Step 1: Write the failing test for image-based container**

Add to `devpodman/devcontainer/devcontainer_test.go`:

```go
func TestLoadImageContainer(t *testing.T) {
	t.Run("loads image-based devcontainer", func(t *testing.T) {
		data := []byte(`{
			"name": "Go Dev",
			"image": "mcr.microsoft.com/devcontainers/go:1",
			"remoteUser": "vscode"
		}`)

		cfg, err := Load(data)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.Build != nil {
			t.Fatal("expected Build to be nil for image-based devcontainer")
		}
		if cfg.Image == nil {
			t.Fatal("expected Image to be non-nil")
		}
		if cfg.Image.Image != "mcr.microsoft.com/devcontainers/go:1" {
			t.Fatalf("expected image 'mcr.microsoft.com/devcontainers/go:1', got %q", cfg.Image.Image)
		}
		if cfg.Common == nil {
			t.Fatal("expected Common to be non-nil")
		}
		if cfg.Common.Name == nil || *cfg.Common.Name != "Go Dev" {
			t.Fatalf("expected name 'Go Dev', got %v", cfg.Common.Name)
		}
	})
}
```

- [ ] **Step 2: Run test — expect failure**

```bash
go test -tags containers_image_openpgp ./devpodman/devcontainer/... -run TestLoadImageContainer/loads_image-based_devcontainer -v
```

Expected: FAIL — `Load` not implemented yet.

- [ ] **Step 3: Implement Load()**

Write the complete `devpodman/devcontainer/devcontainer.go`:

```go
package devcontainer

import (
	"fmt"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
	"github.com/distribution/reference"

	"github.com/niule-eu/devpodman/model"
)

// ResolvedConfig holds the parsed devcontainer configuration after
// CUE validation and Go priority resolution.
type ResolvedConfig struct {
	Build      *model.DockerfileContainer
	Image      *model.ImageContainer
	Common     *model.DevContainerCommon
	NonCompose *model.NonComposeBase
}

// Load parses and validates a devcontainer.json byte slice.
// It validates the JSON against individual CUE definitions and resolves
// conflicts via Go priority (dockerfile over image).
func Load(data []byte) (*ResolvedConfig, error) {
	ctx := cuecontext.New()
	schema := ctx.CompileString(model.Schema)
	if err := schema.Err(); err != nil {
		return nil, fmt.Errorf("failed to compile CUE schema: %w", err)
	}

	// Parse input JSON as CUE value
	input := ctx.CompileBytes(data)
	if err := input.Err(); err != nil {
		return nil, fmt.Errorf("failed to parse devcontainer.json: %w", err)
	}

	var (
		dockerfileOK bool
		imageOK      bool
		dockerfileDC model.DockerfileContainer
		imageDC      model.ImageContainer
		commonDC     model.DevContainerCommon
		nonComposeDC model.NonComposeBase
		errs         []string
	)

	// Try dockerfileContainer
	dfDef := schema.LookupPath(cue.ParsePath("#dockerfileContainer"))
	if dfDef.Exists() {
		val := dfDef.Unify(input)
		if err := val.Err(); err != nil {
			errs = append(errs, fmt.Sprintf("dockerfile: %v", err))
		} else {
			if dErr := val.Decode(&dockerfileDC); dErr != nil {
				errs = append(errs, fmt.Sprintf("dockerfile decode: %v", dErr))
			} else {
				dockerfileOK = true
			}
		}
	}

	// Try imageContainer
	imgDef := schema.LookupPath(cue.ParsePath("#imageContainer"))
	if imgDef.Exists() {
		val := imgDef.Unify(input)
		if err := val.Err(); err != nil {
			errs = append(errs, fmt.Sprintf("image: %v", err))
		} else {
			if dErr := val.Decode(&imageDC); dErr != nil {
				errs = append(errs, fmt.Sprintf("image decode: %v", dErr))
			} else {
				imageOK = true
			}
		}
	}

	// Neither matched
	if !dockerfileOK && !imageOK {
		detail := ""
		if len(errs) > 0 {
			detail = ": " + joinErrs(errs)
		}
		return nil, fmt.Errorf("devcontainer.json must specify either 'image' or 'build'%s", detail)
	}

	// Try devContainerCommon (best-effort)
	commonDef := schema.LookupPath(cue.ParsePath("#devContainerCommon"))
	if commonDef.Exists() {
		val := commonDef.Unify(input)
		if err := val.Err(); err == nil {
			_ = val.Decode(&commonDC)
		}
	}

	// Try nonComposeBase (best-effort)
	ncDef := schema.LookupPath(cue.ParsePath("#nonComposeBase"))
	if ncDef.Exists() {
		val := ncDef.Unify(input)
		if err := val.Err(); err == nil {
			_ = val.Decode(&nonComposeDC)
		}
	}

	cfg := &ResolvedConfig{
		Common:     &commonDC,
		NonCompose: &nonComposeDC,
	}

	// Priority: dockerfile over image
	if dockerfileOK {
		cfg.Build = &dockerfileDC
		return cfg, nil
	}

	// imageOK path
	if err := validateImageReference(imageDC.Image); err != nil {
		return nil, err
	}
	cfg.Image = &imageDC
	return cfg, nil
}

func validateImageReference(ref string) error {
	if ref == "" {
		return fmt.Errorf("'image' must not be empty")
	}
	if _, err := reference.ParseNormalizedNamed(ref); err != nil {
		return fmt.Errorf("'image' must be a valid container image reference, got %q", ref)
	}
	return nil
}

func joinErrs(errs []string) string {
	s := ""
	for i, e := range errs {
		if i > 0 {
			s += "; "
		}
		s += e
	}
	return s
}
```

- [ ] **Step 4: Run test — expect pass**

```bash
go test -tags containers_image_openpgp ./devpodman/devcontainer/... -run TestLoadImageContainer -v
```

Expected: PASS.

- [ ] **Step 5: Write test for build-based container**

Add to `devpodman/devcontainer/devcontainer_test.go`:

```go
func TestLoadBuildContainer(t *testing.T) {
	t.Run("loads build-based devcontainer", func(t *testing.T) {
		data := []byte(`{
			"name": "Custom Build",
			"build": {
				"dockerfile": "Dockerfile",
				"context": ".",
				"target": "dev"
			}
		}`)

		cfg, err := Load(data)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.Image != nil {
			t.Fatal("expected Image to be nil for build-based devcontainer")
		}
		if cfg.Build == nil {
			t.Fatal("expected Build to be non-nil")
		}
		if cfg.Build.Build.Dockerfile != "Dockerfile" {
			t.Fatalf("expected dockerfile 'Dockerfile', got %q", cfg.Build.Build.Dockerfile)
		}
		if cfg.Common.Name == nil || *cfg.Common.Name != "Custom Build" {
			t.Fatalf("expected name 'Custom Build', got %v", cfg.Common.Name)
		}
	})
}
```

- [ ] **Step 6: Run test — expect pass**

```bash
go test -tags containers_image_openpgp ./devpodman/devcontainer/... -run TestLoadBuildContainer -v
```

Expected: PASS.

- [ ] **Step 7: Write priority test**

```go
func TestLoadPriority(t *testing.T) {
	t.Run("both image and build provided — dockerfile wins", func(t *testing.T) {
		data := []byte(`{
			"image": "golang:1.22",
			"build": {
				"dockerfile": "Dockerfile"
			}
		}`)

		cfg, err := Load(data)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.Build == nil {
			t.Fatal("expected Build to win over Image (dockerfile priority)")
		}
		if cfg.Image != nil {
			t.Fatal("expected Image to be nil when dockerfile wins")
		}
	})

	t.Run("neither image nor build — error", func(t *testing.T) {
		data := []byte(`{"name": "no container"}`)
		_, err := Load(data)
		if err == nil {
			t.Fatal("expected error when neither image nor build provided")
		}
	})
}
```

- [ ] **Step 8: Run priority tests — expect pass**

```bash
go test -tags containers_image_openpgp ./devpodman/devcontainer/... -run TestLoadPriority -v
```

Expected: PASS.

- [ ] **Step 9: Write image validation tests**

```go
func TestValidateImageReference(t *testing.T) {
	tests := []struct {
		name    string
		image   string
		wantErr bool
	}{
		{"simple repo with tag", "golang:1.22", false},
		{"simple repo without tag", "alpine", false},
		{"registry with repo and tag", "mcr.microsoft.com/devcontainers/go:1.21", false},
		{"image with digest", "alpine@sha256:abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789", false},
		{"empty image", "", true},
		{"leading whitespace", " golang:1.22", true},
		{"trailing whitespace", "golang:1.22 ", true},
		{"invalid digest format", "alpine@sha256:xyz", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := []byte(fmt.Sprintf(`{"image": %q}`, tt.image))
			_, err := Load(data)
			if tt.wantErr && err == nil {
				t.Fatal("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}
```

- [ ] **Step 10: Run image validation tests**

```bash
go test -tags containers_image_openpgp ./devpodman/devcontainer/... -run TestValidateImageReference -v
```

Expected: all subtests PASS/FAIL correctly.

- [ ] **Step 11: Commit**

```bash
git add devpodman/devcontainer/devcontainer.go devpodman/devcontainer/devcontainer_test.go
git commit -m "feat: implement Load() with CUE multi-def decode and priority resolution"
```

---

### Task 7: Migrate remaining existing tests

**Files:**
- Modify: `devpodman/devcontainer/devcontainer_test.go`

- [ ] **Step 1: Rewrite existing test cases for ResolvedConfig**

Add the remaining test functions adapted from the old PKL-based tests to `devpodman/devcontainer/devcontainer_test.go`:

```go
func TestValidateWorkspaceMountRequiresWorkspaceFolder(t *testing.T) {
	t.Run("returns error when workspaceMount set without workspaceFolder", func(t *testing.T) {
		data := []byte(`{
			"image": "alpine:latest",
			"workspaceMount": {"source": "src", "target": "/workspace", "type": "volume"}
		}`)
		_, err := Load(data)
		if err == nil {
			t.Fatal("expected error when workspaceMount is set without workspaceFolder")
		}
	})

	t.Run("succeeds when workspaceMount and workspaceFolder both set", func(t *testing.T) {
		data := []byte(`{
			"image": "alpine:latest",
			"workspaceMount": {"source": "src", "target": "/workspace", "type": "volume"},
			"workspaceFolder": "/workspace"
		}`)
		cfg, err := Load(data)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.NonCompose == nil || cfg.NonCompose.WorkspaceFolder == nil || *cfg.NonCompose.WorkspaceFolder != "/workspace" {
			t.Fatalf("expected workspaceFolder '/workspace' in NonCompose, got %v", cfg.NonCompose)
		}
	})
}

func TestValidateBuildPathsAreRelative(t *testing.T) {
	t.Run("returns error when build.dockerfile is absolute", func(t *testing.T) {
		data := []byte(`{"build": {"dockerfile": "/absolute/Dockerfile"}}`)
		_, err := Load(data)
		if err == nil {
			t.Fatal("expected error when build.dockerfile is an absolute path")
		}
	})

	t.Run("returns error when build.context is absolute", func(t *testing.T) {
		data := []byte(`{"build": {"dockerfile": "Dockerfile", "context": "/absolute/context"}}`)
		_, err := Load(data)
		if err == nil {
			t.Fatal("expected error when build.context is an absolute path")
		}
	})

	t.Run("succeeds when build.dockerfile and context are relative", func(t *testing.T) {
		data := []byte(`{"build": {"dockerfile": "Dockerfile", "context": "."}}`)
		cfg, err := Load(data)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.Build == nil {
			t.Fatal("expected Build to be non-nil")
		}
	})
}

func TestValidateWorkspaceFolderIsAbsolute(t *testing.T) {
	t.Run("returns error when workspaceFolder is relative", func(t *testing.T) {
		data := []byte(`{"image": "alpine:latest", "workspaceFolder": "relative/path"}`)
		_, err := Load(data)
		if err == nil {
			t.Fatal("expected error when workspaceFolder is a relative path")
		}
	})
}

func TestLoadEdgeCases(t *testing.T) {
	t.Run("returns error for invalid json", func(t *testing.T) {
		_, err := Load([]byte(`{invalid json`))
		if err == nil {
			t.Fatal("expected error for invalid json")
		}
	})

	t.Run("returns error for empty json", func(t *testing.T) {
		_, err := Load([]byte(`{}`))
		if err == nil {
			t.Fatal("expected error for empty devcontainer (neither image nor build)")
		}
	})

	t.Run("parses mounts", func(t *testing.T) {
		data := []byte(`{
			"image": "alpine:latest",
			"mounts": [
				{"source": "data", "target": "/data", "type": "volume"},
				{"source": "/host/path", "target": "/container/path", "type": "bind"}
			]
		}`)
		cfg, err := Load(data)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.Common == nil || cfg.Common.Mounts == nil || len(cfg.Common.Mounts) != 2 {
			t.Fatalf("expected 2 mounts, got %v", cfg.Common)
		}
	})

	t.Run("parses containerEnv and remoteEnv", func(t *testing.T) {
		data := []byte(`{
			"image": "alpine:latest",
			"containerEnv": {"FOO": "bar"},
			"remoteEnv": {"BAZ": "qux"}
		}`)
		cfg, err := Load(data)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.Common == nil || cfg.Common.ContainerEnv == nil || (*cfg.Common.ContainerEnv)["FOO"] != "bar" {
			t.Fatalf("expected containerEnv FOO=bar, got %v", cfg.Common)
		}
		if cfg.Common.RemoteEnv == nil || (*cfg.Common.RemoteEnv)["BAZ"] != "qux" {
			t.Fatalf("expected remoteEnv BAZ=qux, got %v", cfg.Common)
		}
	})
}
```

- [ ] **Step 2: Run all tests**

```bash
go test -tags containers_image_openpgp ./devpodman/devcontainer/... -v
```

Expected: ALL PASS. If some fail due to CUE error message differences, adjust error string checks in tests to match actual output.

- [ ] **Step 3: Commit**

```bash
git add devpodman/devcontainer/devcontainer_test.go
git commit -m "test: migrate test suite to ResolvedConfig and CUE validation"
```

---

### Task 8: Update CLI caller (debug.go)

**Files:**
- Modify: `devpodman/cmd/devpodman/debug.go`

- [ ] **Step 1: Adapt debug.go to use ResolvedConfig**

In `devpodman/cmd/devpodman/debug.go`, replace the Load usage (the section after `os.ReadFile(validatePath)` calling devcontainer.Load):

Old code:
```go
	buildProps, imgProps, commonProps, err := devcontainer.Load(data)
	if err != nil {
		return err
	}

	if buildProps != nil {
		fmt.Fprintf(os.Stdout, "Successfully loaded build config: %+v\n", buildProps.Build.Args)
	}
	if imgProps != nil {
		fmt.Fprintf(os.Stdout, "Successfully loaded image config: %s\n", imgProps.Image)
	}
	if commonProps != nil {
		fmt.Fprintf(os.Stdout, "Successfully loaded common config: %+v\n", commonProps)
	}
```

New code:
```go
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
```

- [ ] **Step 2: Build the binary**

```bash
go build -tags containers_image_openpgp ./devpodman/cmd/devpodman/
```

Expected: compilation succeeds.

- [ ] **Step 3: Test with sample JSON**

```bash
go run -tags containers_image_openpgp ./devpodman/cmd/devpodman/ debug --validate devpodman/cmd/devpodman/testdata/devcontainer.json
```

Expected: output shows successfully loaded config.

- [ ] **Step 4: Commit**

```bash
git add devpodman/cmd/devpodman/debug.go
git commit -m "refactor: update debug command to use ResolvedConfig"
```

---

### Task 9: Update Taskfile and AGENTS.md

**Files:**
- Modify: `devpodman/Taskfile.yaml`
- Modify: `devpodman/AGENTS.md`

- [ ] **Step 1: Update Taskfile.yaml**

Replace the `pkl:generate` task with `cue:generate` in `devpodman/Taskfile.yaml`:

```yaml
  cue:generate:
    desc: Regenerate model types from CUE schema
    cmds:
      - cue exp gengotypes model/devcontainer.cue
```

- [ ] **Step 2: Update AGENTS.md**

In `devpodman/AGENTS.md`, replace all Pkl references with CUE equivalents:

Under "### Taskfile shortcuts":
```
# Regenerate model types from CUE
go-task cue:generate
```
(replace `go-task pkl:generate` line)

Under "### Raw equivalents":
```
# Regenerate model types from CUE
cue exp gengotypes model/devcontainer.cue
```
(replace the pkl run line)

Under "## Architecture", replace the model/ directory description:
```
├── model/               # CUE schema + auto-generated Go types
│   ├── devcontainer.cue            # Source of truth — CUE schema
│   ├── schema.go                   # Embeds CUE schema for runtime
│   ├── cue_types_model_gen.go      # Generated Go types (DO NOT EDIT)
```

Replace the "## Pkl Workflow" heading and content with:
```markdown
## CUE Workflow

The CUE schema at `model/devcontainer.cue` is the source of truth for devcontainer configuration types.
After modifying it, regenerate Go types into `model/cue_types_model_gen.go`:

```bash
cue exp gengotypes model/devcontainer.cue
```

Generated types support both `json:` struct tags for JSON (un)marshalling.
The schema is embedded at build time via `//go:embed` and used at runtime by `cuelang.org/go` for validation.
```

Under "## Dependency Guidelines", replace:
```
| `github.com/apple/pkl-go` | Pkl schema evaluation and code generation |
```
with:
```
| `cuelang.org/go` | CUE schema evaluation and runtime validation |
```

- [ ] **Step 3: Commit**

```bash
git add devpodman/Taskfile.yaml devpodman/AGENTS.md
git commit -m "docs: update Taskfile and AGENTS.md for CUE migration"
```

---

### Task 10: Remove outdated design doc

**Files:**
- Remove: `devpod/docs/2024-03-27-devcontainer-cli-design.md`

- [ ] **Step 1: Remove outdated doc**

```bash
rm devpod/docs/2024-03-27-devcontainer-cli-design.md
```

- [ ] **Step 2: Commit**

```bash
git add devpod/docs/2024-03-27-devcontainer-cli-design.md
git commit -m "docs: remove outdated PKL-based design doc"
```

---

### Task 11: Full test suite + cleanup

- [ ] **Step 1: Run all tests**

```bash
go test -tags containers_image_openpgp ./devpodman/... -v
```

Workdir: `devpodman`.

Expected: ALL tests PASS.

- [ ] **Step 2: Build the binary**

```bash
go build -tags containers_image_openpgp ./devpodman/cmd/devpodman/
```

Workdir: `devpodman`.

Expected: compilation succeeds, binary produced.

- [ ] **Step 3: Run go mod tidy**

```bash
go mod tidy
```

Workdir: `devpodman`.

- [ ] **Step 4: Run gofmt**

```bash
gofmt -w devpodman/
```

- [ ] **Step 5: Final commit**

```bash
git add -A
git commit -m "chore: finalize CUE migration with full test suite pass"
```

---

**Note:** The generated type names from `cue exp gengotypes` (e.g., `model.DockerfileContainer`, `model.ImageContainer`, `model.DevContainerCommon`, `model.NonComposeBase`, `model.BuildOptions`, `model.Mount`) must be verified against the actual generated output. If the names differ, adjust all code references accordingly. The `Mount` type is generated as `Mount` (from `#Mount` → `Mount`), `buildOptions` becomes `BuildOptions`, etc.
