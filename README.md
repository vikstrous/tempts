# tstemporal
Type-safe Temporal Go SDK wrapper

[![Go Reference](https://pkg.go.dev/badge/github.com/vikstrous/tstemporal.svg)](https://pkg.go.dev/github.com/vikstrous/tstemporal)


## Example Usage

Add this dependency with
```
go get github.com/vikstrous/tstemporal@latest
```

Below is a simple example demonstrating how to define a workflow and an activity, register them, and execute the workflow using `tstemporal`.

```go
package main

import (
	"context"
	"fmt"
	"time"

	"github.com/vikstrous/tstemporal"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"
)

// Define a new namespace and task queue.
var nsDefault = tstemporal.NewNamespace(client.DefaultNamespace)
var queueMain = tstemporal.NewQueue(nsDefault, "main")

// Define a workflow with no parameters and no return.
var workflowTypeHello = tstemporal.NewWorkflow[struct{}, struct{}](queueMain, "HelloWorkflow")

// Define an activity with no parameters and no return.
var activityTypeHello = tstemporal.NewActivity[struct{}, struct{}](queueMain, "HelloActivity")

func main() {
	// Create a new client connected to the Temporal server.
	c, err := tstemporal.Dial(client.Options{})
	if err != nil {
		panic(err)
	}
	defer c.Close()

	// Register the workflow and activity in a new worker.
	wrk, err := tstemporal.NewWorker(queueMain, []tstemporal.Registerable{
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

This example sets up a workflow and an activity that simply prints a greeting. It demonstrates the basic setup and execution flow using `tstemporal`. To see a more complex example, look in the example directory.

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

* Set the right workflow argument types
* Can be "set" on start up of the application and the intended effect will be applied to the state of the schedule on the cluster automatically

Queries and updates:

* Are called with the right types
* Return the right types
* Registered functions match the right type signature

### Tools

There are two functions in this library that make it easy to write fixture based replyabaility tests for your tstemporal workflows and activities.
See `example/main_test.go` for an example of how to use them.
```go
func GetWorkflowHistoriesBundle(ctx context.Context, client *tstemporal.Client, w *tstemporal.WorkflowWithImpl) ([]byte, error)
func ReplayWorkflow(historiesBytes []byte, w *tstemporal.WorkflowWithImpl) error
```

## User guide by example

These examples assume that you are already familiar with the Go SDK and just need to know how to do the equivalent thing using this library.

### Connect

Instead of:
```go
c, err := tstemporal.Dial(client.Options{})
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
var nsDefault = tstemporal.NewNamespace(client.DefaultNamespace)
var queueMain = tstemporal.NewQueue(nsDefault, "main")
type exampleWorkflowParamType struct{
	Param string
}
type exampleWorkflowReturnType struct{
	Return string
}
var exampleWorkflowType = tstemporal.NewWorkflow[exampleWorkflowParamType, exampleWorkflowReturnType](queueMain, "ExampleWorkflow")

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
var nsDefault = tstemporal.NewNamespace(client.DefaultNamespace)
var queueMain = tstemporal.NewQueue(nsDefault, "main")
type exampleActivityParamType struct{
	Param string
}
type exampleActivityReturnType struct{
	Return string
}
var exampleActivityType = tstemporal.NewActivity[exampleActivityParamType, exampleActivityReturnType](queueMain, "ExampleActivity")

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
var nsDefault = tstemporal.NewNamespace(client.DefaultNamespace)
var queueMain = tstemporal.NewQueue(nsDefault, "main")

// On start up of your service
wrk, err := tstemporal.NewWorker(queueMain, []tstemporal.Registerable{
	exampleWorkflowType.WithImplementation(exampleWorkflow),
	exampleActivityType.WithImplementation(exampleActivity),
})

err = wrk.Run(ctx, c, worker.Options{})
```
This ensures that all the right workflows and activities are registered for this worker to satisfy the expectations for this queue. No more and no less.

### Query a workflow

TODO

### Create a schedule

TODO

### Fixture based tests

TODO

## Migration

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