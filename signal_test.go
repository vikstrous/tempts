package tempts

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/client"
)

// signalWithStartCapture is a minimal mock that captures the workflowArgs
// passed to SignalWithStartWorkflow. Only this method is implemented; calling
// any other client.Client method will panic (which is fine for these tests).
type signalWithStartCapture struct {
	client.Client
	capturedWorkflowArgs []any
}

func (c *signalWithStartCapture) SignalWithStartWorkflow(
	ctx context.Context, workflowID string, signalName string,
	signalArg any, options client.StartWorkflowOptions,
	workflow any, workflowArgs ...any,
) (client.WorkflowRun, error) {
	c.capturedWorkflowArgs = workflowArgs
	return nil, nil
}

type signalTestParams struct {
	Name string
	Age  int
}

type signalTestResult struct {
	Value string
}

type signalTestSignalParam struct {
	Message string
}

func TestSignalWithStartNonPositional(t *testing.T) {
	q := NewQueue("signal-test-q-nonpositional")
	wf := NewWorkflow[signalTestParams, signalTestResult](q, "signal-test-wf-nonpositional")
	sig := NewWorkflowSignal[signalTestSignalParam](&wf, "test-signal")

	mock := &signalWithStartCapture{}
	c := &Client{Client: mock, namespace: "default"}

	_, err := sig.SignalWithStart(
		context.Background(), c,
		client.StartWorkflowOptions{ID: "test-wf-id"},
		signalTestParams{Name: "Alice", Age: 30},
		signalTestSignalParam{Message: "hello"},
	)
	require.NoError(t, err)

	// Non-positional: the struct is passed as a single argument
	require.Len(t, mock.capturedWorkflowArgs, 1, "non-positional should pass struct as single arg")
}

func TestSignalWithStartPositional(t *testing.T) {
	q := NewQueue("signal-test-q-positional")
	wf := NewWorkflowPositional[signalTestParams, signalTestResult](q, "signal-test-wf-positional")
	sig := NewWorkflowSignal[signalTestSignalParam](&wf, "test-signal")

	mock := &signalWithStartCapture{}
	c := &Client{Client: mock, namespace: "default"}

	_, err := sig.SignalWithStart(
		context.Background(), c,
		client.StartWorkflowOptions{ID: "test-wf-id"},
		signalTestParams{Name: "Alice", Age: 30},
		signalTestSignalParam{Message: "hello"},
	)
	require.NoError(t, err)

	// Positional: the struct fields should be decomposed into separate arguments
	require.Len(t, mock.capturedWorkflowArgs, 2, "positional should decompose struct into individual field args")
	require.Equal(t, "Alice", mock.capturedWorkflowArgs[0])
	require.Equal(t, 30, mock.capturedWorkflowArgs[1])
}
