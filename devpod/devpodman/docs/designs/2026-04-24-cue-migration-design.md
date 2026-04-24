# Design: Migrate devpodman from Pkl to CUE config backend

**Date:** 2026-04-24

## Summary

Replace PKL as the schema-driven type generation backend with CUE. Individual CUE definitions from `devcontainer.base.schema.cue` are validated independently against input JSON. Go implements a priority system to resolve conflicts (e.g., dockerfile over image). All PKL artifacts, the `pkl-go` dependency, and manual validation functions are removed.

## Decisions

| Decision | Choice |
|---|---|
| Schema scope | Individual defs from `devcontainer.base.schema.cue` (no compose) |
| CUE integration | Generate Go types via `cue exp gengotypes` + CUE Go API at runtime |
| Validation | CUE validates each def independently; Go resolves conflicts via priority |
| Schema location | `devpodman/cue/devcontainer.cue` |
| Mount format | Structured objects `{source, target, type}` |
| Return type | Unified `ResolvedConfig` struct |

## Architecture

```
devpodman/
├── cue/
│   └── devcontainer.cue       # Source of truth — individual CUE defs
├── model/
│   └── devcontainer_gen.go    # Generated Go structs (cue exp gengotypes)
├── devcontainer/
│   ├── devcontainer.go        # Load() — multi-def CUE decode + Go priority
│   └── devcontainer_test.go   # Updated for ResolvedConfig + priority tests
├── cmd/
├── podman/
├── effects/
└── (removed)
    └── pkl/, PklProject, PklProject.deps.json,
        model/*/init.pkl.go, model/*/*.pkl.go, model/*.pkl.go
```

### Data flow

```
devcontainer.json ([]byte)
    │
    ▼
embed.FS ← cue/devcontainer.cue (compiled at build time)
    │
    ▼
For each CUE definition, independently decode:
    ├─ source.LookupDef("#dockerfileContainer").Decode(&DockerfileContainer)
    ├─ source.LookupDef("#imageContainer").Decode(&ImageContainer)
    ├─ source.LookupDef("#devContainerCommon").Decode(&DevContainerCommon)
    └─ source.LookupDef("#nonComposeBase").Decode(&NonComposeBase)
    │
    ▼
Go priority logic:
    │
    ├─ Neither dockerfile nor image decoded → error "must specify image or build"
    ├─ Only dockerfile decoded → use Build path
    ├─ Only image decoded → validateImageReference() → use Image path
    └─ Both decoded → prioritize dockerfile container, warn
    │
    ▼
return (*ResolvedConfig, nil)
```

### ResolvedConfig

```go
type ResolvedConfig struct {
    Build      *DockerfileContainer  // non-nil if build-based (dockerfile) path chosen
    Image      *ImageContainer       // non-nil if image-based path chosen
    Common     *DevContainerCommon   // always populated (may be empty)
    NonCompose *NonComposeBase       // optional additional properties
}
```

### Priority rules (in Go)

| dockerfile matched | image matched | Result |
|---|---|---|
| No | No | Error: "must specify image or build" |
| Yes | No | Use Build path |
| No | Yes | Use Image path (with image ref validation) |
| Yes | Yes | Use Build path (dockerfile prioritized), image silently ignored |

### Schema design (`cue/devcontainer.cue`)

Based on `devcontainer.base.schema.cue` with devpodman extensions:

- `#buildOptions` — unchanged from base (dockerfile, context, target, args, cacheFrom)
- `#devContainerCommon` — base def + `privileged?: bool`, structured `mounts?: [...#Mount]`, `containerEnv?`, `containerUser?`, `runArgs?` (merged from `#nonComposeBase`'s relevant fields)
- `#dockerfileContainer` — unchanged (`build!: #buildOptions`)
- `#imageContainer` — unchanged (`image!: string`)
- `#nonComposeBase` — retained as-is (appPort, workspaceFolder, workspaceMount, shutdownAction, overrideCommand)
- `#Mount` — new devpodman definition: `{source!: string, target!: string, type!: "volume" | "bind"}`
- Path constraints: `dockerfile & =~"^[^/]"`, `context & =~"^[^/]"`, `workspaceFolder & =~"^/"`
- Co-dependency: `(workspaceMount != _|_) == (workspaceFolder != _|_)`

Each definition is validated independently — CUE has no top-level combined constraint. Conflicts are resolved in Go.

### Remaining Go-side validation

Only `validateImageReference()` using `distribution/reference` — CUE has no OCI image reference format checker.

## Error handling

CUE's `Decode()` produces structured errors per-definition. These are collected and filtered: errors from all 4 decodes are only surfaced when none of the container defs (`#dockerfileContainer` or `#imageContainer`) succeed. Wrapped with context e.g. `"failed to decode devcontainer.json: dockerfile: <cue error>, image: <cue error>"`.

## Testing

- Positive-path: same JSON fixtures, assert on `ResolvedConfig` fields
- Error-path: same invalid inputs still fail, CUE error messages replace custom ones
- New tests for priority:
  - `"both image and build provided — dockerfile wins"`
  - `"neither image nor build provided — error"`
  - `"build-only succeeds"`, `"image-only succeeds"`
- New test: `TestSchemaCompiles` — verifies `cue/devcontainer.cue` is valid

## Dependencies

| Remove | Add |
|---|---|
| `github.com/apple/pkl-go v0.13.2` | `cuelang.org/go` (latest) |

## Build system

| Before | After |
|---|---|
| `task pkl:generate` — `pkl run @go/gen.pkl pkl/Schema.pkl` | `task cue:generate` — `cue exp gengotypes cue/devcontainer.cue > model/devcontainer_gen.go` |

`go build`/`.goreleaser.yaml` unchanged — `-tags containers_image_openpgp` remains.

## Files changed

| Action | File |
|---|---|
| New | `devpodman/cue/devcontainer.cue` |
| New | `devpodman/model/devcontainer_gen.go` (generated) |
| Modified | `devpodman/devcontainer/devcontainer.go` |
| Modified | `devpodman/devcontainer/devcontainer_test.go` |
| Modified | `devpodman/cmd/devpodman/debug.go` (adapted to `ResolvedConfig`) |
| Modified | `devpodman/go.mod` |
| Modified | `devpodman/go.sum` |
| Modified | `devpodman/Taskfile.yaml` |
| Modified | `devpodman/AGENTS.md` |
| Removed | `devpodman/pkl/` (entire dir) |
| Removed | `devpodman/PklProject` |
| Removed | `devpodman/PklProject.deps.json` |
| Removed | `devpodman/model/build/`, `model/common/`, `model/image/`, `model/*.pkl.go` |
| Removed | `devpod/docs/2024-03-27-devcontainer-cli-design.md` |
