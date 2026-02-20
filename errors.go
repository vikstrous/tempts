package tempts

import (
	"errors"
	"fmt"

	"go.temporal.io/sdk/converter"
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

// isDecodeError returns true if the error wraps converter.ErrUnableToDecode,
// which is the sentinel error used by all Temporal payload converters (JSON,
// proto, proto-JSON) when deserialization fails.
func isDecodeError(err error) bool {
	return err != nil && errors.Is(err, converter.ErrUnableToDecode)
}
