# Getting Started with tempts

## Overview

`tempts` wraps the Temporal Go SDK to provide compile-time type safety via Go generics. All workflows, activities, signals, queries, and Nexus operations are declared as typed global variables, and `tempts.NewWorker` validates that all implementations are correctly registered at startup.

## Quick Start

**Add Dependency:**
```bash
go get github.com/vikstrous/tempts@latest
```

**Complete working example:**

```go
package main

import (
    "context"
    "fmt"

    "github.com/vikstrous/tempts"
    "go.temporal.io/sdk/client"
    "go.temporal.io/sdk/worker"
    "go.temporal.io/sdk/workflow"
)

// Declare a task queue
var queueMain = tempts.NewQueue("main")

// Declare a workflow with typed parameters and return
var workflowTypeHello = tempts.NewWorkflow[struct{}, struct{}](queueMain, "HelloWorkflow")

// Declare an activity with typed parameters and return
var activityTypeHello = tempts.NewActivity[struct{}, struct{}](queueMain, "HelloActivity")

func main() {
    c, err := tempts.Dial(client.Options{})
    if err != nil {
        panic(err)
    }
    defer c.Close()

    // Create worker -- validates all implementations match declarations
    wrk, err := tempts.NewWorker(queueMain, []tempts.Registerable{
        workflowTypeHello.WithImplementation(helloWorkflow),
        activityTypeHello.WithImplementation(helloActivity),
    })
    if err != nil {
        panic(err)
    }

    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()

    go func() {
        err = wrk.Run(ctx, c, worker.Options{})
        if err != nil {
            panic(err)
        }
    }()

    // Execute the workflow (type-safe: params and return are checked at compile time)
    _, err = workflowTypeHello.Run(ctx, c, client.StartWorkflowOptions{}, struct{}{})
    if err != nil {
        panic(err)
    }

    fmt.Println("Workflow completed.")
}

func helloWorkflow(ctx workflow.Context, _ struct{}) (struct{}, error) {
    return activityTypeHello.Run(ctx, struct{}{})
}

func helloActivity(ctx context.Context, _ struct{}) (struct{}, error) {
    fmt.Println("Hello, Temporal!")
    return struct{}{}, nil
}
```

**Start the dev server:** `temporal server start-dev`

**Run the example:** `go run .`

## Key Concepts

### Declaration Pattern

tempts uses a **declare-then-implement** pattern. Types are declared as package-level variables, and implementations are attached when creating a worker.

```go
// 1. Declare types as package-level variables (shared across packages)
var queueMain = tempts.NewQueue("main")

type GreetParams struct {
    Name string
}
type GreetResult struct {
    Message string
}

var workflowTypeGreet = tempts.NewWorkflow[GreetParams, GreetResult](queueMain, "Greet")
var activityTypeGreet = tempts.NewActivity[GreetParams, GreetResult](queueMain, "GreetActivity")

// 2. Implement the functions (matching the declared signatures)
func greetWorkflow(ctx workflow.Context, params GreetParams) (GreetResult, error) {
    return activityTypeGreet.Run(ctx, params)
}

func greetActivity(ctx context.Context, params GreetParams) (GreetResult, error) {
    return GreetResult{Message: "Hello, " + params.Name}, nil
}

// 3. Register implementations in a worker
wrk, err := tempts.NewWorker(queueMain, []tempts.Registerable{
    workflowTypeGreet.WithImplementation(greetWorkflow),
    activityTypeGreet.WithImplementation(greetActivity),
})
```

### Client

tempts wraps the Temporal SDK client:

```go
// Connect to Temporal server
c, err := tempts.Dial(client.Options{})

// Lazy connection (defers until first use)
c, err := tempts.NewLazyClient(client.Options{})

// Wrap an existing SDK client
c, err := tempts.NewFromSDK(existingClient, "default")

defer c.Close()
```

### Workflow Execution

```go
// Synchronous -- blocks until workflow completes
result, err := workflowTypeGreet.Run(ctx, c, client.StartWorkflowOptions{}, GreetParams{Name: "World"})

// Asynchronous -- returns a WorkflowRun handle
run, err := workflowTypeGreet.Execute(ctx, c, client.StartWorkflowOptions{}, GreetParams{Name: "World"})
// ... do other work ...
var result GreetResult
err = run.Get(ctx, &result)
```

The queue is set automatically from the workflow declaration. `client.StartWorkflowOptions.TaskQueue` does not need to be set.

### Activity Execution (from within a workflow)

```go
// Synchronous
result, err := activityTypeGreet.Run(ctx, GreetParams{Name: "World"})

// Asynchronous -- returns a workflow.Future
future := activityTypeGreet.Execute(ctx, GreetParams{Name: "World"})
var result GreetResult
err := future.Get(ctx, &result)

// Append to a futures slice for parallel fan-out
var futures []workflow.Future
activityTypeGreet.AppendFuture(ctx, &futures, GreetParams{Name: "Alice"})
activityTypeGreet.AppendFuture(ctx, &futures, GreetParams{Name: "Bob"})
for _, f := range futures {
    var r GreetResult
    err := f.Get(ctx, &r)
    // handle result
}
```

The queue is set automatically. Activity options (including timeouts) are inherited from the workflow context. tempts sets a default `StartToCloseTimeout` of 10 seconds via the workflow's `WithImplementation` wrapper. Override by calling `workflow.WithActivityOptions` before executing activities:

```go
func myWorkflow(ctx workflow.Context, params MyParams) (MyResult, error) {
    ctx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
        StartToCloseTimeout: 5 * time.Minute,
    })
    return myActivity.Run(ctx, params)
}
```

### Worker

```go
wrk, err := tempts.NewWorker(queueMain, []tempts.Registerable{
    workflowTypeGreet.WithImplementation(greetWorkflow),
    activityTypeGreet.WithImplementation(greetActivity),
})
if err != nil {
    // Validation failed: missing implementation, extra implementation,
    // wrong queue, duplicate name, etc.
    panic(err)
}

// Run blocks until context is cancelled
err = wrk.Run(ctx, c, worker.Options{})
```

`NewWorker` validates:
- Every workflow/activity declared on the queue has an implementation
- No extra implementations are provided that aren't declared on the queue
- No duplicate names
- All Nexus operations for a service are complete

## File Organization

```
myapp/
├── types/
│   └── types.go           # Queue, workflow, activity, signal declarations (shared)
├── workflows/
│   └── greeting.go        # Workflow implementations
├── activities/
│   └── greet.go           # Activity implementations
├── worker/
│   └── main.go            # Worker setup, registers implementations
└── starter/
    └── main.go            # Client code to start workflows
```

**types/types.go** -- shared type declarations:
```go
package types

import "github.com/vikstrous/tempts"

var QueueMain = tempts.NewQueue("main")

type GreetParams struct {
    Name string
}
type GreetResult struct {
    Message string
}

var WorkflowTypeGreet = tempts.NewWorkflow[GreetParams, GreetResult](QueueMain, "Greet")
var ActivityTypeGreet = tempts.NewActivity[GreetParams, GreetResult](QueueMain, "GreetActivity")
```

**worker/main.go** -- worker setup:
```go
package main

import (
    "context"
    "log"

    "yourmodule/activities"
    "yourmodule/types"
    "yourmodule/workflows"

    "github.com/vikstrous/tempts"
    "go.temporal.io/sdk/client"
    "go.temporal.io/sdk/worker"
)

func main() {
    c, err := tempts.Dial(client.Options{})
    if err != nil {
        log.Fatal(err)
    }
    defer c.Close()

    wrk, err := tempts.NewWorker(types.QueueMain, []tempts.Registerable{
        types.WorkflowTypeGreet.WithImplementation(workflows.GreetWorkflow),
        types.ActivityTypeGreet.WithImplementation(activities.GreetActivity),
    })
    if err != nil {
        log.Fatal(err)
    }

    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()

    err = wrk.Run(ctx, c, worker.Options{})
    if err != nil {
        log.Fatal(err)
    }
}
```

## Common Pitfalls

1. **Using non-struct types as parameters** -- All Param and Return type parameters must be Go structs. `tempts.NewWorkflow[string, string](...)` will panic. Wrap in a struct.
2. **Forgetting to register an implementation** -- `NewWorker` returns an error if any declared workflow/activity on the queue is missing an implementation.
3. **Registering an extra implementation** -- `NewWorker` returns an error if you provide an implementation for something not declared on the queue.
4. **Using native goroutines/channels/select in workflows** -- Use `workflow.Go()`, `workflow.Channel`, `workflow.Selector` (same as standard Temporal Go SDK).
5. **Using `time.Sleep` or `time.Now` in workflows** -- Use `workflow.Sleep()` and `workflow.Now()`.
6. **Not setting activity timeouts** -- tempts sets a default 10-second `StartToCloseTimeout`. Override with `workflow.WithActivityOptions` for longer activities.
