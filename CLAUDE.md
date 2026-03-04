# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build & Run Commands

```bash
# Go is installed at C:\Users\Corey\go (zip install)
export PATH="/c/Users/Corey/go/bin:$PATH"

# Build
go build ./...

# Run
go run ./cmd/segmentation -config config/segments.json -addr :8080

# Run all tests
go test ./...

# Run tests for a specific package
go test ./internal/domain/strategy/...
go test ./internal/domain/engine/...

# Run a single test by name
go test ./internal/domain/strategy/ -run TestRuleStrategy_NestedAndOr

# Verbose tests with coverage
go test -v -cover ./...
```

## Architecture

This is a **Domain-Driven Design** Go service with strict dependency rules: domain → application → infrastructure.

### Dependency Flow

```
cmd/segmentation/main.go         (composition root — wires everything)
       ↓
internal/application/            (use cases: evaluate, batch, reload)
       ↓
internal/domain/                 (zero external dependencies)
       ↑
internal/infrastructure/         (implements domain ports)
```

### Domain Ports (Key Interfaces)

Three interfaces in `internal/domain/ports/` define the architecture boundaries:

- **SegmentStore** — `Get() *Snapshot` / `Swap(*Snapshot)` — lock-free reads via `atomic.Pointer`
- **ConfigSource** — `Load() (*Snapshot, error)` — loads config from external source
- **Hasher** — `Bucket(subjectKey, salt string) int` — deterministic hash bucketing [0, 100)

### Evaluation Flow

1. **Evaluator** (`internal/domain/engine/evaluator.go`) iterates layers sorted by `order`
2. For each layer's segments: check promotion time window → check overrides → evaluate primary strategy
3. After each layer resolves, its result is injected into context as `"layer:<name>"` for cross-layer dependencies
4. Three strategies: `StaticStrategy` (map lookup), `RuleStrategy` (recursive AND/OR tree), `PercentageStrategy` (FNV-1a hash)

### Composite Rule Tree

Rules follow a recursive tree structure (`internal/domain/model/rule.go`). A **leaf** has an `Expression` (field/operator/value). A **composite** has an `Operator` (And/Or) and nested `Rules`. Short-circuit evaluation is used. The `evaluateRule` function in `internal/domain/strategy/rule.go` handles the recursion.

### Config Hot-Reload

`internal/infrastructure/config/watcher.go` polls the config file every 500ms. On change, it loads via `ConfigSource`, validates via `validation.ValidateSnapshot`, then atomically swaps into the `SegmentStore`.

## API Endpoints

| Method | Path | Handler |
|--------|------|---------|
| POST | `/v1/evaluate` | Single-user evaluation |
| POST | `/v1/evaluate/batch` | Multi-user parallel evaluation |
| GET | `/v1/segments` | List config (admin) |
| GET | `/v1/health` | Health check |
| POST | `/v1/reload` | Force config reload |

## Config Format

`config/segments.json` defines layers (ordered), each containing segments with a strategy (`static`, `rule`, or `percentage`), optional `overrides`, `promotion` time bounds, and `inputSchema` for validation.
