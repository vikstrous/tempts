package tempts

import (
	"context"
	"fmt"
	"reflect"

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
	_, ok := v.activitiesValidated[a.activityName]
	if ok {
		return fmt.Errorf("duplicate activtity name %s for queue %s", a.activityName, q.name)
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
	panicIfNotStruct[Param]("NewActivity")
	panicIfNotStruct[Return]("NewActivity")
	q.registerActivity(name, (func(context.Context, Param) (Return, error))(nil))
	return Activity[Param, Return]{Name: name, queue: q}
}

func panicIfNotStruct[Param any](funcName string) {
	paramType := reflect.TypeOf((*Param)(nil)).Elem()
	if paramType.Kind() == reflect.Ptr {
		paramType = paramType.Elem()
	}
	if paramType.Kind() != reflect.Struct {
		panic(fmt.Sprintf("%s requires a struct or pointer to struct type parameter, got %v", funcName, paramType.Kind()))
	}
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
	return workflow.ExecuteActivity(workflow.WithTaskQueue(ctx, a.queue.name), a.Name, param)
}
