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
func NewWorkflowSignal[SignalParam any, WorkflowParam, WorkflowReturn any](
	w *Workflow[WorkflowParam, WorkflowReturn],
	signalName string,
) *WorkflowSignal[WorkflowParam, WorkflowReturn, SignalParam] {
	panicIfNotStruct[SignalParam]("NewWorkflowSignal")
	return &WorkflowSignal[WorkflowParam, WorkflowReturn, SignalParam]{
		workflow: w,
		name:     signalName,
	}
}

// Name returns the name of the signal.
func (s *WorkflowSignal[WP, WR, SP]) Name() string {
	return s.name
}

// SetHandler registers a callback function to handle this signal in a workflow.
// The callback is invoked each time the signal is received.
func (s *WorkflowSignal[WP, WR, SP]) SetHandler(ctx workflow.Context, fn func(ctx workflow.Context, param SP)) {
	ch := workflow.GetSignalChannel(ctx, s.name)
	workflow.Go(ctx, func(ctx workflow.Context) {
		for {
			var param SP
			ch.Receive(ctx, &param)
			fn(ctx, param)
		}
	})
}

// GetChannel returns a typed receive channel for this signal.
// Use this for selector-based signal handling patterns.
func (s *WorkflowSignal[WP, WR, SP]) GetChannel(ctx workflow.Context) *ReceiveChannel[SP] {
	return &ReceiveChannel[SP]{
		ch: workflow.GetSignalChannel(ctx, s.name),
	}
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

// ReceiveChannel is a type-safe wrapper around workflow.ReceiveChannel.
type ReceiveChannel[T any] struct {
	ch workflow.ReceiveChannel
}

// Receive blocks until a signal is received and returns the typed value.
func (c *ReceiveChannel[T]) Receive(ctx workflow.Context) (T, bool) {
	var value T
	more := c.ch.Receive(ctx, &value)
	return value, more
}

// ReceiveAsync attempts to receive a signal without blocking.
// Returns the value, whether a value was received, and whether the channel has more values.
func (c *ReceiveChannel[T]) ReceiveAsync() (value T, ok bool) {
	ok = c.ch.ReceiveAsync(&value)
	return value, ok
}

// ReceiveAsyncWithMoreFlag attempts to receive a signal without blocking.
// Returns the value, whether a value was received, and whether the channel has more values.
func (c *ReceiveChannel[T]) ReceiveAsyncWithMoreFlag() (value T, ok bool, more bool) {
	ok, more = c.ch.ReceiveAsyncWithMoreFlag(&value)
	return value, ok, more
}

// Raw returns the underlying workflow.ReceiveChannel for use with workflow.Selector.
// Example:
//
//	selector := workflow.NewSelector(ctx)
//	selector.AddReceive(signal.GetChannel(ctx).Raw(), func(c workflow.ReceiveChannel, more bool) {
//	    value, _ := signal.GetChannel(ctx).Receive(ctx)
//	    // handle value
//	})
func (c *ReceiveChannel[T]) Raw() workflow.ReceiveChannel {
	return c.ch
}
