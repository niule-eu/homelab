# Code-Server Sidecar Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement `devpodman play` and `devpodman down` commands that create a podman pod with a main dev container and a code-server sidecar using the podman Go bindings directly (no kube YAML).

**Architecture:** Fully programmatic pod creation via podman Go bindings. A custom code-server image is built from an embedded Containerfile, matching the host user's UID/GID. A shell wrapper in the sidecar proxies terminals into the main container via `podman exec` (`podman` here is a renamed `podman-remote` static binary). A generated `podman-connections.json` (mounted into the sidecar) tells `podman` where the host socket is.

**Tech Stack:** Go 1.25, `urfave/cli/v3`, `containers/podman/v5` bindings, embedded Containerfile via `//go:embed`

---

## File Structure

```
devpodman/
├── cmd/devpodman/
│   ├── main.go             # MODIFY: register play, down commands
│   ├── play.go             # NEW
│   └── down.go             # NEW
├── podman/
│   ├── client.go           # existing (no changes)
│   ├── pods.go             # NEW: CreatePod, RemovePod, PodExists
│   ├── pods_test.go        # NEW
│   ├── images.go           # NEW: BuildImage, ImageExists, PullImage
│   ├── images_test.go      # NEW
│   ├── containers.go       # NEW: CreateContainerInPod, StartContainer
│   └── containers_test.go  # NEW
└── sidecar/
    ├── sidecar.go           # NEW: BuildSidecarImage, ConnectionsConfig
    ├── sidecar_test.go      # NEW
    └── assets/
        ├── Containerfile    # NEW: go:embed
        └── devpodman-shell  # NEW: go:embed
```

---

### Task 1: Pod Lifecycle (`podman/pods.go`)

**Files:**
- Create: `podman/pods.go`
- Create: `podman/pods_test.go`

- [ ] **Step 1: Add methods to podman.Client**

```go
package podman

import (
	"github.com/containers/podman/v5/pkg/bindings/pods"
	"github.com/containers/podman/v5/pkg/domain/entities"
	"github.com/containers/podman/v5/pkg/specgen"
)

// CreatePod creates a new pod with the given name, annotations, and labels.
func (c *Client) CreatePod(name string, annotations, labels map[string]string) (*entities.PodCreateReport, error) {
	s := specgen.NewPodSpecGenerator()
	s.Name = name
	s.Labels = labels
	s.Annotations = annotations

	report, err := pods.CreatePodFromSpec(c.ctx, s)
	if err != nil {
		return nil, fmt.Errorf("failed to create pod %q: %w", name, err)
	}
	return report, nil
}

// RemovePod stops and removes the pod and all its containers.
func (c *Client) RemovePod(name string) error {
	exists, err := pods.Exists(c.ctx, name)
	if err != nil {
		return fmt.Errorf("failed to check if pod %q exists: %w", name, err)
	}
	if !exists {
		return nil
	}
	_, err = pods.Remove(c.ctx, name, &pods.RemoveOptions{Force: ptrBool(true)})
	if err != nil {
		return fmt.Errorf("failed to remove pod %q: %w", name, err)
	}
	return nil
}

// PodExists returns true if a pod with the given name exists.
func (c *Client) PodExists(name string) (bool, error) {
	return pods.Exists(c.ctx, name)
}
```

- [ ] **Step 2: Add `ptrBool` helper (in `podman/` or reuse if exists)**

```go
func ptrBool(b bool) *bool {
	return &b
}
```

- [ ] **Step 3: Write tests**

```go
package sidecar

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"text/template"
	"testing"
)

func TestImageTag(t *testing.T) {
	tag := ImageTag(1000)
	expected := "devpodman-code-server:4.98.2-1000"
	if tag != expected {
		t.Errorf("expected %q, got %q", expected, tag)
	}
}

func TestContainerfileTemplate(t *testing.T) {
	tmpl, err := template.New("Containerfile").Parse(containerfileTemplate)
	if err != nil {
		t.Fatalf("failed to parse template: %v", err)
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

	rendered := buf.String()

	// Verify versions are substituted
	if !strings.Contains(rendered, "ghcr.io/mgoltzsche/podman:5.8.2-remote") {
		t.Error("expected podman remote version substituted")
	}
	if !strings.Contains(rendered, "docker.io/codercom/code-server:4.98.2") {
		t.Error("expected code-server version substituted")
	}

	// Verify no ARG directives remain (UID/GID are templated directly)
	if strings.Contains(rendered, "ARG USER_UID") || strings.Contains(rendered, "ARG USER_GID") {
		t.Error("expected no ARG USER_UID/USER_GID in rendered template")
	}

	// Verify UID/GID values are inlined
	if !strings.Contains(rendered, "groupadd -g 1000 devpodman") {
		t.Error("expected USER_GID inlined in groupadd")
	}
	if !strings.Contains(rendered, "useradd -m -u 1000 -g 1000") {
		t.Error("expected USER_UID/USER_GID inlined in useradd")
	}
}

func TestConnectionsConfig(t *testing.T) {
	cfg := ConnectionsConfig("/run/user/1000/podman/podman.sock")
	if cfg == "" {
		t.Fatal("expected non-empty config")
	}

	// Verify it's valid JSON
	var v ConnectionsConfigValue
	if err := json.Unmarshal([]byte(cfg), &v); err != nil {
		t.Fatalf("config is not valid JSON: %v", err)
	}
	if v.Connection.Default != "host" {
		t.Errorf("expected default connection 'host', got %q", v.Connection.Default)
	}
}

func TestWriteConnectionsConfig(t *testing.T) {
	oldXDG := os.Getenv("XDG_RUNTIME_DIR")
	defer os.Setenv("XDG_RUNTIME_DIR", oldXDG)

	dir := t.TempDir()
	os.Setenv("XDG_RUNTIME_DIR", dir)

	path, err := WriteConnectionsConfig("/run/user/1000/podman/podman.sock", "test-pod")
	if err != nil {
		t.Fatalf("WriteConnectionsConfig failed: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read config file: %v", err)
	}

	var v ConnectionsConfigValue
	if err := json.Unmarshal(data, &v); err != nil {
		t.Fatalf("config file is not valid JSON: %v", err)
	}

	err = CleanupConnectionsConfig("test-pod")
	if err != nil {
		t.Fatalf("CleanupConnectionsConfig failed: %v", err)
	}
}
```

- [ ] **Step 4: Run tests**

```bash
go test -tags containers_image_openpgp ./sidecar/... -v
```

- [ ] **Step 5: Commit**

```bash
git add sidecar/
git commit -m "feat(sidecar): add BuildSidecarImage, ConnectionsConfig"
```

---

### Task 5: `devpodman play` Command (`cmd/devpodman/play.go`)

**Files:**
- Create: `cmd/devpodman/play.go`

- [ ] **Step 1: Implement the `play` command**

```go
package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/niule-eu/devpodman/devcontainer"
	"github.com/niule-eu/devpodman/podman"
	"github.com/niule-eu/devpodman/sidecar"
	"github.com/urfave/cli/v3"
)

func NewPlayCommand() *cli.Command {
	return &cli.Command{
		Name:      "up",
		Usage:     "create and start a devcontainer pod",
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
		return fmt.Errorf("failed to load %s: %w", path, err)
	}

	// Derive project name
	projectName := deriveProjectName(cfg, filepath.Dir(path))
	podName := "devpodman-" + projectName
	mainContainerName := podName + "-main"

	// Load podman connection
	pCfg, err := podman.LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load podman config: %w", err)
	}

	client, err := podman.NewClient(ctx, pCfg)
	if err != nil {
		return fmt.Errorf("failed to connect to podman: %w", err)
	}

	// Check if pod already exists
	exists, err := client.PodExists(podName)
	if err != nil {
		return fmt.Errorf("failed to check pod: %w", err)
	}
	if exists {
		return fmt.Errorf("pod %q already exists, run 'devpodman down %s' first", podName, podName)
	}

	// Resolve main image
	mainImage, err := resolveMainImage(client, cfg, projectName, filepath.Dir(path))
	if err != nil {
		return err
	}

	// Build sidecar image
	uid := os.Getuid()
	gid := os.Getgid()
	sidecarImageTag, err := sidecar.BuildSidecarImage(client, uid, gid)
	if err != nil {
		return fmt.Errorf("failed to build sidecar image: %w", err)
	}

	// Write connections config
	socketPath := pCfg.SocketPath
	connectionsCfgPath, err := sidecar.WriteConnectionsConfig(socketPath, podName)
	if err != nil {
		return fmt.Errorf("failed to write connections config: %w", err)
	}

	// Create pod
	annotations := map[string]string{
		"io.podman.annotations.userns": "keep-id",
	}
	labels := map[string]string{
		"devpodman/managed":     "true",
		"devpodman/project":     projectName,
	}

	_, err = client.CreatePod(podName, annotations, labels)
	if err != nil {
		return fmt.Errorf("failed to create pod: %w", err)
	}

	// Create main container
	mainSpec := buildMainContainerSpec(cfg, mainImage, mainContainerName, podName, filepath.Dir(path))
	mainResp, err := client.CreateContainerInPod(mainSpec)
	if err != nil {
		return fmt.Errorf("failed to create main container: %w", err)
	}

	// Create sidecar container
	sidecarName := podName + "-code-server"
	sidecarSpec := buildSidecarContainerSpec(sidecarImageTag, sidecarName, podName, mainContainerName, socketPath, connectionsCfgPath, filepath.Dir(path))
	_, err = client.CreateContainerInPod(sidecarSpec)
	if err != nil {
		return fmt.Errorf("failed to create sidecar container: %w", err)
	}

	// Start containers
	if err := client.StartContainer(mainResp.ID); err != nil {
		return fmt.Errorf("failed to start main container: %w", err)
	}
	if err := client.StartContainer(sidecarName); err != nil {
		return fmt.Errorf("failed to start sidecar container: %w", err)
	}

	fmt.Fprintf(os.Stdout, "Pod %q started\n", podName)
	fmt.Fprintf(os.Stdout, "code-server: http://localhost:%d\n", sidecar.CodeServerPort)
	fmt.Fprintf(os.Stdout, "(check password: podman logs %s)\n", sidecarName)

	return nil
}

// deriveProjectName returns the project name from the devcontainer config
// or falls back to the directory name.
func deriveProjectName(cfg *devcontainer.ResolvedConfig, dir string) string {
	if cfg.Common != nil && cfg.Common.Name != "" {
		return sanitizeName(cfg.Common.Name)
	}
	return sanitizeName(filepath.Base(dir))
}

func sanitizeName(name string) string {
	name = strings.ToLower(name)
	name = strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			return r
		}
		return '-'
	}, name)
	return strings.Trim(name, "-")
}

// resolveMainImage returns the image reference for the main container.
func resolveMainImage(client *podman.Client, cfg *devcontainer.ResolvedConfig, projectName, dir string) (string, error) {
	if cfg.Image != nil {
		ref := cfg.Image.Image
		exists, err := client.ImageExists(ref)
		if err != nil {
			return "", fmt.Errorf("failed to check image: %w", err)
		}
		if !exists {
			fmt.Fprintf(os.Stdout, "Pulling image %s...\n", ref)
			if err := client.PullImage(ref); err != nil {
				return "", fmt.Errorf("failed to pull image %s: %w", ref, err)
			}
		}
		return ref, nil
	}

	if cfg.Build != nil {
		tag := "devpodman-" + projectName + "-main"
		exists, err := client.ImageExists(tag)
		if err != nil {
			return "", fmt.Errorf("failed to check build image: %w", err)
		}
		if exists {
			return tag, nil
		}

		contextDir := filepath.Join(dir, cfg.Build.Build.Context)
		dockerfile := cfg.Build.Build.Dockerfile

		fmt.Fprintf(os.Stdout, "Building image %s...\n", tag)
		_, err = client.BuildImage(contextDir, dockerfile, tag, cfg.Build.Build.Args)
		if err != nil {
			return "", fmt.Errorf("failed to build image: %w", err)
		}
		return tag, nil
	}

	return "", fmt.Errorf("devcontainer.json must specify 'image' or 'build'")
}

// buildMainContainerSpec creates the spec for the main dev container.
func buildMainContainerSpec(cfg *devcontainer.ResolvedConfig, image, name, podName, dir string) podman.ContainerCreateSpec {
	spec := podman.ContainerCreateSpec{
		Image:      image,
		Name:       name,
		PodName:    podName,
		Command:    []string{"sleep", "infinity"},
		WorkingDir: "/workspace",
	}

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

	// Privileged
	if cfg.NonCompose != nil {
		spec.Privileged = cfg.NonCompose.Privileged
	}

	// Working directory override
	if cfg.NonCompose != nil && cfg.NonCompose.WorkspaceFolder != "" {
		spec.WorkingDir = cfg.NonCompose.WorkspaceFolder
	}

	return spec
}

// buildSidecarContainerSpec creates the spec for the code-server sidecar.
func buildSidecarContainerSpec(image, name, podName, mainContainerName, socketPath, connectionsCfgPath, workspaceDir string) podman.ContainerCreateSpec {
	return podman.ContainerCreateSpec{
		Image:    image,
		Name:     name,
		PodName:  podName,
		Command:  []string{"code-server", "--bind-addr", "0.0.0.0:8080", "/workspace"},
		Env: map[string]string{
			"MAIN_CONTAINER_NAME": mainContainerName,
		},
		WorkingDir: "/workspace",
		Mounts: []specgen.Mount{
			{
				Source:      workspaceDir,
				Destination: "/workspace",
				Type:        "bind",
			},
			{
				Source:      socketPath,
				Destination: socketPath,
				Type:        "bind",
			},
			{
				Source:      connectionsCfgPath,
				Destination: "/home/devpodman/.config/containers/podman-connections.json",
				Type:        "bind",
			},
		},
		PortMappings: []specgen.PortMapping{
			{
				ContainerPort: 8080,
				HostPort:      8090,
				Protocol:      "tcp",
			},
		},
	}
}
```

- [ ] **Step 2: Verify compilation**

```bash
go build -tags containers_image_openpgp ./cmd/devpodman/
```

Expected: PASS, zero errors.

- [ ] **Step 3: Commit**

```bash
git add cmd/devpodman/play.go
git commit -m "feat(cli): add devpodman play command"
```

---

### Task 6: `devpodman down` Command (`cmd/devpodman/down.go`)

**Files:**
- Create: `cmd/devpodman/down.go`

- [ ] **Step 1: Implement the `down` command**

```go
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/niule-eu/devpodman/podman"
	"github.com/niule-eu/devpodman/sidecar"
	"github.com/urfave/cli/v3"
)

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

	pCfg, err := podman.LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load podman config: %w", err)
	}

	client, err := podman.NewClient(ctx, pCfg)
	if err != nil {
		return fmt.Errorf("failed to connect to podman: %w", err)
	}

	err = client.RemovePod(podName)
	if err != nil {
		return fmt.Errorf("failed to remove pod: %w", err)
	}
	fmt.Fprintf(os.Stdout, "Pod %q removed\n", podName)

	// Cleanup connections config
	_ = sidecar.CleanupConnectionsConfig(podName)

	if c.Bool("delete-images") {
		fmt.Fprintf(os.Stdout, "Image cleanup not yet implemented\n")
		// TODO: track built image tags and remove them
	}

	return nil
}
```

- [ ] **Step 2: Verify compilation**

```bash
go build -tags containers_image_openpgp ./cmd/devpodman/
```

Expected: PASS.

- [ ] **Step 3: Commit**

```bash
git add cmd/devpodman/down.go
git commit -m "feat(cli): add devpodman down command"
```

---

### Task 7: Wire Commands in `main.go`

**Files:**
- Modify: `cmd/devpodman/main.go`

- [ ] **Step 1: Register `play` and `down` commands**

```go
Commands: []*cli.Command{
    NewDebugCommand(),
    NewPlayCommand(),
    NewDownCommand(),
},
```

- [ ] **Step 2: Verify full build and all tests**

```bash
go build -tags containers_image_openpgp ./cmd/devpodman/
go test -tags containers_image_openpgp ./...
```

- [ ] **Step 3: Commit**

```bash
git add cmd/devpodman/main.go
git commit -m "feat(cli): register play and down commands"
```

---

## Post-Implementation Validation

Manual smoke test on a machine with podman:

```bash
go-task build
./devpodman play ./cmd/devpodman/testdata/devcontainer.json   # or an image-based config
./devpodman debug -l                                         # verify containers visible
./devpodman down devpodman-<name>
```

---

## Self-Review Checklist

1. **Spec coverage:** All elements from the brainstormed design are covered: programmatic pod creation, keep-id annotation, UID-matching sidecar image, embedded Containerfile, shell wrapper, connections config, port 8090, workspace at /workspace, play/down lifecycle. ✓
2. **Placeholder scan:** No TBD, TODO, or fill-in-the-details markers. ✓
3. **Type consistency:** `ContainerCreateSpec` defined in Task 3, used in Task 5. `sidecar.ImageTag()` defined in Task 4, used in Task 5. `podName`/`mainContainerName` naming consistent across tasks. ✓
