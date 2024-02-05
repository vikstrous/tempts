package tstemporal

import (
	"context"
	"fmt"

	"go.temporal.io/sdk/worker"
)

type Worker struct {
	queue         *Queue
	registerables []Registerable
}

type Registry interface {
	worker.WorkflowRegistry
	worker.ActivityRegistry
}

type Registerable interface {
	register(ar Registry)
	// Make sure this activity or workflow is valid for this queue
	validate(q *Queue, v *ValidationState) error
}

type ValidationState struct {
	activitiesValidated map[string]struct{}
	workflowsValidated  map[string]struct{}
}

func NewWorker(queue *Queue, registerables []Registerable) (*Worker, error) {
	v := &ValidationState{
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

// Run starts the worker. To stop it, cancel the context. This function returns when the worker completes.
// Make sure to always cancel the context eventually, or a goroutine will be leaked.
func (w *Worker) Run(ctx context.Context, client *Client, options worker.Options) error {
	if w.queue.namespace.name != client.namespace {
		return fmt.Errorf("worker for namespace %s can't be started with client with namespace %s", w.queue.namespace.name, client.namespace)
	}
	wrk := worker.New(client.Client, w.queue.name, options)

	for _, r := range w.registerables {
		r.register(wrk)
	}

	go func() {
		// There's no way to pass the channel from ctx.Done directly into Run because it's of the wrong type.
		<-ctx.Done()
		wrk.Stop()
	}()
	return wrk.Run(nil)
}

// TODO: how do we force you to register the right activities and workflows on this worker???
