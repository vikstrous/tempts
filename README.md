# tempts
`tempts` stands for "Temporal Type-Safe", a Go SDK wrapper for interacting with temporal more safely.

[![Go Reference](https://pkg.go.dev/badge/github.com/vikstrous/tempts.svg)](https://pkg.go.dev/github.com/vikstrous/tempts)


## Example Usage

Add this dependency with
```
go get github.com/vikstrous/tempts@latest
```

Below is a simple example demonstrating how to define a workflow and an activity, register them, and execute the workflow using `tempts`.

```go
package main

import (
    "context"
    "fmt"
    "time"

    "github.com/vikstrous/tempts"
    "go.temporal.io/sdk/client"
    "go.temporal.io/sdk/worker"
    "go.temporal.io/sdk/workflow"
)

// Define a new task queue.
var queueMain = tempts.NewQueue("main")

// Define a workflow with no parameters and no return.
var workflowTypeHello = tempts.NewWorkflow[struct{}, struct{}](queueMain, "HelloWorkflow")

// Define an activity with no parameters and no return.
var activityTypeHello = tempts.NewActivity[struct{}, struct{}](queueMain, "HelloActivity")

func main() {
    // Create a new client connected to the Temporal server.
    c, err := tempts.Dial(client.Options{})
    if err != nil {
        panic(err)
    }
    defer c.Close()

    // Register the workflow and activity in a new worker.
    wrk, err := tempts.NewWorker(queueMain, []tempts.Registerable{
        workflowTypeHello.WithImplementation(helloWorkflow),
        activityTypeHello.WithImplementation(helloActivity),
    })
    if err != nil {
        panic(err)
    }
    ctx := context.Background()
    ctx, cancel := context.WithCancel(ctx)
    defer cancel()
    go func() {
        err = wrk.Run(ctx, c, worker.Options{})
        if err != nil {
            panic(err)
        }
    }()

    // Execute the workflow and wait for it to complete.
    _, err = workflowTypeHello.Run(ctx, c, client.StartWorkflowOptions{}, struct{}{})
    if err != nil {
        panic(err)
    }

    fmt.Println("Workflow completed.")
}

// helloWorkflow is a workflow function that calls the HelloActivity.
func helloWorkflow(ctx workflow.Context, _ struct{}) (struct{}, error) {
    return activityTypeHello.Run(ctx, struct{}{})
}

// helloActivity is an activity function that prints "Hello, Temporal!".
func helloActivity(ctx context.Context, _ struct{}) (struct{}, error) {
    fmt.Println("Hello, Temporal!")
    return struct{}{}, nil
}

```

This example sets up a workflow and an activity that simply prints a greeting. It demonstrates the basic setup and execution flow using `tempts`. To see a more complex example, look in the example directory.

**WARNING: This library can change without notice while I respond to feedback and improve the API. I'll remove this warning when I'm happy with the API and can promise it won't change.**

## Guarantees

List of guarantees provided by this wrapper:

Workers:

* Have all the right activities and workflows registered before starting

Activities:

* Are called on the right queue
* Are called with the right parameter types
* Return the right response types
* Registered functions match the right type signature

Workflows:

* Are called on the right queue
* Are called with the right parameter types
* Return the right response types
* Registered functions match the right type signature

Schedules:

* Set arguments with the right types
* Can be set upon application startup, automatically applying the intended effect to the schedule's state on the cluster

Queries and updates:

* Are called with the right types
* Return the right types
* Registered functions match the right type signature

Signals:

* Are sent with the right parameter types
* Are received with the right parameter types
* SignalWithStart ensures both workflow and signal parameter types are correct

Nexus Operations:

* Registered handlers match the right type signature (also provided by the native Nexus SDK via generics)
* Are called with the right parameter types (the native SDK's `ExecuteOperation` accepts `any`, so call-site type safety is only enforced by tempts)
* Return the right response types (same as above — call-site only)
* Are declared on a specific service
* All declared operations must have implementations (validated at service creation)
* Parameter types must be structs (enforced at declaration time)
* Operations can only be called with a client created from the same service
* Async handler operations allow operation input to differ from workflow input while maintaining type safety on the operation boundary

### Tools

There are two functions in this library that make it easy to write fixture based replyabaility tests for your tempts workflows and activities.
See `example/main_test.go` for an example of how to use them.
```go
func GetWorkflowHistoriesBundle(ctx context.Context, client *tempts.Client, w *tempts.WorkflowWithImpl) ([]byte, error)
func ReplayWorkflow(historiesBytes []byte, w *tempts.WorkflowWithImpl, opts worker.WorkflowReplayerOptions) error
```

## User guide by example

These examples assume that you are already familiar with the Go SDK and just need to know how to do the equivalent thing using this library.

### Connect to temporal

Instead of:
```go
c, err := tempts.Dial(client.Options{})
```
Do:
```go
c, err := client.Dial(client.Options{})
```

### Run a workflow to completion

Instead of:
```go
var ret ReturnType
c.ExecuteWorkflow(ctx, opts, name, param).Get(ctx, &ret)
```
Do:
```go
// Globally define the queue and workflow type (one time, in a centralized package for the queue)
var queueMain = tempts.NewQueue("main")
type exampleWorkflowParamType struct{
    Param string
}
type exampleWorkflowReturnType struct{
    Return string
}
var exampleWorkflowType = tempts.NewWorkflow[exampleWorkflowParamType, exampleWorkflowReturnType](queueMain, "ExampleWorkflow")

// Run and get the result of the workflow
ret, err := exampleWorkflowType.Run(ctx, c, opts, param)
// Use Execute instead of Run to not wait for completion
```
This ensures that the workflow is run on the right queue, with the right name, with the right parameter types and it returns the right type.

### Run an activity to completion

Instead of:
```go
var ret ReturnType
workflow.ExecuteActivity(ctx, name, param).Get(ctx, &ret)
```
Do:
```go
// Globally define the queue and activity type (one time, in a centralized package for the queue)
var queueMain = tempts.NewQueue("main")
type exampleActivityParamType struct{
    Param string
}
type exampleActivityReturnType struct{
    Return string
}
var exampleActivityType = tempts.NewActivity[exampleActivityParamType, exampleActivityReturnType](queueMain, "ExampleActivity")

// Run and get the result of the activity from a workflow
ret, err := exampleActivityType.Run(ctx, param)
// Use Execute instead of Run to not wait for completion
```

This ensures that the activity is run on the right queue, with the right name, with the right parameter types and it returns the right type.

### Create a worker

Instead of:
```go
wrk := worker.New(c, queueName, options)
err = wrk.Run(worker.InterruptCh())
```
Do:
```go
// Globally define the queue (one time, in a centralized package for the queue)
var queueMain = tempts.NewQueue(nsDefault, "main")

// On start up of your service
wrk, err := tempts.NewWorker(queueMain, []tempts.Registerable{
    exampleWorkflowType.WithImplementation(exampleWorkflow),
    exampleActivityType.WithImplementation(exampleActivity),
})

err = wrk.Run(ctx, c, worker.Options{})
```
This ensures that all the right workflows and activities are registered for this worker to satisfy the expectations for this queue. No more and no less.

### Query a workflow

Instead of:
```go
// In the workflow
workflow.SetQueryHandler(ctx, queryName, func(queryParamType) (queryReturnType, error) {
    return queryReturnType{}, nil
})

// Querying it from your application
response, err := c.QueryWorkflow(ctx, workflowID, runID, queryName, param)

var value Return
err = response.Get(&value)
```
Do:
```go
// Globally define the query type
var exampleQueryType = tempts.NewQueryHandler[queryParamType, queryReturnType]("exampleQuery")

// In the workflow
exampleQueryType.SetHandler(ctx, func(queryParamType) (queryReturnType, error) {
    return queryReturnType{}, nil
})

// Query it from your application
ret, err := exampleQueryType.Query(ctx, c, workflowID, runID, param)
```

This ensures that the types match in the application calling the workflow and in the workflow handler function. Unfortunately, we don't guarantee that the workflow is the expected type.

### Create a schedule

Instead of:
```go
_, err = c.ScheduleClient().Create(ctx, client.ScheduleOptions{
    ID:   scheduleID,
    Spec: client.ScheduleSpec{
        Intervals: []client.ScheduleIntervalSpec{
            {
                Every: time.Second * 5,
            },
        },
    },
    Action: &client.ScheduleWorkflowAction{
        ID:        workflowID,
        Workflow:  workflowName,
        TaskQueue: queueName,
        Args:      []any{param},
    },
})
// Ignore the error if it exists already
```
Do:
```go
// This assumes the workflow is already globally registered

// Call this to make sure the schedule matches what your service expects
err = workflowTypeFormatAndGreet.SetSchedule(ctx, c, client.ScheduleOptions{
    ID: scheduleID,
    Spec: client.ScheduleSpec{
        Intervals: []client.ScheduleIntervalSpec{
            {
                Every: time.Second * 5,
            },
        },
    },
}, param)
```

This ensures that the queue is correct for the workflow. It also ensures that the parameter is the right type and that the schedule is updated to match the one defined in your code. It doesn't handle every possible difference yet because temporal doesn't support arbitrary changes to schedules. This feature is a bit less polished than the rest of the package, so let me know how to improve it!

### Fixture based tests

Instead of: coming up with your own strategy to build fixture based tests
Do:
```go
c, err := tempts.Dial(client.Options{})
if err != nil {
    t.Fatal(err)
}
historiesData, err = tempts.GetWorkflowHistoriesBundle(ctx, c, exampleWorkflowType)
if err != nil {
    t.Fatal(err)
}
// Now store historiesData somewhere! (Or don't and make sure your test is always connected to a temporal instance with example workflow runs)
err := tempts.ReplayWorkflow(historiesData, exampleWorkflow, worker.WorkflowReplayerOptions{})
if err != nil {
    t.Fatal(err)
}
```

This is really just the cherry on top once you have your type safety in place. By running fixture based tests, you can make sure to not introduce backwards incompatible changes without versioning them correctly. Even if you don't end up using this package, feel free to adapt this pattern for your own needs. It's not a lot of code for how much extra safety it provides.

### Send a signal to a workflow

Instead of:
```go
// In the workflow
ch := workflow.GetSignalChannel(ctx, signalName)
var param SignalParamType
ch.Receive(ctx, &param)

// Sending a signal from your application
err := c.SignalWorkflow(ctx, workflowID, runID, signalName, param)
```
Do:
```go
// Globally define the signal type, scoped to a workflow
type SignalParamType struct {
    Message string
}
var exampleSignal = tempts.NewWorkflowSignal[SignalParamType](&exampleWorkflowType, "exampleSignal")

// In the workflow - Option 1: Receive (blocking)
param := exampleSignal.Receive(ctx)
// handle param

// In the workflow - Option 2: TryReceive (non-blocking)
if param, ok := exampleSignal.TryReceive(ctx); ok {
    // handle param
}

// In the workflow - Option 3: AddToSelector for multiple signals
selector := workflow.NewSelector(ctx)
exampleSignal.AddToSelector(ctx, selector, func(param SignalParamType) {
    // handle param
})
otherSignal.AddToSelector(ctx, selector, func(param OtherParamType) {
    // handle other signal
})
selector.Select(ctx)

// Sending a signal from your application
err := exampleSignal.Signal(ctx, c, workflowID, runID, SignalParamType{Message: "hello"})
```

This ensures that the signal parameter type matches between the sender and receiver.

### SignalWithStart a workflow

Instead of:
```go
_, err := c.SignalWithStartWorkflow(ctx, workflowID, signalName, signalParam, opts, workflowName, workflowParam)
```
Do:
```go
// Using the signal defined above
run, err := exampleSignal.SignalWithStart(ctx, c, opts, workflowParam, signalParam)
```

This ensures that:
* The workflow parameter type is correct for the workflow
* The signal parameter type is correct for the signal
* The signal is scoped to the correct workflow

### Define and implement Nexus operations

Instead of:
```go
// Define operations manually
echoOp := nexus.NewSyncOperation("echo", func(ctx context.Context, input EchoInput, opts nexus.StartOperationOptions) (EchoOutput, error) {
    return EchoOutput{Message: input.Message}, nil
})

processOp := temporalnexus.NewWorkflowRunOperation("process", processWorkflow, getOptions)

// Create service and register
service := nexus.NewService("my-service")
service.Register(echoOp, processOp)
worker.RegisterNexusService(service)
```
Do:
```go
// Globally define the service and operations (in a shared API package)
var myService = tempts.NewService("my-service")

type EchoInput struct { Message string }
type EchoOutput struct { Message string }
var echoOp = tempts.NewSyncOperation[EchoInput, EchoOutput](myService, "echo")

type ProcessInput struct { Data string }
type ProcessOutput struct { Result string }
var processOp = tempts.NewAsyncOperation[ProcessInput, ProcessOutput](myService, "process")

// Async handler operation - when operation input differs from workflow input
type TransformInput struct { RawData string }
type TransformOutput struct { Result string }
var transformOp = tempts.NewAsyncHandlerOperation[TransformInput, TransformOutput](myService, "transform")

// Operation implementations go directly into NewWorker alongside activities and workflows.
// NewWorker groups them by service and validates that all declared operations have implementations.
wrk, err := tempts.NewWorker(queueMain, []tempts.Registerable{
    workflowType.WithImplementation(workflowFn),
    echoOp.WithImplementation(func(ctx context.Context, input EchoInput, opts nexus.StartOperationOptions) (EchoOutput, error) {
        return EchoOutput{Message: input.Message}, nil
    }),
    processOp.WithImplementation(processWorkflow, func(ctx context.Context, input ProcessInput, opts nexus.StartOperationOptions) (client.StartWorkflowOptions, error) {
        return client.StartWorkflowOptions{ID: "process-" + opts.RequestID}, nil
    }),
    transformOp.WithImplementation(func(ctx context.Context, input TransformInput, opts nexus.StartOperationOptions) (temporalnexus.WorkflowHandle[TransformOutput], error) {
        return temporalnexus.ExecuteWorkflow(ctx, opts, client.StartWorkflowOptions{
            ID: "transform-" + opts.RequestID,
        }, processWorkflow, ProcessInput{Data: input.RawData})
    }),
})
```

This ensures that all declared operations have implementations and no extra implementations are provided. A single operation object is used for both declaration and implementation, unlike the native SDK which requires separate `OperationReference[I, O]` objects.

### Call Nexus operations from a workflow

Instead of:
```go
// Note: ExecuteOperation accepts `any` for both the operation name and input,
// so mismatched types compile but fail at runtime.
c := workflow.NewNexusClient("my-endpoint", "my-service")
future := c.ExecuteOperation(ctx, "echo", input, workflow.NexusOperationOptions{})
var result EchoOutput
err := future.Get(ctx, &result)
```
Do:
```go
// Using the globally defined service and operations
c := myService.NewClient(ctx, "my-endpoint")

// Synchronous call
result, err := echoOp.Run(ctx, c, EchoInput{Message: "hello"}, workflow.NexusOperationOptions{})

// Or async call
future := echoOp.Execute(ctx, c, EchoInput{Message: "hello"}, workflow.NexusOperationOptions{})
// ... do other work ...
var result EchoOutput
err := future.Get(ctx, &result)
```

This ensures that the operation is called with the correct parameter type and returns the correct result type. The native SDK's `ExecuteOperation` accepts `any` for both operation and input, so mismatched types compile but fail at runtime. tempts enforces correct types at compile time via generics, and verifies the client matches the operation's service at runtime.

## Migration for Go SDK users

Since this library is opinionated, it doesn't support all temporal features. To use this library effectively, the temporal queue you are migrating must meet these pre-requisities:
* Queues names must be static (assuming that your types are defined as global variables so they can be used anywhere).
* All workflows and activities for a given queue must be migrated at once.
* All types must be defined as Go structs that follow the Temporal Go SDK's marshaling and unmarshaling logic.

There maybe more restrictions that I'm not aware of yet. Open an issue if any are missed.

To simplify migration, if your workflows and activities don't use a single struct input type, use `NewWorkflowPositional`/`NewActivityPositional`. Don't use these functions in new code.

## Potential future improvements

* Return typed futures instead of generic ones
* Ensure that all queries and signals are defined on the right workflows
