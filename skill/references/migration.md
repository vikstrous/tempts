# Migrating from the Standard Go SDK to tempts

## Prerequisites

Before migrating a queue to tempts:

- **Queue names must be static** -- tempts declares queues as global variables
- **All workflows and activities for a given queue must be migrated at once** -- tempts validates completeness at worker creation
- **All parameter types must be Go structs** -- tempts enforces struct types for all Param and Return generics

## Migration Reference

### Connect to Temporal

**Before:**
```go
c, err := client.Dial(client.Options{})
```

**After:**
```go
c, err := tempts.Dial(client.Options{})
```

### Declare Queues

**Before:**
```go
const taskQueue = "main"
```

**After:**
```go
var queueMain = tempts.NewQueue("main")
```

### Declare and Run Workflows

**Before:**
```go
// Start workflow
var ret ReturnType
run, err := c.ExecuteWorkflow(ctx, client.StartWorkflowOptions{
    TaskQueue: "main",
}, "MyWorkflow", param)
err = run.Get(ctx, &ret)
```

**After:**
```go
// Declare (once, as a package-level variable)
var myWorkflowType = tempts.NewWorkflow[MyParam, MyReturn](queueMain, "MyWorkflow")

// Start workflow (queue is set automatically)
ret, err := myWorkflowType.Run(ctx, c, client.StartWorkflowOptions{}, MyParam{Name: "test"})
```

### Declare and Run Activities

**Before:**
```go
// In workflow
var ret ReturnType
err := workflow.ExecuteActivity(ctx, "MyActivity", param).Get(ctx, &ret)
```

**After:**
```go
// Declare (once, as a package-level variable)
var myActivityType = tempts.NewActivity[MyParam, MyReturn](queueMain, "MyActivity")

// In workflow
ret, err := myActivityType.Run(ctx, MyParam{Name: "test"})
```

### Create a Worker

**Before:**
```go
w := worker.New(c, "main", worker.Options{})
w.RegisterWorkflow(MyWorkflow)
w.RegisterActivity(&Activities{})
err = w.Run(worker.InterruptCh())
```

**After:**
```go
wrk, err := tempts.NewWorker(queueMain, []tempts.Registerable{
    myWorkflowType.WithImplementation(myWorkflowFn),
    myActivityType.WithImplementation(myActivityFn),
})
if err != nil {
    panic(err) // validation failure
}
err = wrk.Run(ctx, c, worker.Options{})
```

### Queries

**Before:**
```go
// In workflow
workflow.SetQueryHandler(ctx, "get-status", func() (string, error) {
    return status, nil
})

// Querying
response, err := c.QueryWorkflow(ctx, workflowID, runID, "get-status")
var value string
err = response.Get(&value)
```

**After:**
```go
// Declare
var queryGetStatus = tempts.NewQueryHandler[struct{}, StatusResult]("get-status")

// In workflow
queryGetStatus.SetHandler(ctx, func(_ struct{}) (StatusResult, error) {
    return StatusResult{Status: status}, nil
})

// Querying (typed result)
result, err := queryGetStatus.Query(ctx, c, workflowID, runID, struct{}{})
```

### Signals

**Before:**
```go
// In workflow
ch := workflow.GetSignalChannel(ctx, "my-signal")
var param SignalParam
ch.Receive(ctx, &param)

// Sending
err := c.SignalWorkflow(ctx, workflowID, runID, "my-signal", param)
```

**After:**
```go
// Declare (scoped to a workflow)
var mySignal = tempts.NewWorkflowSignal[SignalParam](&myWorkflowType, "my-signal")

// In workflow
param := mySignal.Receive(ctx)

// Sending (typed)
err := mySignal.Signal(ctx, c, workflowID, runID, SignalParam{Message: "hello"})
```

### Child Workflows

**Before:**
```go
var ret ReturnType
err := workflow.ExecuteChildWorkflow(ctx, "ChildWorkflow", param).Get(ctx, &ret)
```

**After:**
```go
ret, err := childWorkflowType.RunChild(ctx, workflow.ChildWorkflowOptions{}, ChildParam{})
```

### Schedules

**Before:**
```go
_, err = c.ScheduleClient().Create(ctx, client.ScheduleOptions{
    ID: "my-schedule",
    Spec: client.ScheduleSpec{...},
    Action: &client.ScheduleWorkflowAction{
        ID:        "workflow-id",
        Workflow:  "MyWorkflow",
        TaskQueue: "main",
        Args:      []any{param},
    },
})
```

**After:**
```go
err = myWorkflowType.SetSchedule(ctx, c, client.ScheduleOptions{
    ID: "my-schedule",
    Spec: client.ScheduleSpec{...},
}, MyParam{Name: "test"})
```

`SetSchedule` creates the schedule if it doesn't exist, or updates it to match.

### SignalWithStart

**Before:**
```go
_, err := c.SignalWithStartWorkflow(ctx, workflowID, "my-signal", signalParam, opts, "MyWorkflow", workflowParam)
```

**After:**
```go
run, err := mySignal.SignalWithStart(ctx, c, opts, workflowParam, signalParam)
```

Both the workflow parameter type and signal parameter type are checked at compile time.

## Positional Arguments (Legacy Migration)

If existing workflows or activities use multiple positional arguments instead of a single struct, use `NewWorkflowPositional` / `NewActivityPositional` for backward compatibility:

```go
// Existing workflow that takes (string, int) as positional args
var legacyWorkflow = tempts.NewWorkflowPositional[LegacyParams, LegacyResult](queueMain, "LegacyWorkflow")

type LegacyParams struct {
    Name  string
    Count int
}
```

The struct fields are passed as positional arguments in the order they are defined. This maintains wire compatibility with existing workflow histories.

Do not use `NewWorkflowPositional` / `NewActivityPositional` in new code.
