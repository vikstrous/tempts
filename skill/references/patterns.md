# tempts Patterns

## Signals

Signals in tempts are scoped to a specific workflow via `NewWorkflowSignal`. This enforces that SignalWithStart operations use matching workflow and signal types.

### Declaring Signals

```go
type UpdateSuffixParams struct {
    Suffix string
}

// Signal is scoped to workflowTypeFormatAndGreet
var signalUpdateSuffix = tempts.NewWorkflowSignal[UpdateSuffixParams](
    &workflowTypeFormatAndGreet, "update_suffix",
)
```

### Receiving Signals in a Workflow

**Blocking receive (single signal):**
```go
func myWorkflow(ctx workflow.Context, params MyParams) (MyResult, error) {
    // Blocks until signal is received
    signalParams := signalUpdateSuffix.Receive(ctx)
    // use signalParams.Suffix
}
```

**Non-blocking receive:**
```go
if params, ok := signalUpdateSuffix.TryReceive(ctx); ok {
    // Signal was received
    suffix = params.Suffix
}
```

**Multiple signals with Selector:**
```go
selector := workflow.NewSelector(ctx)

signalUpdateSuffix.AddToSelector(ctx, selector, func(params UpdateSuffixParams) {
    suffix = params.Suffix
})
otherSignal.AddToSelector(ctx, selector, func(params OtherParams) {
    // handle other signal
})

selector.Select(ctx) // blocks until one signal fires
```

### Sending Signals from External Code

```go
err := signalUpdateSuffix.Signal(ctx, c, workflowID, runID, UpdateSuffixParams{Suffix: "!"})
```

### SignalWithStart

Atomically starts the workflow if it doesn't exist, or signals it if it does. Both the workflow parameter type and signal parameter type are checked at compile time.

```go
run, err := signalUpdateSuffix.SignalWithStart(
    ctx, c,
    client.StartWorkflowOptions{ID: "my-workflow-id"},
    FormatAndGreetParams{Name: "Viktor"},   // workflow param (type-checked)
    UpdateSuffixParams{Suffix: " (started)"}, // signal param (type-checked)
)
```

## Queries

Queries allow read-only inspection of workflow state. There is no compile-time enforcement that a query is registered on a specific workflow -- the name and types must match by convention.

### Declaring and Using Queries

```go
type GetNameResult struct {
    Name string
}

var queryGetName = tempts.NewQueryHandler[struct{}, GetNameResult]("get_formatted_name")
```

**Setting the handler inside a workflow:**
```go
func myWorkflow(ctx workflow.Context, params MyParams) (MyResult, error) {
    currentName := "unknown"

    queryGetName.SetHandler(ctx, func(_ struct{}) (GetNameResult, error) {
        return GetNameResult{Name: currentName}, nil
    })

    // ... workflow logic that updates currentName ...
}
```

**Querying from external code:**
```go
result, err := queryGetName.Query(ctx, c, workflowID, runID, struct{}{})
fmt.Println(result.Name)
```

## Child Workflows

```go
// Synchronous -- blocks until child completes
result, err := workflowTypeJustGreet.RunChild(
    ctx,
    workflow.ChildWorkflowOptions{},
    JustGreetParams{Name: "World"},
)

// Asynchronous -- returns a ChildWorkflowFuture
future := workflowTypeJustGreet.ExecuteChild(
    ctx,
    workflow.ChildWorkflowOptions{},
    JustGreetParams{Name: "World"},
)
var result JustGreetResult
err := future.Get(ctx, &result)
```

The queue is set automatically from the child workflow's declaration.

### Child Workflow Options

```go
import enumspb "go.temporal.io/api/enums/v1"

result, err := childWorkflowType.RunChild(ctx, workflow.ChildWorkflowOptions{
    WorkflowID:               "child-" + workflow.GetInfo(ctx).WorkflowExecution.ID,
    ParentClosePolicy:        enumspb.PARENT_CLOSE_POLICY_ABANDON,
    WorkflowExecutionTimeout: 10 * time.Minute,
}, params)
```

## Schedules

tempts provides `SetSchedule`, which creates the schedule if it doesn't exist or updates it if it does.

```go
err = workflowTypeFormatAndGreet.SetSchedule(ctx, c, client.ScheduleOptions{
    ID: "every5s",
    Spec: client.ScheduleSpec{
        Intervals: []client.ScheduleIntervalSpec{
            {
                Every: time.Second * 5,
            },
        },
    },
}, FormatAndGreetParams{Name: "Viktor"})
```

The queue and workflow name are set automatically. The parameter type is checked at compile time.

## Parallel Execution

Use `AppendFuture` to fan out multiple activities, then collect results:

```go
func parallelWorkflow(ctx workflow.Context, params ParallelParams) (ParallelResult, error) {
    ctx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
        StartToCloseTimeout: 5 * time.Minute,
    })

    var futures []workflow.Future
    for _, item := range params.Items {
        activityTypeProcess.AppendFuture(ctx, &futures, ProcessParams{Item: item})
    }

    var results []string
    for _, f := range futures {
        var r ProcessResult
        if err := f.Get(ctx, &r); err != nil {
            return ParallelResult{}, err
        }
        results = append(results, r.Output)
    }
    return ParallelResult{Outputs: results}, nil
}
```

Or use `Execute` directly:

```go
futures := make([]workflow.Future, len(items))
for i, item := range items {
    futures[i] = activityTypeProcess.Execute(ctx, ProcessParams{Item: item})
}
```

## Selector Pattern

`workflow.Selector` replaces Go's native `select`. Use it to wait on multiple channels, futures, and timers. Signals integrate via `AddToSelector`:

```go
func approvalWorkflow(ctx workflow.Context, _ struct{}) (ApprovalResult, error) {
    var outcome string

    timerCtx, cancelTimer := workflow.WithCancel(ctx)
    timer := workflow.NewTimer(timerCtx, 24*time.Hour)

    selector := workflow.NewSelector(ctx)

    approveSignal.AddToSelector(ctx, selector, func(params ApproveParams) {
        cancelTimer()
        outcome = "approved"
    })

    selector.AddFuture(timer, func(f workflow.Future) {
        if err := f.Get(ctx, nil); err == nil {
            outcome = "timed-out"
        }
    })

    selector.Select(ctx)
    return ApprovalResult{Outcome: outcome}, nil
}
```

## Continue-as-New

```go
func longRunningWorkflow(ctx workflow.Context, state WorkflowState) (WorkflowResult, error) {
    for {
        state = processBatch(ctx, state)

        if state.IsComplete {
            return WorkflowResult{}, nil
        }

        if workflow.GetInfo(ctx).GetContinueAsNewSuggested() {
            return WorkflowResult{}, workflow.NewContinueAsNewError(ctx, longRunningWorkflow, state)
        }
    }
}
```

Drain signals before continue-as-new to avoid signal loss:

```go
for {
    if params, ok := mySignal.TryReceive(ctx); ok {
        // process signal
    } else {
        break
    }
}
return WorkflowResult{}, workflow.NewContinueAsNewError(ctx, longRunningWorkflow, state)
```

## Cancellation Handling

Use `ctx.Done()` to detect cancellation and `workflow.NewDisconnectedContext` for cleanup:

```go
func myWorkflow(ctx workflow.Context, params MyParams) (MyResult, error) {
    err := longRunningActivity.Run(ctx, params)
    if err != nil && temporal.IsCanceledError(ctx.Err()) {
        disconnectedCtx, _ := workflow.NewDisconnectedContext(ctx)
        disconnectedCtx = workflow.WithActivityOptions(disconnectedCtx, workflow.ActivityOptions{
            StartToCloseTimeout: 5 * time.Minute,
        })
        _ = cleanupActivity.Run(disconnectedCtx, CleanupParams{})
        return MyResult{}, err
    }
    return MyResult{}, err
}
```

## Saga Pattern (Compensations)

```go
func orderWorkflow(ctx workflow.Context, order OrderParams) (OrderResult, error) {
    var compensations []func(ctx workflow.Context) error

    runCompensations := func() {
        disconnectedCtx, _ := workflow.NewDisconnectedContext(ctx)
        disconnectedCtx = workflow.WithActivityOptions(disconnectedCtx, workflow.ActivityOptions{
            StartToCloseTimeout: 5 * time.Minute,
        })
        for i := len(compensations) - 1; i >= 0; i-- {
            if err := compensations[i](disconnectedCtx); err != nil {
                workflow.GetLogger(ctx).Error("Compensation failed", "error", err)
            }
        }
    }

    // Register compensation BEFORE running the activity
    compensations = append(compensations, func(ctx workflow.Context) error {
        _, err := releaseInventoryActivity.Run(ctx, ReleaseParams{OrderID: order.ID})
        return err
    })
    if _, err := reserveInventoryActivity.Run(ctx, ReserveParams{OrderID: order.ID}); err != nil {
        runCompensations()
        return OrderResult{}, err
    }

    compensations = append(compensations, func(ctx workflow.Context) error {
        _, err := refundPaymentActivity.Run(ctx, RefundParams{OrderID: order.ID})
        return err
    })
    if _, err := chargePaymentActivity.Run(ctx, ChargeParams{OrderID: order.ID}); err != nil {
        runCompensations()
        return OrderResult{}, err
    }

    if _, err := shipOrderActivity.Run(ctx, ShipParams{OrderID: order.ID}); err != nil {
        runCompensations()
        return OrderResult{}, err
    }

    return OrderResult{Status: "completed"}, nil
}
```

## Activity Heartbeating

Heartbeating is unchanged from the standard SDK -- use `activity.RecordHeartbeat` and `activity.HasHeartbeatDetails`:

```go
func processLargeFile(ctx context.Context, params ProcessFileParams) (ProcessFileResult, error) {
    startIdx := 0
    if activity.HasHeartbeatDetails(ctx) {
        if err := activity.GetHeartbeatDetails(ctx, &startIdx); err == nil {
            startIdx++
        }
    }

    lines := readFileLines(params.FilePath)
    for i := startIdx; i < len(lines); i++ {
        processLine(lines[i])
        activity.RecordHeartbeat(ctx, i)
        if ctx.Err() != nil {
            return ProcessFileResult{}, ctx.Err()
        }
    }

    return ProcessFileResult{Status: "completed"}, nil
}
```

## Timers

```go
// Simple sleep
err := workflow.Sleep(ctx, time.Hour)

// Timer as a Future (for use with Selector)
timerCtx, cancelTimer := workflow.WithCancel(ctx)
timer := workflow.NewTimer(timerCtx, 30*time.Minute)

// Cancel the timer when no longer needed
cancelTimer()
```
