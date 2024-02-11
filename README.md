# tstemporal
Type-safe Temporal Go SDK wrapper

## Example Usage

Below is a simple example demonstrating how to define a workflow and an activity, register them, and execute the workflow using `tstemporal`.

```go
package main

import (
	"context"
	"fmt"
	"github.com/vikstrous/tstemporal"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"
)

// Define a new namespace and task queue.
var nsDefault = tstemporal.NewNamespace(client.DefaultNamespace)
var queueMain = tstemporal.NewQueue(nsDefault, "main")

// Define a workflow with no parameters and no return.
var workflowTypeHello = tstemporal.NewWorkflow0(queueMain, "HelloWorkflow")

// Define an activity with no parameters and no return.
var activityTypeHello = tstemporal.NewActivity0(queueMain, "HelloActivity")

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
	err = workflowTypeHello.Run(ctx, c, client.StartWorkflowOptions{})
	if err != nil {
		panic(err)
	}

	fmt.Println("Workflow completed.")
}

// helloWorkflow is a workflow function that calls the HelloActivity.
func helloWorkflow(ctx workflow.Context) error {
	ctx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: time.Second * 10,
	})
	err := activityTypeHello.Run(ctx)
	return err
}

// helloActivity is an activity function that prints "Hello, Temporal!".
func helloActivity(ctx context.Context) error {
	fmt.Println("Hello, Temporal!")
	return nil
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

Tools:
* Plumbing for fixture based tests with namespace safety checks

More guarantees coming soon:

* Signals type safety