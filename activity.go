package tempts

import (
	"context"
	"fmt"

	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"
)

// ActivityWithImpl is a temporary struct that implements Registerable. It's meant to be passed into `tempts.NewWorker`.
type ActivityWithImpl struct {
	activityName string
	queue        *Queue
	fn           any
}

func (a ActivityWithImpl) register(ar worker.Registry) {
	ar.RegisterActivityWithOptions(a.fn, activity.RegisterOptions{Name: a.activityName})
}

func (a ActivityWithImpl) validate(q *Queue, v *validationState) error {
	if a.queue.name != q.name {
		return fmt.Errorf("activity for queue %s can't be registered on worker with queue %s", a.queue.name, q.name)
	}
	if a.queue.namespace.name != q.namespace.name {
		return fmt.Errorf("activity for namespace %s can't be registered on worker with namespace %s", a.queue.namespace.name, q.namespace.name)
	}
	_, ok := v.activitiesValidated[a.activityName]
	if ok {
		return fmt.Errorf("duplicate activtity name %s for queue %s and namespace %s", a.activityName, q.name, q.namespace.name)
	}
	v.activitiesValidated[a.activityName] = struct{}{}
	return nil
}

// Activity is used for interacting with activities in a safe way that takes into account the input and output types, queue name and other properties.
type Activity[Param, Return any] struct {
	Name  string
	queue *Queue
}

// NewActivity declares the existence of an activity on a given queue with a given name.
func NewActivity[Param, Return any](q *Queue, name string) Activity[Param, Return] {
	q.registerActivity(name, (func(context.Context, Param) (Return, error))(nil))
	return Activity[Param, Return]{Name: name, queue: q}
}

// WithImplementation should be called to create the parameters for NewWorker(). It declares which function implements the activity.
func (a Activity[Param, Return]) WithImplementation(fn func(context.Context, Param) (Return, error)) *ActivityWithImpl {
	return &ActivityWithImpl{activityName: a.Name, queue: a.queue, fn: fn}
}

// Run executes the activity and synchronously returns the output.
func (a Activity[Param, Return]) Run(ctx workflow.Context, param Param) (Return, error) {
	var ret Return
	err := a.Execute(ctx, param).Get(ctx, &ret)
	return ret, err
}

// Execute asynchnronously executes the activity and returns a promise.
func (a Activity[Param, Return]) Execute(ctx workflow.Context, param Param) workflow.Future {
	return workflow.ExecuteActivity(workflow.WithWorkflowNamespace(workflow.WithTaskQueue(ctx, a.queue.name), a.queue.namespace.name), a.Name, param)
}
