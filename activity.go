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

type Activity0 struct {
	Name  string
	queue *Queue
}

func NewActivity0(q *Queue, name string) Activity0 {
	q.registerActivity(name, (func(context.Context) error)(nil))
	return Activity0{Name: name, queue: q}
}

func (a Activity0) WithImplementation(fn func(context.Context) error) *ActivityWithImpl {
	return &ActivityWithImpl{activityName: a.Name, queue: a.queue, fn: fn}
}

func (a Activity0) Run(ctx workflow.Context) error {
	return a.Execute(ctx).Get(ctx, nil)
}

func (a Activity0) Execute(ctx workflow.Context) workflow.Future {
	return workflow.ExecuteActivity(workflow.WithWorkflowNamespace(workflow.WithTaskQueue(ctx, a.queue.name), a.queue.namespace.name), a.Name)
}

func (a Activity0) Register(w worker.ActivityRegistry, fn func(context.Context) error) {
	w.RegisterActivityWithOptions(fn, activity.RegisterOptions{Name: a.Name})
}

type Activity0R[Return any] struct {
	Name  string
	queue *Queue
}

func NewActivity0R[Return any](q *Queue, name string) Activity0R[Return] {
	q.registerActivity(name, (func(context.Context) (Return, error))(nil))
	return Activity0R[Return]{Name: name, queue: q}
}

func (a Activity0R[Return]) WithImplementation(fn func(context.Context) (Return, error)) *ActivityWithImpl {
	return &ActivityWithImpl{activityName: a.Name, queue: a.queue, fn: fn}
}

func (a Activity0R[Return]) Run(ctx workflow.Context) (Return, error) {
	var ret Return
	err := a.Execute(ctx).Get(ctx, &ret)
	return ret, err
}

func (a Activity0R[Return]) Execute(ctx workflow.Context) workflow.Future {
	return workflow.ExecuteActivity(workflow.WithWorkflowNamespace(workflow.WithTaskQueue(ctx, a.queue.name), a.queue.namespace.name), a.Name)
}

func (a Activity0R[Return]) Register(w worker.ActivityRegistry, fn func(context.Context) (Return, error)) {
	w.RegisterActivityWithOptions(fn, activity.RegisterOptions{Name: a.Name})
}

type Activity1[Param any] struct {
	Name  string
	queue *Queue
}

func NewActivity1[Param any](q *Queue, name string) Activity1[Param] {
	q.registerActivity(name, (func(context.Context, Param) error)(nil))
	return Activity1[Param]{Name: name, queue: q}
}

func (a Activity1[Param]) WithImplementation(fn func(context.Context, Param) error) *ActivityWithImpl {
	return &ActivityWithImpl{activityName: a.Name, queue: a.queue, fn: fn}
}

func (a Activity1[Param]) Run(ctx workflow.Context, param Param) error {
	return a.Execute(ctx, param).Get(ctx, nil)
}

func (a Activity1[Param]) Execute(ctx workflow.Context, param Param) workflow.Future {
	return workflow.ExecuteActivity(workflow.WithWorkflowNamespace(workflow.WithTaskQueue(ctx, a.queue.name), a.queue.namespace.name), a.Name, param)
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

func (a Activity1R[Param, Return]) WithImplementation(fn func(context.Context, Param) (Return, error)) *ActivityWithImpl {
	return &ActivityWithImpl{activityName: a.Name, queue: a.queue, fn: fn}
}

func (a Activity1R[Param, Return]) Run(ctx workflow.Context, param Param) (Return, error) {
	var ret Return
	err := a.Execute(ctx, param).Get(ctx, &ret)
	return ret, err
}

func (a Activity1R[Param, Return]) Execute(ctx workflow.Context, param Param) workflow.Future {
	return workflow.ExecuteActivity(workflow.WithWorkflowNamespace(workflow.WithTaskQueue(ctx, a.queue.name), a.queue.namespace.name), a.Name, param)
}

func (a Activity1R[Param, Return]) Register(w worker.ActivityRegistry, fn func(context.Context, Param) (Return, error)) {
	w.RegisterActivityWithOptions(fn, activity.RegisterOptions{Name: a.Name})
}
