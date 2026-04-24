# devpodman Pod Lifecycle Commands

## Summary

Add `generate`, `up`, and `down` commands to devpodman CLI to create and manage podman pods from devcontainer.json specifications.

## Motivation

devpodman currently only has a `debug` command for validating devcontainer.json files. Users need the ability to actually create and manage podman pods based on their devcontainer specifications.

## Design

### Commands

| Command | Purpose |
|---------|---------|
| `devpodman generate <path>` | Output Kubernetes-style YAML pod manifest to stdout |
| `devpodman up <path>` | Create and start a podman pod from devcontainer.json |
| `devpodman down <pod-name>` | Stop and remove a podman pod |

### Architecture

```
devpodman/
├── cmd/devpodman/
│   ├── main.go          # existing
│   ├── debug.go         # existing
│   ├── generate.go      # NEW
│   ├── up.go            # NEW
│   └── down.go          # NEW
├── podman/
│   ├── client.go        # existing
│   ├── config.go        # existing
│   ├── pods.go          # NEW - CreatePod, RemovePod, InspectPod, ListPods
│   └── build.go         # NEW - BuildImage, ImageExists
├── generate/
│   └── yaml.go          # NEW - PodYAML generation
└── model/               # existing - Pkl-generated types
```

### Command Flows

#### `devpodman generate <path-to-devcontainer.json>`

1. Read and parse devcontainer.json via `devcontainer.Load()`
2. Derive pod name from `name` property or directory name
3. Generate Kubernetes-style YAML manifest
4. Write YAML to stdout

#### `devpodman up <path-to-devcontainer.json>`

1. Read and parse devcontainer.json via `devcontainer.Load()`
2. Derive pod name from `name` property or directory name
3. Check if pod exists → error if yes (requires explicit `down` first)
4. If build case:
   - Compute image tag: `devpodman-<hash>` where hash = SHA256(path + content)
   - Check if image exists → skip build if present
   - Build image if needed
5. Create pod with derived name
6. Create container in pod with:
   - Image (built or from `image` property)
   - Mounts (workspace + additional)
   - Environment variables
   - Security settings (`privileged`, `runArgs`)
   - User (`remoteUser` > `containerUser`)
7. Start container
8. Output pod name and container ID

#### `devpodman down <pod-name>`

1. Stop pod
2. Remove pod (removes containers)
3. If `--delete-images`: remove images built by devpodman
4. If `--delete-volumes`: remove named volumes
5. Output confirmation

### Property Mapping

| devcontainer.json Property | podman Mapping |
|---------------------------|----------------|
| `name` | Pod name |
| `image` | Container image spec |
| `build.dockerfile` | Build context, image tag from hash |
| `build.context` | Build context directory |
| `build.args` | Build args (`--build-arg`) |
| `build.target` | Build target (`--target`) |
| `workspaceMount` | Container mount spec |
| `workspaceFolder` | Container working directory |
| `mounts[]` | Additional container mounts |
| `containerEnv` | Container environment variables |
| `remoteEnv` | Container environment variables (merged) |
| `containerUser` | `--user` flag |
| `remoteUser` | `--user` flag (takes precedence) |
| `privileged` | `--privileged` flag |
| `runArgs[]` | Passed through to container create |

### YAML Output Format

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: <pod-name>
  labels:
    devpodman/managed: "true"
    devpodman/config-hash: <sha256>
spec:
  containers:
    - name: main
      image: <image-ref>
      workingDir: <workspaceFolder or /workspace>
      env:
        - name: KEY
          value: VALUE
      securityContext:
        privileged: <bool>
        runAsUser: <uid>
      volumeMounts:
        - name: workspace
          mountPath: /workspace
      command: ["/bin/sh", "-c", "while sleep 1000; do :; done"]
  volumes:
    - name: workspace
      hostPath:
        path: /path/to/source
        type: Directory
```

### Error Handling

| Scenario | Error Message | Exit Code |
|----------|---------------|-----------|
| devcontainer.json not found | `devcontainer.json not found: <path>` | 1 |
| Invalid JSON | `failed to parse devcontainer.json: <error>` | 1 |
| Missing required field | `devcontainer.json must specify either 'image' or 'build'` | 1 |
| Pod already exists | `pod '<name>' already exists, run 'devpodman down <name>' first` | 1 |
| Build failure | `failed to build image: <error>` | 1 |
| Image pull failure | `failed to pull image '<ref>': <error>` | 1 |
| Podman socket not accessible | `podman socket not accessible: <path>` | 1 |

### Build Caching

- Image tag: `devpodman-<hash>` where hash = SHA256(devcontainer.json path + content)
- Before building, check if image with tag exists
- Skip build if image exists (assumes unchanged config)

### Idempotency

- `generate` is always idempotent (pure output)
- `up` fails if pod exists (requires explicit `down`)
- `down` succeeds if pod doesn't exist (no-op with warning)

### Labels

Pods created by devpodman are labeled:
- `devpodman/managed=true` - identifies pods managed by devpodman
- `devpodman/config-hash=<hash>` - enables future cache validation

## Implementation Notes

### User Handling

- Pass `--user` to podman based on `remoteUser` (preferred) or `containerUser`
- Do not implement UID mapping for initial version

### Workspace Handling

- No variable expansion (e.g., `${localWorkspaceFolder}`)
- Use `workspaceMount` and `workspaceFolder` directly from file
- Validate that both are specified if either is present

### Mount Types

- `type: "bind"` → host bind mount
- `type: "volume"` → named volume

## Testing

### Unit Tests

- `devcontainer.Load()` edge cases
- YAML output structure validation
- Pod name derivation logic

### Integration Tests

- `up` with image-based devcontainer
- `up` with build-based devcontainer
- `up` error on existing pod
- `down` cleanup
- `down --delete-images`
- `down --delete-volumes`

### Test Fixtures

- `testdata/image-based.json`
- `testdata/build-based.json`
- `testdata/with-mounts.json`
- `testdata/invalid.json`

## Out of Scope

- Variable expansion (e.g., `${localWorkspaceFolder}`)
- UID/GID mapping
- Multi-container pods (user-facing)
- Docker Compose support
- Lifecycle scripts (`onCreateCommand`, etc.)
- Port forwarding
- `forwardPorts` support

## Future Considerations

- Port forwarding (`forwardPorts`)
- Lifecycle script execution
- Multi-container pod support
- Config hot-reload