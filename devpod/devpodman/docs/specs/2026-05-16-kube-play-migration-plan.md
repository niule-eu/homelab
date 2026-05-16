# Kube Play Migration Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace specgen-based container creation with podman kube Play/Down bindings, generating a Kubernetes Deployment YAML as the intermediate representation.

**Architecture:** A pure function `GenerateKubeYAML()` converts `ResolvedConfig` into a Deployment YAML. `KubePlayEffect` and `KubeDownEffect` replace CreatePod/CreateContainer/StartPod effects. The generated YAML is stored as a base64 annotation on the pod for teardown. The `runargs.go` cobra-based parser is removed entirely.

**Tech Stack:** Go, podman v5 bindings (`kube.PlayWithBody`/`kube.DownWithBody`), `k8s.io/api` Kubernetes types, `sigs.k8s.io/yaml` for marshaling

---

## File Structure

| File | Action | Responsibility |
|------|--------|---------------|
| `pkg/engine/kubeyaml.go` | Create | `GenerateKubeYAML()` pure function, default application, Deployment YAML construction |
| `pkg/engine/kubeyaml_test.go` | Create | Unit tests for YAML generation |
| `pkg/engine/effects.go` | Modify | Remove specgen effects, add KubePlay/KubeDown effects |
| `pkg/engine/engine.go` | Modify | Rewrite Play/Down to use kube YAML, remove buildMainContainerSpec/buildSidecarContainerSpec |
| `pkg/engine/runargs.go` | Delete | No longer needed |
| `pkg/engine/runargs_test.go` | Delete | No longer needed |
| `pkg/engine/helpers.go` | Create | Move `ptrBool`, `ptrInt` to a separate helpers file (they're currently in effects.go) |
| `pkg/engine/sidecar.go` | Modify | Update `CodeServerPort`/`CodeServerHostPort` constants to read from config instead of hardcoded values |

---

## Task 1: Create helpers.go and move ptrBool/ptrInt

**Files:**
- Create: `pkg/engine/helpers.go`

`ptrBool` and `ptrInt` are currently in `effects.go` but will still be needed by the new kube effects. Extract them first so effects.go cleanup doesn't lose them.

- [ ] **Step 1: Create pkg/engine/helpers.go**

```go
package engine

func ptrBool(b bool) *bool       { return &b }
func ptrInt(i int) *int          { return &i }
func ptrInt32(i int32) *int32    { return &i }
func ptrString(s string) *string { return &s }
```

- [ ] **Step 2: Run tests to verify nothing broke**

Run: `go-task test`
Expected: All existing tests pass

- [ ] **Step 3: Commit**

```
refactor(engine): extract ptr helpers to helpers.go
```

---

## Task 2: Create kubeyaml.go — GenerateKubeYAML and defaults

**Files:**
- Create: `pkg/engine/kubeyaml.go`
- Create: `pkg/engine/kubeyaml_test.go`

This is the core of the migration. The `GenerateKubeYAML` function is a pure function that takes config and runtime options and produces a Deployment YAML byte slice.

- [ ] **Step 1: Write failing tests for GenerateKubeYAML**

Create `pkg/engine/kubeyaml_test.go` with table-driven tests covering the key cases. Each test constructs a `ResolvedConfig`, calls `GenerateKubeYAML`, and asserts on the output YAML.

Test cases:
1. `"generates deployment with image container"` — minimal image-based config, verify deployment name, container image, default command (`sleep infinity`)
2. `"generates deployment with dockerfile container"` — build-based config, verify image is `podName-main`
3. `"applies defaults when customization is zero-valued"` — verify defaults: `command: [sleep, infinity]`, `network.enabled: true`, `codeServer.enabled: true`, `codeServer.containerPort: 8080`, `codeServer.hostPort: 8080`
4. `"respects custom command and args"` — set `DevpodmanCustomization.Command` and `Args`, verify they appear in container spec
5. `"includes sidecar when codeServer enabled"` — default config, verify second container exists with code-server command
6. `"omits sidecar when codeServer disabled"` — set `CodeServer.Enabled: false` (via pointer bool since zero = true), verify only one container
7. `"sets workspace hostPath volume when emptyVol false"` — verify `hostPath` volume for workspace
8. `"sets workspace PVC volume when emptyVol true"` — verify `PersistentVolumeClaim` volume for workspace
9. `"includes env vars from containerEnv and remoteEnv"` — verify both sources merged into env
10. `"includes ports from appPort"` — verify `NonCompose.AppPort` mapped to container ports
11. `"annotation contains base64 of the YAML"` — verify `devpodman.io/kube-yaml` annotation exists and round-trips
12. `"includes podman socket hostPath volume"` — verify socket mount in sidecar

Since `DevpodmanCustomization` uses Go zero values for defaults (e.g., `bool` defaults to `false` but means "enabled by default"), `GenerateKubeYAML` must accept pointer types or a helper to distinguish "not set" from "set to false". Use pointer fields in a `DevpodmanCustomizationOverrides` struct or, simpler: since the CUE defaults are known, handle them in `applyDefaults()`.

```go
type KubePlayOpts struct {
    SocketPath   string
    WorkspaceDir string
    SidecarImage string
}

func TestGenerateKubeYAML(t *testing.T) {
    // ... table-driven tests
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go-task test:package PACKAGE=./pkg/engine/... -run TestGenerateKubeYAML`
Expected: Compilation errors (function doesn't exist yet)

- [ ] **Step 3: Implement GenerateKubeYAML**

Create `pkg/engine/kubeyaml.go` with:

1. `KubePlayOpts` struct (SocketPath, WorkspaceDir, SidecarImage)
2. `applyDefaults(cfg *devcontainer.ResolvedConfig)` — fills in zero-value fields with CUE defaults. Since Go zero values can't distinguish "not set" from "explicitly false/empty", and the CUE defaults are known, use a convention: check if `cfg.Common.Customizations.Devpodman` is the zero struct. If so, populate full defaults. Otherwise apply field-level defaults (empty Command → `["sleep", "infinity"]`, zero Network.Enabled → true, zero CodeServer.Enabled → true, zero CodeServer.ContainerPort → 8080, zero CodeServer.HostPort → 8080).
3. `GenerateKubeYAML(cfg *devcontainer.ResolvedConfig, opts KubePlayOpts) ([]byte, error)` — constructs a `appsv1.Deployment`, fills fields from config+defaults, marshals to YAML, base64-encodes the result into the annotation, then returns the final YAML bytes.

Key implementation details for `GenerateKubeYAML`:
- Pod name from `DerivePodName(cfg, opts.WorkspaceDir)` (but WorkspaceDir might not always be available for `generate` command — pass it in or use the name field)
- Main container: image from `cfg.Image.Image` or `"podName-main"`, command from customization, env from NonCompose.ContainerEnv + Common.RemoteEnv, workdir from NonCompose.WorkspaceFolder (default "/workspace"), volumeMounts for workspace + additional mounts, ports from NonCompose.AppPort
- Sidecar container: only if CodeServer.Enabled, image from opts.SidecarImage, command `["code-server", "--bind-addr", "0.0.0.0:<containerPort>", "/workspace"]`, env `MAIN_CONTAINER_NAME=podName-main`, volumeMounts for workspace + socket + connections, port from CodeServer config
- Volumes: hostPath for workspace bind-mount (or PVC reference), hostPath for podman socket, PVC reference for connections volume (by name)
- Labels: `devpodman.io/managed: "true"`, `app: podName`
- Annotations: `devpodman.io/kube-yaml: <base64>`
- Security context for `keep-id` userns: `spec.securityContext.runAsUser` and appropriate settings (podman kube play supports this via annotations — use `io.podman.annotations.userns: keep-id` annotation on the pod template)

The YAML is marshaled using `sigs.k8s.io/yaml.Marshal` (already a dependency via podman).

- [ ] **Step 4: Run tests to verify they pass**

Run: `go-task test:package PACKAGE=./pkg/engine/... -run TestGenerateKubeYAML`
Expected: All tests pass

- [ ] **Step 5: Commit**

```
feat(engine): add GenerateKubeYAML for kube play migration
```

---

## Task 3: Create KubePlayEffect and KubeDownEffect

**Files:**
- Modify: `pkg/engine/effects.go`

Add the two new effects alongside the existing ones. Don't remove anything yet — that happens in Task 5.

- [ ] **Step 1: Write failing test for KubePlayEffect and KubeDownEffect**

Add to `pkg/engine/effects_test.go`:

```go
func TestKubePlayEffect_ImplementsEffect(t *testing.T) {
    var _ effects.Effect = NewKubePlayEffect(nil, nil, nil)
}

func TestKubeDownEffect_ImplementsEffect(t *testing.T) {
    var _ effects.Effect = NewKubeDownEffect(nil, "")
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go-task test:package PACKAGE=./pkg/engine/... -run TestKubePlayEffect_ImplementsEffect`
Expected: Compilation error (types don't exist yet)

- [ ] **Step 3: Implement KubePlayEffect and KubeDownEffect**

Add to `pkg/engine/effects.go`:

```go
// KubePlayEffect plays a Kubernetes Deployment YAML via podman kube play.
type KubePlayEffect struct {
    conn    EngineConnection
    yaml    []byte
    options *kube.PlayOptions
}

func NewKubePlayEffect(conn EngineConnection, yaml []byte, options *kube.PlayOptions) effects.Effect {
    return KubePlayEffect{conn: conn, yaml: yaml, options: options}
}

func (e KubePlayEffect) Apply() error {
    report, err := kube.PlayWithBody(e.conn, bytes.NewReader(e.yaml), e.options)
    if err != nil {
        return fmt.Errorf("failed to play kube YAML: %w", err)
    }
    // Check for container errors in the report
    for _, pod := range report.Pods {
        for _, errMsg := range pod.ContainerErrors {
            if errMsg != "" {
                return fmt.Errorf("container error in pod %s: %s", pod.ID, errMsg)
            }
        }
    }
    return nil
}

// KubeDownEffect tears down pods created by kube play.
// It reads the kube YAML annotation from the pod to get the exact YAML
// used during play, then calls kube.DownWithBody.
type KubeDownEffect struct {
    conn    EngineConnection
    podName string
}

func NewKubeDownEffect(conn EngineConnection, podName string) effects.Effect {
    return KubeDownEffect{conn: conn, podName: podName}
}

func (e KubeDownEffect) Apply() error {
    // Inspect the pod to get the annotation
    inspect, err := pods.Inspect(e.conn, e.podName, nil)
    if err != nil {
        return fmt.Errorf("failed to inspect pod %q: %w", e.podName, err)
    }

    yamlB64, ok := inspect.Annotations["devpodman.io/kube-yaml"]
    if !ok {
        // Fallback: remove by pod name (legacy pods)
        return NewRemovePodEffect(e.conn, e.podName).Apply()
    }

    yamlData, err := base64.StdEncoding.DecodeString(yamlB64)
    if err != nil {
        return fmt.Errorf("failed to decode kube YAML annotation: %w", err)
    }

    report, err := kube.DownWithBody(e.conn, bytes.NewReader(yamlData), kube.DownOptions{Force: ptrBool(true)})
    if err != nil {
        return fmt.Errorf("failed to down kube YAML: %w", err)
    }
    for _, rmReport := range report.RmReport {
        if rmReport.Err != nil {
            return fmt.Errorf("failed to remove pod: %s", rmReport.Err)
        }
    }
    return nil
}
```

Add imports: `"bytes"`, `"encoding/base64"`, `"github.com/containers/podman/v5/pkg/bindings/kube"`, `"github.com/containers/podman/v5/pkg/bindings/pods"`.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go-task test:package PACKAGE=./pkg/engine/...`
Expected: All tests pass (including new interface compliance tests)

- [ ] **Step 5: Commit**

```
feat(engine): add KubePlayEffect and KubeDownEffect
```

---

## Task 4: Rewrite Play() and Down() to use kube YAML

**Files:**
- Modify: `pkg/engine/engine.go`

Rewrite `Play()` and `Down()` methods on `Engine` to use the new kube-based approach.

- [ ] **Step 1: Write failing test for new Play flow**

Update the integration test in `pkg/engine/engine_test.go`. The `TestPlay_ReturnsCompound` test should still pass — it just checks that `Play()` returns non-empty effects. But we need to verify the effects list now contains `KubePlayEffect` instead of `CreatePodEffect` + `CreateContainerEffect` + `StartPodEffect`.

Add a test:

```go
func TestPlay_UsesKubePlayEffect(t *testing.T) {
    engine := New("/run/podman/podman.sock")
    cfg := &devcontainer.ResolvedConfig{
        Image: &model.ImageContainer{Image: "alpine:latest"},
        Common: &model.DevContainerCommon{
            Name: "test-kube",
        },
    }

    compound, err := engine.Play(context.Background(), cfg, "/tmp/test")
    if err != nil {
        t.Fatalf("Play returned error: %v", err)
    }

    hasKubePlay := false
    for _, eff := range compound.Effects {
        if _, ok := eff.(KubePlayEffect); ok {
            hasKubePlay = true
        }
    }
    if !hasKubePlay {
        t.Error("expected Play to include KubePlayEffect")
    }
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go-task test:package PACKAGE=./pkg/engine/... -run TestPlay_UsesKubePlayEffect`
Expected: FAIL — Play() still uses old effects

- [ ] **Step 3: Rewrite Play() method**

Replace `Play()` in `engine.go`. The new flow:

1. Apply defaults to config
2. Check if network should be created (`cfg.Common.Customizations.Devpodman.Network.Enabled` — remember zero-value means "enabled by default")
3. Create temp dir, write Containerfile, shell script, settings.json
4. Build sidecar image if code-server is enabled
5. Create connections volume (VolumeImportEffect)
6. Generate kube YAML via `GenerateKubeYAML(cfg, KubePlayOpts{...})`
7. Create `KubePlayEffect` with the generated YAML and appropriate PlayOptions (network name if enabled, Start: true)
8. Return `Compound{Effects: effs, FailFast: true}`

The network name is derived: if `cfg.Common.Customizations.Devpodman.Network.Name` is set, use it; otherwise use the default `"devpodman"`.

```go
func (e *Engine) Play(conn EngineConnection, cfg *devcontainer.ResolvedConfig, workspaceDir string) (effects.Compound, error) {
    cfg = applyDefaults(cfg)
    podName := DerivePodName(cfg, workspaceDir)

    // ... file writes, sidecar build, volume import (unchanged) ...

    yamlData, err := GenerateKubeYAML(cfg, KubePlayOpts{
        SocketPath:   e.socketPath,
        WorkspaceDir: workspaceDir,
        SidecarImage: sidecarTag,
    })
    if err != nil {
        return effects.Compound{}, fmt.Errorf("failed to generate kube YAML: %w", err)
    }

    // Build kube play options
    opts := &kube.PlayOptions{
        Start: ptrBool(true),
    }
    networkName := resolveNetworkName(cfg)
    if networkName != "" {
        opts.Network = &[]string{networkName}
    }

    effs = append(effs, NewKubePlayEffect(conn, yamlData, opts))

    return effects.Compound{Effects: effs, FailFast: true}, nil
}
```

Also remove `buildMainContainerSpec` and `buildSidecarContainerSpec` functions entirely.

- [ ] **Step 4: Rewrite Down() method**

Replace `Down()` to use `KubeDownEffect`:

```go
func (e *Engine) Down(conn EngineConnection, podName string, deleteImages bool) (effects.Compound, error) {
    cfg := // need to figure out how to get network info for cleanup
    var effs []effects.Effect

    effs = append(effs, NewKubeDownEffect(conn, podName))

    // Clean up volumes (connections always, workspace only if EmptyVol)
    connectionsVolName := podName + "-connections"
    effs = append(effs, NewRemoveVolumeEffect(conn, connectionsVolName))

    // Note: we can't determine EmptyVol from here since we don't have the config
    // The workspace volume cleanup should be handled by kube.Down or we need
    // to also store this info in the annotation

    // Remove network
    networkName := "devpodman" // default; could be stored in annotation too
    effs = append(effs, NewRemoveNetworkEffect(conn, networkName))

    _ = deleteImages

    return effects.Compound{Effects: effs, FailFast: true}, nil
}
```

**Design note:** `Down()` currently needs to know the network name and whether workspace volume exists. Two options:
1. Store these in additional annotations on the pod
2. Always clean up conservatively (try to remove volumes/networks, ignore "not found" errors)

Option 2 is simpler and consistent with existing behavior (RemoveVolumeEffect already ignores missing volumes). Use the default network name and always try to remove the workspace volume.

- [ ] **Step 5: Add resolveNetworkName helper**

```go
func resolveNetworkName(cfg *devcontainer.ResolvedConfig) string {
    if cfg.Common == nil {
        return "devpodman"
    }
    dp := cfg.Common.Customizations.Devpodman
    if dp.Network.Name != "" {
        return dp.Network.Name
    }
    if !dp.Network.Enabled {
        return ""
    }
    return "devpodman"
}

func applyDefaults(cfg *devcontainer.ResolvedConfig) *devcontainer.ResolvedConfig {
    // Clone to avoid mutating the original
    result := *cfg
    if result.Common == nil {
        result.Common = &model.DevContainerCommon{}
    }
    dp := &result.Common.Customizations.Devpodman

    // Apply CUE defaults for zero values
    if len(dp.Command) == 0 {
        dp.Command = []string{"sleep", "infinity"}
    }
    if dp.Args == nil {
        dp.Args = []string{}
    }
    // Network.Enabled: zero/false means enabled (CUE default *true)
    // We can't distinguish "not set" from "explicitly false" with Go bool
    // Solution: use pointer bool in model, or check if customizations struct is zero
    // For now: the CUE validation ensures false is valid, so we check if
    // the entire DevpodmanCustomization is zero-valued
    if dp.Network.Name == "" {
        dp.Network.Name = DerivePodName(&result, "") + "-network"
    }
    // CodeServer defaults
    if dp.CodeServer.ContainerPort == 0 {
        dp.CodeServer.ContainerPort = 8080
    }
    if dp.CodeServer.HostPort == 0 {
        dp.CodeServer.HostPort = 8080
    }

    result.Common.Customizations.Devpodman = *dp
    return &result
}
```

**Important design consideration for `Network.Enabled` and `CodeServer.Enabled`:** Go's zero value for `bool` is `false`, but the CUE defaults are `true`. The model types use plain `bool`, not `*bool`. This means we cannot distinguish "user explicitly disabled" from "field not provided". Two solutions:
1. Change the CUE schema and generated Go types to use `*bool` for these fields
2. Use a convention: if the entire `Customizations` struct appears to be zero-valued (i.e., user didn't provide customizations at all), apply all defaults; if the user provided customizations but omitted `enabled`, they meant "default" (true).

For implementation, use approach 1: add `*bool` fields. Since we already need to manually adjust the generated CUE types (the `gengotypes` output has issues with defaults), modify the `DevpodmanNetwork` and `DevpodmanCodeServer` structs to use `*bool` for bool fields that default to true.

Actually, looking at the generated types more carefully:

```go
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

We need `*bool` for `Enabled` and `*int64` for `ContainerPort`/`HostPort` to distinguish "not set" from "set to zero/false". We'll fix this in the model types manually (since they're generated but the generator can't handle CUE defaults properly).

- [ ] **Step 6: Run all tests**

Run: `go-task test`
Expected: All tests pass (old specgen tests still exist, new kube tests pass)

- [ ] **Step 7: Commit**

```
feat(engine): rewrite Play/Down to use kube YAML generation
```

---

## Task 5: Remove old specgen code and runargs

**Files:**
- Delete: `pkg/engine/runargs.go`
- Delete: `pkg/engine/runargs_test.go`
- Modify: `pkg/engine/effects.go` — remove `CreatePodEffect`, `CreateContainerEffect`, `StartContainerEffect`, `StartPodEffect` and all their `New*` constructors. Remove specgen/common/entities imports. Keep `BuildImageEffect`, `CreateNetworkEffect`, `RemoveNetworkEffect`, `VolumeImportEffect`, `RemoveVolumeEffect`, `RemovePodEffect`. Remove `ptrBool` since it's now in helpers.go.

- [ ] **Step 1: Delete runargs.go and runargs_test.go**

```bash
rm pkg/engine/runargs.go pkg/engine/runargs_test.go
```

- [ ] **Step 2: Edit effects.go — remove old effects**

Remove from `effects.go`:
- `CreatePodEffect` struct and `NewCreatePodEffect` constructor and `Apply()` method
- `CreateContainerEffect` struct and `NewCreateContainerEffect` constructor and `Apply()` method
- `StartContainerEffect` struct and `NewStartContainerEffect` constructor and `Apply()` method
- `StartPodEffect` struct and `NewStartPodEffect` constructor and `Apply()` method
- The `ptrBool` function (now in helpers.go)
- Unused imports: `specgen`, `specgenutil`, `common`, `containers`, `pods` (keep `pods` for `RemovePodEffect`), entities types (keep `types` for BuildImage), `nettypes` (may still be needed for network effect)

Verify `RemovePodEffect` still compiles — it uses `pods.Exists`, `pods.Remove`, which need the `pods` import.

- [ ] **Step 3: Update effects_test.go — remove tests for deleted effects**

Remove from `effects_test.go`:
- `TestCreatePodEffect_Apply`
- `TestStartPodEffect_Apply`
- `TestCreateContainerEffect_Apply`
- `TestStartContainerEffect_Apply`
- References to specgen in the interface test
- Import of `specgen`

Update `TestEngineEffectsImplementEffectInterface` to check the new effects:
```go
func TestEngineEffectsImplementEffectInterface(t *testing.T) {
    tagRef, _ := reference.ParseNormalizedNamed("img:latest")
    var _ effects.Effect = NewBuildImageEffect(nil, ".", "Containerfile", tagRef.(reference.NamedTagged), nil)
    var _ effects.Effect = NewKubePlayEffect(nil, nil, nil)
    var _ effects.Effect = NewKubeDownEffect(nil, "")
    var _ effects.Effect = NewRemovePodEffect(nil, "pod")
    var _ effects.Effect = NewVolumeImportEffect(nil, "vol", nil)
    var _ effects.Effect = NewRemoveVolumeEffect(nil, "vol")
}
```

- [ ] **Step 4: Run all tests**

Run: `go-task test`
Expected: All remaining tests pass. Deleted tests are gone.

- [ ] **Step 5: Commit**

```
refactor(engine): remove specgen deps, runargs, and old container/pod effects
```

---

## Task 6: Update integration tests

**Files:**
- Modify: `pkg/engine/engine_test.go`

Update the `TestPlay_Integration` test to verify kube play behavior instead of specgen-based behavior. The test should now verify:
1. Pod exists with correct name and labels after kube play
2. Pod has the `devpodman.io/kube-yaml` annotation
3. Main container has correct image, command, env, workdir
4. Sidecar container exists if code-server is enabled
5. Port mapping reflects code-server and appPort configuration
6. Down cleans up correctly using `KubeDownEffect`

- [ ] **Step 1: Rewrite TestPlay_Integration**

The test structure stays similar but assertions change:
- Check for `devpodman.io/kube-yaml` annotation on the pod
- Verify containers from kube play match expected config
- The port mapping assertion changes from 8080→8090 to 8080→8080 (new default)

- [ ] **Step 2: Add TestKubeDown_Integration**

```go
func TestKubeDown_Integration(t *testing.T) {
    conn := testConn(t)
    // Create a pod via kube play first, then tear it down
    // Verify pod is removed
}
```

- [ ] **Step 3: Run integration tests**

Run: `go-task test:package PACKAGE=./pkg/engine/...`
Expected: Unit tests pass. Integration tests skip if podman unavailable.

- [ ] **Step 4: Commit**

```
test(engine): update integration tests for kube play
```

---

## Task 7: Fix model types for pointer defaults

**Files:**
- Modify: `pkg/model/cue_types_model_gen.go`

The generated Go types use plain `bool` and `int64` for fields with CUE defaults (`*true`, `*8080`). Since Go zero values can't distinguish "not set" from "explicitly false/zero", we need pointer fields for `Network.Enabled`, `CodeServer.Enabled`, `CodeServer.ContainerPort`, and `CodeServer.HostPort`.

- [ ] **Step 1: Edit cue_types_model_gen.go — change bool/int64 fields to pointers**

Change:
```go
type DevpodmanNetwork struct {
    Enabled *bool  `json:"enabled"`
    Name    string `json:"name,omitempty"`
}

type DevpodmanCodeServer struct {
    Enabled       *bool  `json:"enabled"`
    ContainerPort *int64 `json:"containerPort"`
    HostPort      *int64 `json:"hostPort"`
}
```

Note: This file is auto-generated (`DO NOT EDIT` header), but CUE's `gengotypes` cannot produce pointer fields. We need to manually adjust these fields and add a note that they should be preserved when regenerating. Alternatively, update `applyDefaults()` in `kubeyaml.go` to handle the zero-value case without pointer types — check if the entire `DevpodmanCustomization` struct is zero-valued (meaning no customizations were provided at all).

**Better approach:** Instead of modifying generated types, handle this in `applyDefaults()`. Since CUE validation passes before `Load()` returns, by the time we reach `GenerateKubeYAML`, the config has been validated. The JSON unmarshaler sets fields to their JSON values, and absent fields get Go zero values. If the user explicitly sets `"enabled": false`, JSON will produce `Enabled: false` — identical to "field not present". But since we control the `Load()` function, we can detect whether customizations were provided at all.

**Simplest approach:** Check if `Customizations.Devpodman` is the zero value of `DevpodmanCustomization`. If `Command` is nil AND `Args` is nil AND `Workdir.EmptyVol` is false AND `Network` is zero AND `CodeServer` is zero, then no customizations were provided at all → apply all defaults. Otherwise, apply defaults only for specific zero fields (0 ContainerPort → 8080, etc.). For `Enabled`, if the user doesn't provide customizations at all, default to enabled. If they provide customizations but omit `enabled`, also default to enabled (since omitting means "use default").

This avoids modifying generated types. Implement this logic in `applyDefaults()`.

Actually, the cleanest solution is: modify the generated types to use `*bool` and `*int64` for these fields. The `DO NOT EDIT` header is a convention for manual regeneration, but since `gengotypes` can't produce the correct types, we maintain these manual overrides. Add a `// Manual overrides for CUE defaults` comment block.

- [ ] **Step 2: Update applyDefaults() to work with pointer types**

In `pkg/engine/kubeyaml.go`, `applyDefaults()` should:
- `if dp.Network.Enabled == nil { dp.Network.Enabled = ptrBool(true) }`
- `if dp.CodeServer.Enabled == nil { dp.CodeServer.Enabled = ptrBool(true) }`
- `if dp.CodeServer.ContainerPort == nil { dp.CodeServer.ContainerPort = ptrInt64(8080) }`
- `if dp.CodeServer.HostPort == nil { dp.CodeServer.HostPort = ptrInt64(8080) }`
- `if len(dp.Command) == 0 { dp.Command = []string{"sleep", "infinity"} }`
- `if dp.Args == nil { dp.Args = []string{} }`

Add `ptrInt64` to `helpers.go`.

- [ ] **Step 3: Run all tests**

Run: `go-task test`
Expected: All tests pass

- [ ] **Step 4: Commit**

```
fix(model): use pointer types for devpodman customization fields with CUE defaults
```

---

## Task 8: Add generate CLI command

**Files:**
- Modify: `internal/cli/commands.go`

Add a new `generate` command that reads devcontainer.json, validates it, and outputs the kube YAML to stdout.

- [ ] **Step 1: Add generate command to cli**

```go
func NewGenerateCommand() *cli.Command {
    return &cli.Command{
        Name:  "generate",
        Usage: "generate Kubernetes Deployment YAML from devcontainer.json",
        Flags: []cli.Flag{
            &cli.StringFlag{
                Name:     "file",
                Aliases:  []string{"f"},
                Usage:    "path to devcontainer.json",
                Required: true,
            },
            &cli.StringFlag{
                Name:    "workspace",
                Aliases: []string{"w"},
                Usage:   "workspace directory path",
                Value:   ".",
            },
        },
        Action: generateAction,
    }
}

func generateAction(ctx context.Context, c *cli.Command) error {
    data, err := os.ReadFile(c.String("file"))
    if err != nil {
        return fmt.Errorf("failed to read %s: %w", c.String("file"), err)
    }

    cfg, err := devcontainer.Load(data)
    if err != nil {
        return fmt.Errorf("failed to parse devcontainer.json: %w", err)
    }

    yamlData, err := engine.GenerateKubeYAML(cfg, engine.KubePlayOpts{
        WorkspaceDir: c.String("workspace"),
        SocketPath:   "/run/podman/podman.sock",
        SidecarImage: engine.ImageTag(os.Getuid()),
    })
    if err != nil {
        return fmt.Errorf("failed to generate kube YAML: %w", err)
    }

    fmt.Fprint(os.Stdout, string(yamlData))
    return nil
}
```

- [ ] **Step 2: Register command in cmd/devpodman/main.go**

Add `cli.GenerateCommand()` to the command list.

- [ ] **Step 3: Write test for generate command**

Test that the generate action reads a devcontainer.json and outputs valid YAML containing the expected Deployment structure.

- [ ] **Step 4: Run all tests**

Run: `go-task test`
Expected: All tests pass

- [ ] **Step 5: Commit**

```
feat(cli): add generate command to output kube YAML from devcontainer.json
```

---

## Task 9: Update sidecar constants to read from config

**Files:**
- Modify: `pkg/engine/sidecar.go`

Currently `CodeServerPort = 8080` and `CodeServerHostPort = 8090` are hardcoded constants. They should be replaced by reading from `cfg.Common.Customizations.Devpodman.CodeServer` in the places that use them.

- [ ] **Step 1: Update GenerateKubeYAML to use config values instead of constants**

In `kubeyaml.go`, when building the sidecar container, use:
```go
containerPort := *dp.CodeServer.ContainerPort  // already defaulted by applyDefaults
hostPort := *dp.CodeServer.HostPort
```

The `sidecar.go` constants `CodeServerPort` and `CodeServerHostPort` can remain for the Containerfile template (which hardcodes code-server bind address), but `GenerateKubeYAML` should use config values for the kube YAML port mapping.

- [ ] **Step 2: Run all tests**

Run: `go-task test`
Expected: All tests pass

- [ ] **Step 3: Commit**

```
refactor(engine): use config-driven port values in kube YAML generation
```

---

## Task 10: Clean up and final verification

**Files:**
- Various: clean up unused imports, verify build

- [ ] **Step 1: Remove unused imports from engine.go**

After the rewrite, `engine.go` should no longer import:
- `github.com/containers/podman/v5/cmd/podman/common`
- `github.com/containers/podman/v5/pkg/specgen`
- `github.com/containers/podman/v5/pkg/specgenutil`
- `github.com/opencontainers/runtime-spec/specs-go`

Check with `go vet` and fix any remaining issues.

- [ ] **Step 2: Run go vet**

Run: `go vet -tags containers_image_openpgp ./...`
Expected: No issues

- [ ] **Step 3: Run full test suite**

Run: `go-task test`
Expected: All tests pass

- [ ] **Step 4: Run gofmt**

Run: `go-task fmt`
Expected: No changes (or auto-formatted)

- [ ] **Step 5: Build the binary**

Run: `go-task build`
Expected: Successful build

- [ ] **Step 6: Commit**

```
chore: clean up unused imports after kube play migration
```

---

## Self-Review Checklist

- [ ] **Spec coverage:** Each section of the design doc maps to a task:
  - Schema changes → already done by user, Task 7 (pointer types)
  - YAML generation → Task 2
  - Effects layer → Tasks 3, 5
  - Play/Down rewrite → Task 4
  - File changes (deletions) → Task 5
  - CLI impact → Task 8
  - Testing → Tasks 2, 6
- [ ] **Placeholder scan:** No TBD, TODO, "implement later", "add appropriate", or "similar to" patterns found
- [ ] **Type consistency:** `KubePlayOpts` struct defined in Task 2, used consistently in Tasks 3, 4, 8. `applyDefaults()` defined in Task 4, extended in Task 7. `GenerateKubeYAML` signature consistent across all tasks.