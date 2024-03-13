package tempts

import (
	"context"
	"errors"
	"fmt"

	"go.temporal.io/api/serviceerror"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"
)

type WorkflowWithImpl struct {
	workflowName string
	queue        Queue
	fn           any
}

func (w WorkflowWithImpl) Name() string {
	return w.workflowName
}

func (w WorkflowWithImpl) register(ar worker.Registry) {
	ar.RegisterWorkflowWithOptions(w.fn, workflow.RegisterOptions{Name: w.workflowName})
}

func (w WorkflowWithImpl) validate(q *Queue, v *ValidationState) error {
	if w.queue.name != q.name {
		return fmt.Errorf("workflow for queue %s can't be registered on worker with queue %s", w.queue.name, q.name)
	}
	if w.queue.namespace.name != q.namespace.name {
		return fmt.Errorf("workflow for namespace %s can't be registered on worker with namespace %s", w.queue.namespace.name, q.namespace.name)
	}
	_, ok := v.workflowsValidated[w.workflowName]
	if ok {
		return fmt.Errorf("duplicate activtity name %s for queue %s and namespace %s", w.workflowName, q.name, q.namespace.name)
	}
	v.workflowsValidated[w.workflowName] = struct{}{}
	return nil
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

	if temporalClient.namespace != queue.namespace.name {
		return fmt.Errorf("attempting to set a schedule for a workflow in %s on the wrong namespace: %s", queue.namespace.name, temporalClient.namespace)
	}

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

type Workflow[Param any, Return any] struct {
	Name  string
	queue *Queue
}

func NewWorkflow[
	Param any,
	Return any,
](queue *Queue, name string,
) Workflow[Param, Return] {
	queue.registerWorkflow(name, (func(context.Context, Param) (Return, error))(nil))
	return Workflow[Param, Return]{
		Name:  name,
		queue: queue,
	}
}

func (w Workflow[Param, Return]) WithImplementation(fn func(workflow.Context, Param) (Return, error)) *WorkflowWithImpl {
	return &WorkflowWithImpl{workflowName: w.Name, queue: *w.queue, fn: fn}
}

func (w Workflow[Param, Return]) Run(ctx context.Context, temporalClient *Client, opts client.StartWorkflowOptions, param Param) (Return, error) {
	var ret Return
	r, err := w.Execute(ctx, temporalClient, opts, param)
	if err != nil {
		return ret, err
	}
	err = r.Get(ctx, &ret)
	return ret, err
}

func (w Workflow[Param, Return]) Execute(ctx context.Context, temporalClient *Client, opts client.StartWorkflowOptions, param Param) (client.WorkflowRun, error) {
	opts.TaskQueue = w.queue.name
	if w.queue.namespace.name != temporalClient.namespace {
		// The user must provide a client that's connected to the right namespace to be able to start this workflow.
		return nil, fmt.Errorf("wrong namespace for client %s vs workflow %s", temporalClient.namespace, w.queue.namespace.name)
	}
	return temporalClient.Client.ExecuteWorkflow(ctx, opts, w.Name, param)
}

func (w Workflow[Param, Return]) RunChild(ctx workflow.Context, opts workflow.ChildWorkflowOptions, param Param) (Return, error) {
	var ret Return
	err := w.ExecuteChild(ctx, opts, param).Get(ctx, &ret)
	if err != nil {
		return ret, err
	}
	return ret, nil
}

func (w Workflow[Param, Return]) ExecuteChild(ctx workflow.Context, opts workflow.ChildWorkflowOptions, param Param) workflow.ChildWorkflowFuture {
	opts.TaskQueue = w.queue.name
	opts.Namespace = w.queue.namespace.name
	return workflow.ExecuteChildWorkflow(workflow.WithChildOptions(ctx, opts), w.Name, param)
}

func (w Workflow[Param, Return]) SetSchedule(ctx context.Context, temporalClient *Client, opts client.ScheduleOptions, param Param) error {
	return setSchedule(ctx, temporalClient, opts, w.Name, w.queue, []any{param})
}
