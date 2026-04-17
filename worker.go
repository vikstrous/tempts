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
	validate(v *validationState) error
}

type validationState struct {
	queue               *Queue
	activitiesValidated map[string]struct{}
	workflowsValidated  map[string]struct{}
	// nexusOps groups operation implementations by their parent service name.
	nexusOps map[string][]OperationWithImpl
	// nexusServicesHandled tracks service names that were passed as pre-validated ServiceWithImpl.
	nexusServicesHandled map[string]struct{}
}

// NewWorker defines a worker along with all of the workflows, activities and Nexus operations.
// Nexus operation implementations are automatically grouped by service and validated for
// completeness (all declared operations must have implementations). Example usage:
/*
wrk, err := tempts.NewWorker(queueMain, []tempts.Registerable{
	activityTypeFormatName.WithImplementation(activityFormatName),
	workflowTypeFormatAndGreet.WithImplementation(workflowFormatAndGreet),
	echoOp.WithImplementation(echoHandler),
	processOp.WithImplementation(processWorkflow, processGetOptions),
})
*/
func NewWorker(queue *Queue, registerables []Registerable) (*Worker, error) {
	v := &validationState{
		queue:                queue,
		activitiesValidated:  map[string]struct{}{},
		workflowsValidated:   map[string]struct{}{},
		nexusOps:             map[string][]OperationWithImpl{},
		nexusServicesHandled: map[string]struct{}{},
	}
	for _, r := range registerables {
		err := r.validate(v)
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

	// Bundle nexus operations into services and validate completeness.
	var services []*ServiceWithImpl
	for _, ops := range v.nexusOps {
		svc := ops[0].getService()
		svcImpl, err := svc.WithImplementations(ops...)
		if err != nil {
			return nil, err
		}
		services = append(services, svcImpl)
	}

	// Add the built ServiceWithImpls to registerables so Register() picks them up.
	allRegisterables := make([]Registerable, 0, len(registerables)+len(services))
	allRegisterables = append(allRegisterables, registerables...)
	for _, svc := range services {
		allRegisterables = append(allRegisterables, svc)
	}

	return &Worker{queue: queue, registerables: allRegisterables}, nil
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
	options.DisableRegistrationAliasing = true
	wrk := worker.New(client.Client, w.queue.name, options)
	w.Register(wrk)

	interruptCh := make(chan any)
	go func() {
		<-ctx.Done()
		close(interruptCh)
	}()
	return wrk.Run(interruptCh)
}
