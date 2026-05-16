# Kube Play Migration Design

## Problem

The current engine uses `specgen.SpecGenerator`, `ContainerCreateOptions`, and `specgenutil.FillOutSpecGen` to construct container specs programmatically. This is tightly coupled to podman internals, requires a cobra-based runArgs parser, and produces opaque pod+container creation — there's no inspectable intermediate representation.

## Solution

Replace specgen-based container creation with podman's `kube.PlayWithBody()` / `kube.DownWithBody()` bindings. Generate a Kubernetes Deployment YAML from `ResolvedConfig` and let podman handle pod and container lifecycle in a single call.

## Approach

Stateless YAML generation: `GenerateKubeYAML()` is a pure function from `ResolvedConfig` to `[]byte`. The generated YAML is stored as a base64 annotation on the pod so `down` can retrieve it without needing the original devcontainer.json.

## Schema Changes

### devcontainer.cue — #devpodmanCustomization (already edited)

```cue
#devpodmanCustomization: {
    command:    [...string] // defaults to ["sleep", "infinity"]
    args:       [...string] // defaults to []
    workdir:    #devpodmanWorkdir
    network:    #devpodmanNetwork
    codeServer: #devpodmanCodeServer
}

#devpodmanNetwork: {
    enabled: bool | *true
    name?:   string & =~"^[a-z0-9A-Z][a-z0-9A-Z_.-]*[a-z0-9A-Z]$" // defaults to pod name + "-network"
}

#devpodmanCodeServer: {
    enabled:       bool | *true
    containerPort: int & <=65535 & >=0 | *8080
    hostPort:      int & <=65535 & >=0 | *8080
}

#devpodmanWorkdir: {
    emptyVol: bool | *false
}
```

Fields that already exist in `#nonComposeBase` or `#devContainerCommon` (privileged, overrideCommand, ports, containerEnv, etc.) are not duplicated — they are inferred from those definitions at YAML generation time.

### Default Handling

CUE defaults (`*true`, `*8080`, etc.) are validation-time only — they do not carry into the generated Go structs. The generated `DevpodmanCustomization` type has plain fields:

```go
type DevpodmanCustomization struct {
    Command    []string           `json:"command"`
    Args       []string           `json:"args"`
    Workdir    DevpodmanWorkdir   `json:"workdir"`
    Network    DevpodmanNetwork   `json:"network"`
    CodeServer DevpodmanCodeServer `json:"codeServer"`
}

type DevpodmanNetwork struct {
    Enabled bool   `json:"enabled"`
    Name     string `json:"name,omitempty"`
}

type DevpodmanCodeServer struct {
    Enabled       bool  `json:"enabled"`
    ContainerPort int64 `json:"containerPort"`
    HostPort      int64 `json:"hostPort"`
}
```

When `devcontainer.json` omits these fields, the Go zero values (`false`, `0`, `nil`) will be populated. `GenerateKubeYAML` must apply defaults at runtime:

| Field | Zero value | Default |
|-------|-----------|---------|
| `Command` | `nil` | `["sleep", "infinity"]` |
| `Args` | `nil` | `[]` |
| `Network.Enabled` | `false` | `true` |
| `Network.Name` | `""` | `podName + "-network"` |
| `CodeServer.Enabled` | `false` | `true` |
| `CodeServer.ContainerPort` | `0` | `8080` |
| `CodeServer.HostPort` | `0` | `8080` |
| `Workdir.EmptyVol` | `false` | `false` |

Note: `Network.Enabled` and `CodeServer.Enabled` default to `false` at the Go zero-value level despite CUE `*true`. This means the YAML generator must treat `false` as "use default = true" unless explicitly set to `false`. This requires either pointer fields (`*bool`) in the Go struct or a convention that absent = enabled. The CUE validation ensures the JSON is valid, but the Go code must handle the zero-value case.

## YAML Generation

### Function Signature

```go
func GenerateKubeYAML(cfg *devcontainer.ResolvedConfig, opts KubePlayOpts) ([]byte, error)
```

`KubePlayOpts` carries runtime values that cannot come from devcontainer.json:

- `SocketPath string` — host podman socket path
- `WorkspaceDir string` — host workspace directory
- `SidecarImage string` — tag for the built sidecar image

### YAML Structure

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: devpodman-<name>
  labels:
    devpodman.io/managed: "true"
  annotations:
    devpodman.io/kube-yaml: <base64 of this YAML>
spec:
  replicas: 1
  selector:
    matchLabels:
      app: devpodman-<name>
  template:
    metadata:
      labels:
        app: devpodman-<name>
    spec:
      containers:
        - name: main
          image: <cfg.Image.Image or "podName-main">
          command: <cfg.Common.Customizations.Devpodman.Command>
          args: <cfg.Common.Customizations.Devpodman.Args>
          env: <from NonCompose.ContainerEnv + Common.RemoteEnv>
          workingDir: <NonCompose.WorkspaceFolder or "/workspace">
          volumeMounts: [...]
          ports: [...]
        - name: sidecar
          image: <sidecarTag>
          command: ["code-server", "--bind-addr", "0.0.0.0:<containerPort>", "/workspace"]
          env:
            - name: MAIN_CONTAINER_NAME
              value: <podName>-main
          volumeMounts: [...]
          ports: [...]
      volumes: [...]
```

### Volume Mapping

| Source | Kubernetes Volume Type | When |
|--------|----------------------|------|
| Workspace dir | `hostPath` (bind mount) | `EmptyVol == false` (default) |
| Workspace dir | Named volume | `EmptyVol == true` |
| Podman socket | `hostPath` | Always |
| Connections volume | Created via VolumeImportEffect before kube play | Always |

Connections volume cannot be expressed in kube YAML (it requires tar import), so it remains an imperative step before `KubePlayEffect`.

### Port Mapping

Ports from `cfg.NonCompose.AppPort` + code-server port (from `devpodmanCustomization.codeServer`) are mapped as `containerPort` / `hostPort` in the container specs.

## Effects Layer

### Removed Effects

- `CreatePodEffect`
- `CreateContainerEffect`
- `StartContainerEffect`
- `StartPodEffect`

### New Effects

- `KubePlayEffect` — calls `kube.PlayWithBody()` with generated YAML. Options: `Start: ptrBool(true)`, `Network: networkName` (if enabled), `PublishPorts` from config.
- `KubeDownEffect` — inspects the pod's `devpodman.io/kube-yaml` annotation via `pods.Inspect()`, decodes base64, calls `kube.DownWithBody()`.

### Retained Effects (unchanged)

- `CreateNetworkEffect` / `RemoveNetworkEffect`
- `BuildImageEffect`
- `VolumeImportEffect` / `RemoveVolumeEffect`
- `FileWrite`, `FileDelete`, `Stdout`, `NoOp`
- `RemovePodEffect` (kept as fallback for legacy pods without annotation)

### Updated Play() Flow

1. `CreateNetworkEffect` (if `cfg.Common.Customizations.Devpodman.Network.Enabled`)
2. `FileWrite` x 3 (Containerfile, shell script, settings.json to temp dir)
3. `BuildImageEffect` (sidecar image)
4. `VolumeImportEffect` (connections volume)
5. `KubePlayEffect` (single call — replaces CreatePod + CreateContainerx2 + StartPod)

### Updated Down() Flow

1. `KubeDownEffect` (read annotation, decode YAML, call kube.DownWithBody)
2. `RemoveVolumeEffect` x 2 (connections, workspace — only if EmptyVol)
3. `RemoveNetworkEffect` (if network was enabled)

## Imports Changed

### Removed

- `github.com/containers/podman/v5/cmd/podman/common`
- `github.com/containers/podman/v5/pkg/domain/entities` (ContainerCreateOptions only)
- `github.com/containers/podman/v5/pkg/specgen`
- `github.com/containers/podman/v5/pkg/specgenutil`
- `github.com/opencontainers/runtime-spec/specs-go` (OCI mount types)

### Added

- `github.com/containers/podman/v5/pkg/bindings/kube`
- `k8s.io/api/apps/v1` and `k8s.io/api/core/v1` (Deployment, Pod, Container types)
- `sigs.k8s.io/yaml` (for marshaling to YAML)
- `encoding/base64` (for annotation)

## File Changes

### Deleted

| File | Reason |
|------|--------|
| `pkg/engine/runargs.go` | runArgs dropped, no more ContainerCreateOptions |
| `pkg/engine/runargs_test.go` | tests for runArgs parser |

### Rewritten

| File | Changes |
|------|---------|
| `pkg/engine/engine.go` | Play()/Down() rewritten. buildMainContainerSpec/buildSidecarContainerSpec removed. GenerateKubeYAML called instead. |
| `pkg/engine/effects.go` | CreatePod/CreateContainer/StartPod/StartContainer removed. KubePlay/KubeDown added. Imports updated. |

### New

| File | Purpose |
|------|---------|
| `pkg/engine/kubeyaml.go` | GenerateKubeYAML() pure function |
| `pkg/engine/kubeyaml_test.go` | Unit tests for YAML generation |

### Regenerated

| File | Reason |
|------|--------|
| `pkg/model/cue_types_model_gen.go` | CUE schema changed, regenerate with `cue exp gengotype .` |

### Unchanged

- `pkg/devcontainer/devcontainer.go`
- `pkg/effects/effects.go`
- `pkg/engine/sidecar.go`
- `pkg/engine/assets/*`
- `internal/podman/*`
- `internal/cli/*` (minor updates only)

## CLI Impact

### generate (new command)

Reads devcontainer.json, validates, resolves config, calls `GenerateKubeYAML()`, writes to stdout. User can pipe to `podman kube play -` or inspect it.

### play (updated)

Same flow, uses `KubePlayEffect` instead of individual pod/container/start effects.

### down (updated)

Derives pod name, inspects `devpodman.io/kube-yaml` annotation, decodes, calls `KubeDownEffect`. Falls back to name-based removal for legacy pods.

### debug (minor update)

Shows generated kube YAML alongside resolved config.

## Testing Strategy

### Unit Tests

- `kubeyaml_test.go`: table-driven tests for `GenerateKubeYAML` — image-based, dockerfile-based, with/without code-server, with/without EmptyVol, with customizations, verify YAML structure (deployment name, container images, commands, env, volumes, ports, annotation round-trip).

### Integration Tests

- `effects_test.go`: replace CreatePod/CreateContainer/StartPod tests with KubePlay/KubeDown tests. Verify annotation-based down works.

### Removed

- `runargs_test.go`: deleted with runargs.go.
- `engine_test.go`: integration test for Play updated to use kube play.

## Open Questions

- **VolumeImport for connections**: The connections volume is created via `VolumeImportEffect` (podman volume + tar import). Under kube Play, this remains an imperative step before `KubePlayEffect`. The volume is referenced by name in the kube YAML.
- **Network attachment**: `kube.PlayWithBody` supports a `Network` option in `PlayOptions`. Whether the devpodman network is specified in the YAML or via PlayOptions needs testing during implementation. Current design uses `PlayOptions.Network` to match existing behavior.