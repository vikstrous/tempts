package tempts

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.temporal.io/api/serviceerror"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"
)

// WorkflowWithImpl is a temporary struct that implements Registerable. It's meant to be passed into `tempts.NewWorker`.
type WorkflowWithImpl[Param any, Return any] struct {
	workflowName string
	queue        Queue
	fn           any
}

// Name returns the name of the workflow being implemented.
func (w WorkflowWithImpl[Param, Return]) Name() string {
	return w.workflowName
}

func (w WorkflowWithImpl[Param, Return]) register(ar worker.Registry) {
	ar.RegisterWorkflowWithOptions(w.fn, workflow.RegisterOptions{Name: w.workflowName})
}

func (w WorkflowWithImpl[Param, Return]) validate(q *Queue, v *validationState) error {
	if w.queue.name != q.name {
		return fmt.Errorf("workflow for queue %s can't be registered on worker with queue %s", w.queue.name, q.name)
	}
	_, ok := v.workflowsValidated[w.workflowName]
	if ok {
		return fmt.Errorf("duplicate activtity name %s for queue %s", w.workflowName, q.name)
	}
	v.workflowsValidated[w.workflowName] = struct{}{}
	return nil
}

type testEnvironment interface {
	ExecuteWorkflow(workflowFn interface{}, args ...interface{})
	GetWorkflowResult(valuePtr interface{}) error
	GetWorkflowError() error
}

// ExecuteInTest executes the given workflow implementation in a unit test and returns the output of the workflow.
func (w WorkflowWithImpl[Param, Return]) ExecuteInTest(e testEnvironment, p Param) (Return, error) {
	e.ExecuteWorkflow(w.fn, p)
	var ret Return
	err := e.GetWorkflowResult(&ret)
	if err != nil {
		return ret, err
	}
	return ret, e.GetWorkflowError()
}

// WorkflowDeclaration always contains Workflow but doesn't have type parameters, so it can be passed into non-generic functions.
type WorkflowDeclaration interface {
	Name() string
	getQueue() *Queue
}

// Workflow is used for interacting with workflows in a safe way that takes into account the input and output types, queue name and other properties.
// Workflows are resumable functions registered on workers that execute activities.
type Workflow[Param any, Return any] struct {
	name  string
	queue *Queue
}

// NewWorkflow declares the existence of a workflow on a given queue with a given name.
func NewWorkflow[
	Param any,
	Return any,
](queue *Queue, name string,
) Workflow[Param, Return] {
	queue.registerWorkflow(name, (func(context.Context, Param) (Return, error))(nil))
	return Workflow[Param, Return]{
		name:  name,
		queue: queue,
	}
}

// Name returns the name of the workflow.
func (w Workflow[Param, Return]) Name() string {
	return w.name
}

// getQueue returns the queue for the workflow
func (w Workflow[Param, Return]) getQueue() *Queue {
	return w.queue
}

// WithImplementation should be called to create the parameters for NewWorker(). It declares which function implements the workflow.
func (w Workflow[Param, Return]) WithImplementation(fn func(workflow.Context, Param) (Return, error)) *WorkflowWithImpl[Param, Return] {
	return &WorkflowWithImpl[Param, Return]{workflowName: w.name, queue: *w.queue, fn: func(ctx workflow.Context, param Param) (Return, error) {
		// Set a default timeout so if a workflow doesn't need to customize it, it doesn't have to call this function.
		ctx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
			StartToCloseTimeout: time.Second * 10,
		})
		return fn(ctx, param)
	}}
}

// Run executes the workflow and synchronously returns the output.
func (w Workflow[Param, Return]) Run(ctx context.Context, temporalClient *Client, opts client.StartWorkflowOptions, param Param) (Return, error) {
	var ret Return
	r, err := w.Execute(ctx, temporalClient, opts, param)
	if err != nil {
		return ret, err
	}
	err = r.Get(ctx, &ret)
	return ret, err
}

// Execute asynchnronously executes the workflow and returns a promise.
func (w Workflow[Param, Return]) Execute(ctx context.Context, temporalClient *Client, opts client.StartWorkflowOptions, param Param) (client.WorkflowRun, error) {
	opts.TaskQueue = w.queue.name
	return temporalClient.Client.ExecuteWorkflow(ctx, opts, w.name, param)
}

// Run executes the workflow from another parent workflow and synchronously returns the output.
func (w Workflow[Param, Return]) RunChild(ctx workflow.Context, opts workflow.ChildWorkflowOptions, param Param) (Return, error) {
	var ret Return
	err := w.ExecuteChild(ctx, opts, param).Get(ctx, &ret)
	if err != nil {
		return ret, err
	}
	return ret, nil
}

// Execute asynchnronously executes the workflow from another parent workflow and returns a promise.
func (w Workflow[Param, Return]) ExecuteChild(ctx workflow.Context, opts workflow.ChildWorkflowOptions, param Param) workflow.ChildWorkflowFuture {
	opts.TaskQueue = w.queue.name
	return workflow.ExecuteChildWorkflow(workflow.WithChildOptions(ctx, opts), w.name, param)
}

// SetSchedule creates or updates the schedule to match the given definition.
// WARNING:
// This feature is not as seamless as it could be because of the complex API exposed by temporal.
// In some cases, when the schedule has been modified in some non-updateable way, this method can't update the schedule and it returns an error.
func (w Workflow[Param, Return]) SetSchedule(ctx context.Context, temporalClient *Client, opts client.ScheduleOptions, param Param) error {
	return setSchedule(ctx, temporalClient, opts, w.name, w.queue, []any{param})
}

func setSchedule(ctx context.Context, temporalClient *Client, opts client.ScheduleOptions, workflowName string, queue *Queue, args []any) error {
	if opts.Action == nil {
		opts.Action = &client.ScheduleWorkflowAction{}
	}
	a, ok := opts.Action.(*client.ScheduleWorkflowAction)
	if !ok {
		return fmt.Errorf("opts.Action is %T, not *client.ScheduleWorkflowAction", opts.Action)
	}
	a.Workflow = workflowName
	a.TaskQueue = queue.name
	a.Args = args
	opts.Action = a

	s := temporalClient.Client.ScheduleClient().GetHandle(ctx, opts.ID)
	info, err := s.Describe(ctx)
	if err != nil {
		var notFound *serviceerror.NotFound
		if !errors.As(err, &notFound) {
			return err
		}
		_, err = temporalClient.Client.ScheduleClient().Create(ctx, opts)
		if err != nil {
			return err
		}
		return nil
	}
	// If not equal, error because we can't make them match?
	// Don't compare if neither is set
	if !(info.Memo == nil && len(opts.Memo) == 0) && !info.Memo.Equal(opts.Memo) {
		// TODO: re-create the schedule in this case?
		return fmt.Errorf("provided memo %s doesn't match schedule memo %s and there's no way to fix this without re-creating the schedule", opts.Memo, info.Memo)
	}
	if info.Schedule.State.Note != opts.Note {
		// TODO: re-create the schedule in this case?
		return fmt.Errorf("provided note %s doesn't match schedule note %s and there's no way to fix this without re-creating the schedule", opts.Note, info.Schedule.State.Note)
	}
	// Warning: comparing search attributes doesn't seem to work because temporal craetes its own even when none are provided, so it's not safe to simply compare them.
	// if !info.SearchAttributes.Equal(opts.SearchAttributes) {
	// 	// TODO: re-create the schedule in this case?
	// 	return fmt.Errorf("provided search attributes %s doesn't match schedule search attributes %s and there's no way to fix this without re-creating the schedule", opts.SearchAttributes, info.SearchAttributes)
	// }

	// Update anything we can
	err = s.Update(ctx, client.ScheduleUpdateOptions{
		DoUpdate: func(input client.ScheduleUpdateInput) (*client.ScheduleUpdate, error) {
			s := input.Description.Schedule
			s.Action = opts.Action
			s.Spec = &opts.Spec
			s.State.Paused = opts.Paused
			s.Policy = &client.SchedulePolicies{
				Overlap:        opts.Overlap,
				CatchupWindow:  opts.CatchupWindow,
				PauseOnFailure: opts.PauseOnFailure,
			}
			return &client.ScheduleUpdate{
				Schedule: &s,
			}, nil
		},
	})
	if err != nil {
		return err
	}
	return nil
}
