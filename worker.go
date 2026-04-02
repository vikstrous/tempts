package tempts

import (
	"context"
	"fmt"

	"go.temporal.io/sdk/worker"
)

// Worker represents a temporal worker that connects to the temporal server to execute activities and workflows.
type Worker struct {
	queue         *Queue
	registerables []Registerable
}

// Registerable can be created by calling WithImplementation() on activity or workflow definitions.
// It's a parameter to `tempts.NewWorker()`.
type Registerable interface {
	register(ar worker.Registry)
	// Make sure this activity or workflow is valid for this queue
	validate(q *Queue, v *validationState) error
}

type validationState struct {
	activitiesValidated map[string]struct{}
	workflowsValidated  map[string]struct{}
}

// NewWorker defines a worker along with all of the workflows and activities. Example usage:
/*
wrk, err := tempts.NewWorker(queueMain, []tempts.Registerable{
	activityTypeFormatName.WithImplementation(activityFormatName),
	activityTypeGreet.WithImplementation(activityGreet),
	workflowTypeFormatAndGreet.WithImplementation(workflowFormatAndGreet),
	workflowTypeJustGreet.WithImplementation(workflowJustGreet),
})
*/
func NewWorker(queue *Queue, registerables []Registerable) (*Worker, error) {
	v := &validationState{
		activitiesValidated: map[string]struct{}{},
		workflowsValidated:  map[string]struct{}{},
	}
	for _, r := range registerables {
		err := r.validate(queue, v)
		if err != nil {
			return nil, err
		}
	}
	for a := range queue.activities {
		if _, ok := v.activitiesValidated[a]; !ok {
			return nil, fmt.Errorf("an implementation for activity %s not provided when calling NewWorker", a)
		}
	}
	for a := range v.activitiesValidated {
		if _, ok := queue.activities[a]; !ok {
			return nil, fmt.Errorf("an implementation for activity %s provided, but not registered with queue", a)
		}
	}
	for w := range queue.workflows {
		if _, ok := v.workflowsValidated[w]; !ok {
			return nil, fmt.Errorf("an implementation for workflow %s not provided when calling NewWorker", w)
		}
	}
	for w := range v.workflowsValidated {
		if _, ok := queue.workflows[w]; !ok {
			return nil, fmt.Errorf("an implementation for workflow %s provided, but not registered with queue", w)
		}
	}
	return &Worker{queue: queue, registerables: registerables}, nil
}

// Register is useful in unit tests to define all of the worker's workflows and activities in the test environment.
func (w *Worker) Register(wrk worker.Registry) {
	for _, r := range w.registerables {
		r.register(wrk)
	}
}

// Build creates a configured Temporal SDK worker with all workflows and activities registered.
// Use this when you need to manage the worker lifecycle yourself (e.g. with a custom runnable).
func (w *Worker) Build(client *Client, options worker.Options) worker.Worker {
	options.DisableRegistrationAliasing = true
	wrk := worker.New(client.Client, w.queue.name, options)
	w.Register(wrk)
	return wrk
}

// Run starts the worker. To stop it, cancel the context. This function returns when the worker completes.
// Make sure to always cancel the context eventually, or a goroutine will be leaked.
func (w *Worker) Run(ctx context.Context, client *Client, options worker.Options) error {
	wrk := w.Build(client, options)

	// Use an interrupt channel instead of calling Stop() directly to avoid a race
	// condition between Start() and Stop(). Run(interruptCh) calls Start()
	// synchronously before selecting on the channel, ensuring the worker is fully
	// started before Stop() is called.
	interruptCh := make(chan interface{})
	go func() {
		<-ctx.Done()
		close(interruptCh)
	}()
	return wrk.Run(interruptCh)
}
