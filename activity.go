package tempts

import (
	"context"
	"fmt"

	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"
)

type ActivityWithImpl struct {
	activityName string
	queue        *Queue
	fn           any
}

func (a ActivityWithImpl) register(ar Registry) {
	ar.RegisterActivityWithOptions(a.fn, activity.RegisterOptions{Name: a.activityName})
}

func (a ActivityWithImpl) validate(q *Queue, v *ValidationState) error {
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

type Activity[Param, Return any] struct {
	Name  string
	queue *Queue
}

func NewActivity[Param, Return any](q *Queue, name string) Activity[Param, Return] {
	q.registerActivity(name, (func(context.Context, Param) (Return, error))(nil))
	return Activity[Param, Return]{Name: name, queue: q}
}

func (a Activity[Param, Return]) WithImplementation(fn func(context.Context, Param) (Return, error)) *ActivityWithImpl {
	return &ActivityWithImpl{activityName: a.Name, queue: a.queue, fn: fn}
}

func (a Activity[Param, Return]) Run(ctx workflow.Context, param Param) (Return, error) {
	var ret Return
	err := a.Execute(ctx, param).Get(ctx, &ret)
	return ret, err
}

func (a Activity[Param, Return]) Execute(ctx workflow.Context, param Param) workflow.Future {
	return workflow.ExecuteActivity(workflow.WithWorkflowNamespace(workflow.WithTaskQueue(ctx, a.queue.name), a.queue.namespace.name), a.Name, param)
}

func (a Activity[Param, Return]) Register(w worker.ActivityRegistry, fn func(context.Context, Param) (Return, error)) {
	w.RegisterActivityWithOptions(fn, activity.RegisterOptions{Name: a.Name})
}
