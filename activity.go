package tstemporal

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

type Activity1[Param any] struct {
	Name  string
	queue *Queue
}

func NewActivity1[Param any](q *Queue, name string) Activity1[Param] {
	q.registerActivity(name, (func(context.Context, Param) error)(nil))
	return Activity1[Param]{Name: name, queue: q}
}

func (a Activity1[Param]) WithImplementation(fn func(context.Context, Param) error) Registerable {
	return &ActivityWithImpl{activityName: a.Name, queue: a.queue, fn: fn}
}

func (a Activity1[Param]) Run(ctx workflow.Context, i1 Param) error {
	return a.Execute(ctx, i1).Get(ctx, nil)
}

func (a Activity1[Param]) Execute(ctx workflow.Context, i1 Param) workflow.Future {
	return workflow.ExecuteActivity(workflow.WithWorkflowNamespace(workflow.WithTaskQueue(ctx, a.queue.name), a.queue.namespace.name), a.Name, i1)
}

func (a Activity1[Param]) Register(w worker.ActivityRegistry, fn func(context.Context, Param) error) {
	w.RegisterActivityWithOptions(fn, activity.RegisterOptions{Name: a.Name})
}

type Activity1R[Param, Return any] struct {
	Name  string
	queue *Queue
}

func NewActivity1R[Param, Return any](q *Queue, name string) Activity1R[Param, Return] {
	q.registerActivity(name, (func(context.Context, Param) (Return, error))(nil))
	return Activity1R[Param, Return]{Name: name, queue: q}
}

func (a Activity1R[Param, Return]) WithImplementation(fn func(context.Context, Param) (Return, error)) Registerable {
	return &ActivityWithImpl{activityName: a.Name, queue: a.queue, fn: fn}
}

func (a Activity1R[Param, Return]) Run(ctx workflow.Context, i1 Param) (Return, error) {
	var ret Return
	err := a.Execute(ctx, i1).Get(ctx, &ret)
	return ret, err
}

func (a Activity1R[Param, Return]) Execute(ctx workflow.Context, i1 Param) workflow.Future {
	return workflow.ExecuteActivity(workflow.WithWorkflowNamespace(workflow.WithTaskQueue(ctx, a.queue.name), a.queue.namespace.name), a.Name, i1)
}

func (a Activity1R[Param, Return]) Register(w worker.ActivityRegistry, fn func(context.Context, Param) (Return, error)) {
	w.RegisterActivityWithOptions(fn, activity.RegisterOptions{Name: a.Name})
}
