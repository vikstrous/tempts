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

// Run starts the worker. To stop it, cancel the context. This function returns when the worker completes.
// Make sure to always cancel the context eventually, or a goroutine will be leaked.
func (w *Worker) Run(ctx context.Context, client *Client, options worker.Options) error {
	if w.queue.namespace.name != client.namespace {
		return fmt.Errorf("worker for namespace %s can't be started with client with namespace %s", w.queue.namespace.name, client.namespace)
	}
	options.DisableRegistrationAliasing = true
	wrk := worker.New(client.Client, w.queue.name, options)
	w.Register(wrk)

	go func() {
		// There's no way to pass the channel from ctx.Done directly into Run because it's of the wrong type.
		<-ctx.Done()
		wrk.Stop()
	}()
	return wrk.Run(nil)
}
