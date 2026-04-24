# Embedding Pkl in Go Binary

## Concept
Instead of parsing/validating `devcontainer.json` in Go, embed a Pkl schema and let it produce a typed Pkl representation of the JSON input.

## Implementation Sketch

```go
//go:embed schema.pkl
var schemaFS embed.FS

evaluator, err := pkl.NewEvaluator(ctx, pkl.PreconfiguredOptions,
    pkl.WithModuleReaders(map[string]pkl.ModuleReader{
        "embed:": pkl.NewFsModuleReader(schemaFS),
    }))
// Evaluate "embed:schema.pkl" with devcontainer.json as input
```

## Critical Caveat
`pkl-go` is a **client**, not an interpreter. It spawns the `pkl` CLI binary and communicates over stdio.

**Consequences:**
- Binary still requires `pkl` installed on the host at runtime
- If `pkl` not in `$PATH`, CLI fails
- Cannot distribute a single, zero-dependency binary

## Verdict
- **Current Go parsing approach** → self-contained binary, works anywhere
- **Embedded Pkl** → only viable if we control deployment or require `pkl` as a dependency

## Status
Deferred. Revisit after core features are implemented.
