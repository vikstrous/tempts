# Determinism in tempts Workflows

## Overview

tempts wraps the Temporal Go SDK but does not change the determinism model. All standard Temporal determinism rules apply. The Go SDK has no runtime sandbox -- determinism is the developer's responsibility.

## Why Determinism Matters

Temporal achieves durability through **history replay**. When a Worker needs to restore workflow state (after crash, cache eviction, or a long timer), it re-executes the workflow code from the beginning, comparing generated Commands against stored Events in history. If they don't match, a non-determinism error occurs and the workflow becomes blocked.

```
Initial Execution:
  Code runs → Generates Commands → Server stores as Events

Replay (Recovery):
  Code runs again → Generates Commands → SDK compares to Events
  If match: Use stored results, continue
  If mismatch: NondeterminismError
```

## Forbidden Operations in Workflow Code

Do not use any of the following in workflow functions:

- **Native goroutines** (`go func()`) -- use `workflow.Go()` instead
- **Native channels** (`chan`, send, receive) -- use `workflow.Channel` instead
- **Native `select`** -- use `workflow.Selector` instead
- **`time.Now()`** -- use `workflow.Now(ctx)` instead
- **`time.Sleep()`** -- use `workflow.Sleep(ctx, duration)` instead
- **`math/rand` global** -- use `workflow.SideEffect` instead
- **Map range iteration** (`for k, v := range myMap`) -- sort keys first, then iterate
- **I/O operations** (network, filesystem, database) -- move to activities
- **Reading environment variables** -- move to activities or pass as workflow parameters
- **`fmt.Println`** -- use `workflow.GetLogger(ctx)` for replay-safe logging

## Safe Alternatives

| Instead of | Use |
|---|---|
| `go func() { ... }()` | `workflow.Go(ctx, func(ctx workflow.Context) { ... })` |
| `chan T` | `workflow.NewChannel(ctx)` / `workflow.NewBufferedChannel(ctx, size)` |
| `select { ... }` | `workflow.NewSelector(ctx)` |
| `time.Now()` | `workflow.Now(ctx)` |
| `time.Sleep(d)` | `workflow.Sleep(ctx, d)` |
| `rand.Intn(100)` | `workflow.SideEffect(ctx, func(ctx workflow.Context) interface{} { return rand.Intn(100) })` |
| `log.Println(...)` | `workflow.GetLogger(ctx).Info(...)` |

## What tempts Does NOT Change

- Workflow functions still receive `workflow.Context` (not `context.Context`)
- Activity functions still receive `context.Context`
- All `workflow.*` APIs (Go, Channel, Selector, Sleep, Now, SideEffect, GetLogger) are used directly -- tempts does not wrap these
- The `workflowcheck` static analysis tool works with tempts workflows

## Static Analysis with workflowcheck

The optional `workflowcheck` tool can catch many non-deterministic operations at compile time:

```bash
go install go.temporal.io/sdk/contrib/tools/workflowcheck@latest
workflowcheck ./...
```

## Detecting Non-Determinism

### During Execution
- `NondeterminismError` is raised when Commands don't match Events
- The workflow becomes blocked until code is fixed

### Testing with Replay
Use tempts replay tests to verify backward compatibility. See `references/testing.md` for details.

## Recovery from Non-Determinism

### Accidental Change
1. Revert code to match what's in history
2. Restart worker
3. Workflow auto-recovers

### Intentional Change
1. Use the Temporal Patching API (`workflow.GetVersion`) to support both old and new code paths
2. Or terminate old workflows and start new ones with updated code
