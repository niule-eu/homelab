# KB for Agents

## Personality

* You do not provide expansive answers unless explicitly asked for.
* You do not "sugarcoat" uncomfortable truths or opinions you think might antagonize others.

## PKL Basics

**Comments:** `//` single-line, `/* */` multi-line
**Syntax:**
- Variables: `hidden varName = "value"`
- Objects: `new { key = "value" }`
- Lists: `new Listing { "item1", "item2" }`
- Interpolation: `"Hello ${name}!"`

### PKL Rendering Workflow

**Two-level system:** Kustomization PKL → Final manifests

**Standard Rendering:**
```bash
hlcli render-pkl -p /absolute/path/to/project kustomization.pkl -f
```

**Manual Rendering (debugging):**
```bash
hlcli render-pkl -p /absolute/path/to/project controller.pkl -f
```

**Requirements:**
- `-p` flag MUST use absolute path to PklProject directory
- Relative paths (`.`, `./`) will fail
- PklProject must be accessible for dependencies

**Process:**
1. Kustomization.pkl imports all `*.pkl` files
2. Each file generates YAML outputs
3. Final `kustomization.yaml` references all manifests

## Commit Messages

**Style:** Functional, concise, present tense, objective. Use [conventional commits](https://www.conventionalcommits.org/en/v1.0.0/).

**Focus on:** Purpose ("why") over implementation ("what changed")

**Structure**: 
* Concise header with coarse outline on commit effect
* Body explaining why this change is necessary

**Good:** "Limit parallelism to prevent resource exhaustion"
**Avoid:** "Changed parallelism from 1 to 2 in argo-workflows.pkl"

