package tempts

import (
	"errors"
	"fmt"

	"go.temporal.io/sdk/converter"
)

// panicOnDecodeError panics if the error wraps converter.ErrUnableToDecode,
// the sentinel used by all Temporal payload converters (JSON, proto, proto-JSON)
// when deserialization fails. This is called from the WithImplementation wrapper,
// which runs outside the user's workflow function scope. A panic here causes a
// workflow task failure (not a workflow execution failure), matching the behavior
// of non-determinism errors: the workflow stays open and the task is retried
// until a fixed worker is deployed.
func panicOnDecodeError(err error) {
	if err != nil && errors.Is(err, converter.ErrUnableToDecode) {
		panic(fmt.Sprintf("payload decode error: %v", err))
	}
}
