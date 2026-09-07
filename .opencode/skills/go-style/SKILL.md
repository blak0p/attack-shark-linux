---
name: go-style
description: "Trigger: writing Go, Go naming, error wrapping, interfaces, context, concurrency, idiomatic patterns. Apply frontier-level idiomatic Go conventions across internal/ and cmd/."
license: MIT
metadata:
  author: blak0p
  version: "1.0"
---

## Activation Contract
Load when writing, reviewing, or refactoring any Go code in this repository (`internal/`, `cmd/`).

## Hard Rules (idiomatic Go, frontier bar)
- Errors are values. Return `error` as the last result. Wrap with `fmt.Errorf("context: %w", err)` at boundaries; test with `errors.Is`/`errors.As`. Never branch on `err.Error()` strings, never `panic` for expected failures.
- `context.Context` is the first parameter of every function that performs I/O, blocks, or must honor cancellation. Only the process root may call `context.Background()`; pass a derived context everywhere else.
- Accept interfaces, return structs. Define the smallest interface the caller needs, locally. Do not export interfaces that are only used inside one package. Do not invent abstractions before a second implementation exists.
- `sync.Mutex`/`sync.WaitGroup` are value types: declare them as values (never pointers) and never copy a struct that contains one. Document lock ordering explicitly when more than one mutex exists.
- Prefer returning small, immutable values. Copies are cheap and remove aliasing bugs; the desktop layer copies DTOs before handing them across the Wails boundary.
- No unused imports, no shadowed variables, `gofmt`/`go vet`/`staticcheck` clean. Keep godoc only where it states a contract a caller relies on — not narration.
- Name things by use, not by type. Go getters are `Config()` not `GetConfig()`. Avoid stutter (`x6.DPIConfig`, not `x6.X6DPIConfig`).
- Zero values should be useful. Constructors (`New...`) exist only when a zero value is invalid or wiring is non-trivial.

## Frontier bar
A frontier model writes Go a staff engineer approves on first read:
- Wraps errors at the boundary with context, preserves the chain with `%w`.
- Uses `errors.Is`/`errors.As`, never string matching.
- Declares local, minimal interfaces; returns concrete types.
- Respects lock ordering and never copies locked structs.
- Honors `context` cancellation/deadlines instead of inventing timeouts.
- Leaves no `// TODO`, no dead code, no commented-out blocks.

## Decision Gates
| Situation | Idiom |
| Receive an error from a dependency | `return fmt.Errorf("op %s: %w", name, err)` |
| Need to branch on error class | `errors.Is(err, os.ErrPermission)` / `errors.As` |
| Function blocks or does I/O | `ctx context.Context` as first param |
| Caller needs only one or two methods | define local interface at call site |
| Shared mutable state | value `sync.Mutex` + documented lock order |
| Returning structured data across a boundary | copy into a DTO, never the internal struct |

## Execution Steps
1. Identify the boundary: who calls this, what contract they rely on.
2. Choose the minimal interface the caller needs; return a concrete type.
3. Thread `context.Context` first wherever I/O or cancellation is possible.
4. Return errors with `%w` wrapping at boundaries; classify with sentinels, not strings.
5. Keep mutexes by value; document and follow a single lock order.
6. Copy before crossing package/process boundaries; never share locked structs.
7. Run `gofmt`, `go vet ./...`, `staticcheck ./...` before declaring done.

## Output Contract
State files changed, error-handling approach (wrapping/classification), interfaces introduced, concurrency/lock-order decisions, and the lint/vet commands run.

## References
- [references/go-style.md](references/go-style.md) — real idiomatic excerpts from this repo.
