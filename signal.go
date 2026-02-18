package tempts

import (
	"context"

	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/workflow"
)

// WorkflowSignal is a type-safe signal scoped to a specific workflow.
// It captures the workflow's parameter and return types to enable type-safe SignalWithStart.
type WorkflowSignal[WorkflowParam, WorkflowReturn, SignalParam any] struct {
	workflow *Workflow[WorkflowParam, WorkflowReturn]
	name     string
}

// NewWorkflowSignal declares a signal for a specific workflow.
// The signal is scoped to this workflow, enabling type-safe SignalWithStart operations.
//
// The signalName is used as the Temporal channel name (via workflow.GetSignalChannel)
// and must match when sending signals from external code.
func NewWorkflowSignal[SignalParam any, WorkflowParam, WorkflowReturn any](
	w *Workflow[WorkflowParam, WorkflowReturn],
	signalName string,
) *WorkflowSignal[WorkflowParam, WorkflowReturn, SignalParam] {
	validateName(signalName)
	panicIfNotStruct[SignalParam]("NewWorkflowSignal")
	return &WorkflowSignal[WorkflowParam, WorkflowReturn, SignalParam]{
		workflow: w,
		name:     signalName,
	}
}

// Name returns the signal name, which is also the underlying Temporal channel name.
func (s *WorkflowSignal[WP, WR, SP]) Name() string {
	return s.name
}

// Receive blocks until a signal is received and returns the typed value.
// For handling multiple signals, use AddToSelector instead.
func (s *WorkflowSignal[WP, WR, SP]) Receive(ctx workflow.Context) SP {
	ch := workflow.GetSignalChannel(ctx, s.name)
	var param SP
	ch.Receive(ctx, &param)
	return param
}

// TryReceive attempts to receive a signal without blocking.
// Returns the value and whether a value was received.
// For handling multiple signals, use AddToSelector instead.
func (s *WorkflowSignal[WP, WR, SP]) TryReceive(ctx workflow.Context) (value SP, ok bool) {
	ch := workflow.GetSignalChannel(ctx, s.name)
	ok = ch.ReceiveAsync(&value)
	return value, ok
}

// AddToSelector adds this signal to a workflow.Selector with a typed callback.
// This is the recommended way to handle multiple signals in a select-style pattern.
//
// Example:
//
//	selector := workflow.NewSelector(ctx)
//	signal1.AddToSelector(ctx, selector, func(val Signal1Param) {
//	    // handle signal1
//	})
//	signal2.AddToSelector(ctx, selector, func(val Signal2Param) {
//	    // handle signal2
//	})
//	selector.Select(ctx)
func (s *WorkflowSignal[WP, WR, SP]) AddToSelector(ctx workflow.Context, selector workflow.Selector, fn func(SP)) workflow.Selector {
	ch := workflow.GetSignalChannel(ctx, s.name)
	return selector.AddReceive(ch, func(c workflow.ReceiveChannel, more bool) {
		var param SP
		c.Receive(ctx, &param)
		fn(param)
	})
}

// Signal sends the signal to an already-running workflow.
func (s *WorkflowSignal[WP, WR, SP]) Signal(
	ctx context.Context,
	temporalClient *Client,
	workflowID, runID string,
	param SP,
) error {
	return temporalClient.Client.SignalWorkflow(ctx, workflowID, runID, s.name, param)
}

// SignalWithStart atomically starts the workflow if it doesn't exist, or signals it if it does.
// This is useful for ensuring exactly-once workflow creation with an initial signal.
func (s *WorkflowSignal[WP, WR, SP]) SignalWithStart(
	ctx context.Context,
	temporalClient *Client,
	opts client.StartWorkflowOptions,
	workflowParam WP,
	signalParam SP,
) (client.WorkflowRun, error) {
	opts.TaskQueue = s.workflow.queue.name
	return temporalClient.Client.SignalWithStartWorkflow(
		ctx,
		opts.ID,
		s.name,
		signalParam,
		opts,
		s.workflow.name,
		workflowParam,
	)
}
