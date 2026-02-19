package tempts

import (
	"errors"
	"fmt"

	"go.temporal.io/sdk/temporal"
)

// DecodeError wraps an error that occurred during deserialization of an activity
// or child workflow result. When a workflow function returns a DecodeError
// (directly or wrapped via fmt.Errorf %w), the WithImplementation wrapper panics,
// causing a workflow task failure instead of a workflow execution failure.
// This matches the behavior of non-determinism errors: the workflow stays open
// and the task is retried until a fixed worker is deployed.
type DecodeError struct {
	err error
}

func (e *DecodeError) Error() string {
	return fmt.Sprintf("decode error: %v", e.err)
}

func (e *DecodeError) Unwrap() error {
	return e.err
}

// panicOnDecodeError panics if the error wraps a DecodeError.
// This is called from the WithImplementation wrapper, which runs outside
// the user's workflow function scope. A panic here causes a workflow task
// failure (not a workflow execution failure), matching the behavior of
// non-determinism errors.
func panicOnDecodeError(err error) {
	var decodeErr *DecodeError
	if errors.As(err, &decodeErr) {
		panic(fmt.Sprintf("payload decode error: %v", err))
	}
}

// isActivityDecodeError returns true if the error from an activity Future.Get()
// is a deserialization failure rather than an activity execution failure.
func isActivityDecodeError(err error) bool {
	if err == nil {
		return false
	}
	var activityErr *temporal.ActivityError
	var canceledErr *temporal.CanceledError
	if errors.As(err, &activityErr) || errors.As(err, &canceledErr) {
		return false
	}
	return true
}

// isChildWorkflowDecodeError returns true if the error from a child workflow
// Future.Get() is a deserialization failure rather than a child workflow execution failure.
func isChildWorkflowDecodeError(err error) bool {
	if err == nil {
		return false
	}
	var childErr *temporal.ChildWorkflowExecutionError
	var canceledErr *temporal.CanceledError
	if errors.As(err, &childErr) || errors.As(err, &canceledErr) {
		return false
	}
	return true
}
