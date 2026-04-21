---
name: tempts-developer
description: This skill should be used when the user asks to "create a Temporal workflow with tempts", "write a type-safe Temporal activity", "tempts workflow", "tempts activity", "tempts signal", "tempts query", "tempts worker", "tempts Nexus", "type-safe Temporal Go", "tempts schedule", "tempts replay test", "tempts mock test", "migrate to tempts", or mentions the tempts library or type-safe Temporal Go SDK development.
version: 0.1.0
---

# Skill: tempts-developer

## Overview

`tempts` (Temporal Type-Safe) is a Go wrapper library around the Temporal Go SDK that provides compile-time type safety for workflows, activities, signals, queries, schedules, and Nexus operations. It eliminates runtime type mismatches by using Go generics.

This skill provides guidance for building Temporal applications in Go using the `tempts` library.

## Core Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                     Temporal Cluster                            │
│  ┌─────────────────┐  ┌─────────────────┐  ┌────────────────┐  │
│  │  Event History  │  │   Task Queues   │  │   Visibility   │  │
│  │  (Durable Log)  │  │  (Work Router)  │  │   (Search)     │  │
│  └─────────────────┘  └─────────────────┘  └────────────────┘  │
└─────────────────────────────────────────────────────────────────┘
                              ▲
                              │ Poll / Complete
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                    Worker (tempts)                               │
│  ┌─────────────────────────┐  ┌──────────────────────────────┐  │
│  │  Workflow Definitions   │  │  Activity Implementations    │  │
│  │  (Type-safe, Generic)   │  │  (Type-safe, Generic)        │  │
│  └─────────────────────────┘  └──────────────────────────────┘  │
│  ┌─────────────────────────────────────────────────────────────┐│
│  │  tempts.NewWorker validates all registrations at startup    ││
│  └─────────────────────────────────────────────────────────────┘│
└─────────────────────────────────────────────────────────────────┘
```

**tempts enforces at compile time and startup:**
- Workflows and activities are called with correct parameter and return types
- Workflows and activities are routed to the correct queue
- Workers have all required implementations registered (no more, no less)
- Signals are sent and received with matching types
- Queries are called and handled with matching types
- Nexus operations have matching types and complete service implementations

**Components (same as Temporal, but type-safe):**
- **Queues** (`tempts.NewQueue`) - Named task queues; workflows and activities are declared on a queue
- **Workflows** (`tempts.NewWorkflow[Param, Return]`) - Durable, deterministic functions
- **Activities** (`tempts.NewActivity[Param, Return]`) - Non-deterministic operations (API calls, I/O)
- **Workers** (`tempts.NewWorker`) - Validates and runs all registered implementations
- **Signals** (`tempts.NewWorkflowSignal[SignalParam]`) - Type-safe signals scoped to a workflow
- **Queries** (`tempts.NewQueryHandler[Param, Return]`) - Type-safe query handlers
- **Nexus Operations** - Type-safe sync and async Nexus operations on services

## Constraint: All parameter and return types must be Go structs

tempts enforces that all Param and Return type parameters are Go structs (or pointers to structs). Using non-struct types (string, int, etc.) will panic at declaration time. Use a wrapper struct:

```go
// BAD - will panic
var myWorkflow = tempts.NewWorkflow[string, string](queue, "MyWorkflow")

// GOOD
type MyWorkflowParams struct {
    Name string
}
type MyWorkflowResult struct {
    Greeting string
}
var myWorkflow = tempts.NewWorkflow[MyWorkflowParams, MyWorkflowResult](queue, "MyWorkflow")

// For no parameters or return, use struct{}
var myWorkflow = tempts.NewWorkflow[struct{}, struct{}](queue, "MyWorkflow")
```

## History Replay: Why Determinism Matters

Temporal achieves durability through **history replay**. This is unchanged when using tempts -- all standard Temporal determinism rules apply:

1. **Initial Execution** - Worker runs workflow, generates Commands, stored as Events in history
2. **Recovery** - On restart/failure, Worker re-executes workflow from beginning
3. **Matching** - SDK compares generated Commands against stored Events
4. **Restoration** - Uses stored Activity results instead of re-executing

**If Commands don't match Events = Non-determinism Error = Workflow blocked**

See `references/determinism.md` for detailed determinism rules.

## Getting Started

### Ensure Temporal CLI is installed

Check if `temporal` CLI is installed. If not, follow these instructions:

#### macOS

```
brew install temporal
```

#### Linux

Check your machine's architecture and download the appropriate archive:

- [Linux amd64](https://temporal.download/cli/archive/latest?platform=linux&arch=amd64)
- [Linux arm64](https://temporal.download/cli/archive/latest?platform=linux&arch=arm64)

Once you've downloaded the file, extract the downloaded archive and add the temporal binary to your PATH by copying it to a directory like /usr/local/bin

### Read All Relevant References

1. First, read `references/getting-started.md` for the quick start guide
2. Then read references relevant to your task:
   - `references/patterns.md` - Signals, queries, child workflows, parallel execution
   - `references/testing.md` - Unit tests, mock tests, replay tests
   - `references/nexus.md` - Nexus service operations
   - `references/determinism.md` - Determinism rules and safe alternatives
   - `references/migration.md` - Migrating from the standard Go SDK

## Primary References
- **`references/getting-started.md`** - Quick start, file organization, worker setup
- **`references/patterns.md`** - Signals, queries, child workflows, schedules, selectors, saga pattern, continue-as-new, cancellation
- **`references/testing.md`** - Unit testing, mock testing, replay/fixture testing
- **`references/nexus.md`** - Nexus service declarations, sync/async operations, calling from workflows
- **`references/determinism.md`** - Determinism rules, forbidden operations, safe alternatives
- **`references/migration.md`** - Migration guide from the standard Temporal Go SDK to tempts
