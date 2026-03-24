# tempts Nexus Operations

## Overview

tempts provides type-safe wrappers for Temporal Nexus operations. There are three operation types:

- **SyncOperation** -- Simple RPC-style request-response
- **AsyncOperation** -- Backed by a workflow with matching input/output types
- **AsyncHandlerOperation** -- Custom handler where operation input can differ from workflow input

Operations are declared on a `Service`, and all declared operations must have implementations when creating a worker.

## Declaring Services and Operations

```go
// Declare a Nexus service (in a shared API package)
var myService = tempts.NewService("my-nexus-service")

// Sync operation -- simple RPC
type EchoInput struct { Message string }
type EchoOutput struct { Message string }
var echoOp = tempts.NewSyncOperation[EchoInput, EchoOutput](myService, "echo")

// Async operation -- workflow-backed, input/output types match the workflow
type ProcessInput struct { Data string }
type ProcessOutput struct { Result string }
var processOp = tempts.NewAsyncOperation[ProcessInput, ProcessOutput](myService, "process")

// Async handler operation -- operation input can differ from workflow input
type TransformInput struct { RawData string }
type TransformOutput struct { Result string }
var transformOp = tempts.NewAsyncHandlerOperation[TransformInput, TransformOutput](myService, "transform")
```

## Implementing Operations

### Sync Operation

```go
func echoHandler(ctx context.Context, input EchoInput, opts nexus.StartOperationOptions) (EchoOutput, error) {
    return EchoOutput{Message: "Echo: " + input.Message}, nil
}

// Register:
echoOp.WithImplementation(echoHandler)
```

### Async Operation

Requires a workflow function and a function that returns `client.StartWorkflowOptions`:

```go
func processWorkflow(ctx workflow.Context, input ProcessInput) (ProcessOutput, error) {
    return ProcessOutput{Result: "Processed: " + input.Data}, nil
}

func processGetOptions(ctx context.Context, input ProcessInput, opts nexus.StartOperationOptions) (client.StartWorkflowOptions, error) {
    return client.StartWorkflowOptions{
        ID: "process-" + opts.RequestID,
    }, nil
}

// Register:
processOp.WithImplementation(processWorkflow, processGetOptions)
```

### Async Handler Operation

The handler receives the operation input and returns a `WorkflowHandle`. This allows the operation input type to differ from the workflow input type:

```go
func transformHandler(ctx context.Context, input TransformInput, opts nexus.StartOperationOptions) (temporalnexus.WorkflowHandle[TransformOutput], error) {
    return temporalnexus.ExecuteWorkflow(ctx, opts, client.StartWorkflowOptions{
        ID: "transform-" + opts.RequestID,
    }, transformWorkflow, ProcessInput{Data: input.RawData})
}

// Register:
transformOp.WithImplementation(transformHandler)
```

## Registering with a Worker

Nexus operation implementations are passed directly into `NewWorker` alongside activities and workflows. `NewWorker` groups them by service and validates completeness:

```go
wrk, err := tempts.NewWorker(queueMain, []tempts.Registerable{
    // Activities and workflows
    workflowType.WithImplementation(workflowFn),
    activityType.WithImplementation(activityFn),
    // Nexus operations
    echoOp.WithImplementation(echoHandler),
    processOp.WithImplementation(processWorkflow, processGetOptions),
    transformOp.WithImplementation(transformHandler),
})
```

`NewWorker` validates that all operations declared on `myService` have implementations and that no extra implementations are provided.

## Calling Operations from a Workflow

Create a `NexusClient` from the service declaration, then call operations:

```go
func callerWorkflow(ctx workflow.Context, params CallerParams) (CallerResult, error) {
    c := myService.NewClient("my-nexus-endpoint")

    // Synchronous call
    echoResult, err := echoOp.Run(ctx, c, EchoInput{Message: "hello"}, workflow.NexusOperationOptions{})
    if err != nil {
        return CallerResult{}, err
    }

    // Asynchronous call
    future := processOp.Execute(ctx, c, ProcessInput{Data: echoResult.Message}, workflow.NexusOperationOptions{})
    // ... do other work ...
    var processResult ProcessOutput
    err = future.Get(ctx, &processResult)
    if err != nil {
        return CallerResult{}, err
    }

    return CallerResult{Output: processResult.Result}, nil
}
```

The client validates at runtime that the operation belongs to the correct service. Calling an operation with a client from a different service panics with a descriptive error.
