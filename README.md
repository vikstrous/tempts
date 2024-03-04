# tempts
Type-safe Temporal Go SDK wrapper

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

// Define a new namespace and task queue.
var nsDefault = tempts.NewNamespace(client.DefaultNamespace)
var queueMain = tempts.NewQueue(nsDefault, "main")

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
    ctx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
        StartToCloseTimeout: time.Second * 10,
    })
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

* Are called on the right namespace and queue
* Are called with the right parameter types
* Return the right response types
* Registered functions match the right type signature

Workflows:

* Are called on the right namespace and queue
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

### Tools

There are two functions in this library that make it easy to write fixture based replyabaility tests for your tempts workflows and activities.
See `example/main_test.go` for an example of how to use them.
```go
func GetWorkflowHistoriesBundle(ctx context.Context, client *tempts.Client, w *tempts.WorkflowWithImpl) ([]byte, error)
func ReplayWorkflow(historiesBytes []byte, w *tempts.WorkflowWithImpl) error
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
This creates a wrapper that keeps track of which namespace the client is connected to so that actions are attempted against the wrong namespace, such as starting the wrong workflow, can be caught early.

### Run a workflow to completion

Instead of:
```go
var ret ReturnType
c.ExecuteWorkflow(ctx, opts, name, param).Get(ctx, &ret)
```
Do:
```go
// Globally define the namespace, queue and workflow type (one time, in a centralized package for the queue)
var nsDefault = tempts.NewNamespace(client.DefaultNamespace)
var queueMain = tempts.NewQueue(nsDefault, "main")
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
This ensures that the workflow is run in the right namespace, on the right queue, with the right name, with the right parameter types and it returns the right type.

### Run an activity to completion

Instead of:
```go
var ret ReturnType
workflow.ExecuteActivity(ctx, name, param).Get(ctx, &ret)
```
Do:
```go
// Globally define the namespace, queue and activity type (one time, in a centralized package for the queue)
var nsDefault = tempts.NewNamespace(client.DefaultNamespace)
var queueMain = tempts.NewQueue(nsDefault, "main")
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

This ensures that the activity is run in the right namespace, on the right queue, with the right name, with the right parameter types and it returns the right type.

### Create a worker

Instead of:
```go
wrk := worker.New(c, queueName, options)
err = wrk.Run(worker.InterruptCh())
```
Do:
```go
// Globally define the namespace and queue (one time, in a centralized package for the queue)
var nsDefault = tempts.NewNamespace(client.DefaultNamespace)
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

This ensures that the namespace and queue are correct for the workflow. It also ensures that the parameter is the right type and that the schedule is updated to match the one defined in your code. It doesn't handle every possible difference yet because temporal doesn't support arbitrary changes to schedules. This feature is a bit less polished than the rest of the package, so let me know how to improve it!

### Fixture based tests

Instead of: coming up with your own strategy to build fixture based tests
Do:
```go
c, err := tempts.Dial(client.Options{})
if err != nil {
    t.Fatal(err)
}
workflowImpl := exampleWorkflowType.WithImplementation(exampleWorkflow)
historiesData, err = tempts.GetWorkflowHistoriesBundle(ctx, c, workflowImpl)
if err != nil {
    t.Fatal(err)
}
// Now store historiesData somewhere! (Or don't and make sure your test is always connected to a temporal instance with example workflow runs)
err := tempts.ReplayWorkflow(historiesData, workflowImpl)
if err != nil {
    t.Fatal(err)
}
```

This is really just the cherry on top once you have your type safety in place. By running fixture based tests, you can make sure to not introduce backwards incompatible changes without versioning them correctly. Even if you don't end up using this package, feel free to adapt this pattern for your own needs. It's not a lot of code for how much extra safety it provides.

## Migration for Go SDK users

Since this library is opinionated, it doesn't support all temporal features. To use this library effectively, the temporal queue you are migrating must meet these pre-requisities:
* All workflows must take at most one parameter and return at most one parameter.
* All activities must take at most one parameter and return at most one parameter.
* Namespaces names and queues names must be static (assuming that your types are defined as global variables so they can be used anywhere).
* All workflows and activities for the queue must be known.
* All types must be defined as Go structs that follow the Temporal Go SDK's marshaling and unmarshaling logic.

There maybe more restrictions that I'm not aware of yet. Open an issue if any are missed.

## Potential future improvements

* Wrap the APIs for channels and signals
* Return typed futures instead of generic ones
* Ensure that all queries are defined on the right workflows