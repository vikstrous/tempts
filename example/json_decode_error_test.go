package main

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/vikstrous/tempts"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/testsuite"
	"go.temporal.io/sdk/workflow"
)

// --- Activity with broken JSON decode ---

type BrokenJSONParams struct {
	Input string
}

type BrokenJSONResult struct {
	Value string
}

// UnmarshalJSON always returns an error, simulating a broken JSON decode.
func (b *BrokenJSONResult) UnmarshalJSON(data []byte) error {
	return fmt.Errorf("intentional JSON decode error")
}

var queueBrokenJSON = tempts.NewQueue("broken_json")

var activityTypeBrokenJSON = tempts.NewActivity[BrokenJSONParams, BrokenJSONResult](queueBrokenJSON, "broken_json_activity")

type BrokenJSONWorkflowParams struct {
	Input string
}

type BrokenJSONWorkflowResult struct {
	Output string
}

var workflowTypeBrokenJSON = tempts.NewWorkflow[BrokenJSONWorkflowParams, BrokenJSONWorkflowResult](queueBrokenJSON, "broken_json_workflow")

func activityBrokenJSON(ctx context.Context, params BrokenJSONParams) (BrokenJSONResult, error) {
	return BrokenJSONResult{Value: "hello " + params.Input}, nil
}

// workflowBrokenJSON is a plain workflow — no special panic handling.
// tempts' WithImplementation wrapper detects the DecodeError and panics
// outside this function's scope.
func workflowBrokenJSON(ctx workflow.Context, params BrokenJSONWorkflowParams) (BrokenJSONWorkflowResult, error) {
	result, err := activityTypeBrokenJSON.Run(ctx, BrokenJSONParams{Input: params.Input})
	if err != nil {
		return BrokenJSONWorkflowResult{}, fmt.Errorf("activity failed: %w", err)
	}
	return BrokenJSONWorkflowResult{Output: result.Value}, nil
}

// TestDecodeErrorCausesPanic verifies that a broken UnmarshalJSON on an activity
// result causes the WithImplementation wrapper to panic (producing a PanicError),
// rather than the workflow completing with a normal application error.
//
// In production with the default BlockWorkflow panic policy, this panic causes a
// WorkflowTaskFailed event and the workflow stays Running. The test environment
// surfaces panics as PanicError, so we check for that.
func TestDecodeErrorCausesPanic(t *testing.T) {
	wf := workflowTypeBrokenJSON.WithImplementation(workflowBrokenJSON)
	wrk, err := tempts.NewWorker(queueBrokenJSON, []tempts.Registerable{
		activityTypeBrokenJSON.WithImplementation(activityBrokenJSON),
		wf,
	})
	if err != nil {
		t.Fatal(err)
	}

	ts := testsuite.WorkflowTestSuite{}
	ts.SetDisableRegistrationAliasing(true)
	we := ts.NewTestWorkflowEnvironment()
	wrk.Register(we)

	_, err = wf.ExecuteInTest(we, BrokenJSONWorkflowParams{Input: "world"})
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	// The test environment wraps panics in WorkflowExecutionError -> PanicError.
	// Verify it's a PanicError (not an ApplicationError, which would mean the
	// workflow completed with a normal failure).
	var panicErr *temporal.PanicError
	var appErr *temporal.ApplicationError
	if errors.As(err, &appErr) {
		t.Fatalf("expected PanicError (workflow task failure), got ApplicationError (workflow execution failure): %v", err)
	}
	if !errors.As(err, &panicErr) {
		t.Fatalf("expected PanicError, got: %T: %v", err, err)
	}

	// Verify the panic message contains the decode error context
	if !strings.Contains(panicErr.Error(), "payload decode error") {
		t.Fatalf("expected panic message to contain 'payload decode error', got: %s", panicErr.Error())
	}
	if !strings.Contains(panicErr.Error(), "intentional JSON decode error") {
		t.Fatalf("expected panic message to contain 'intentional JSON decode error', got: %s", panicErr.Error())
	}

	t.Logf("Got expected PanicError: %v", panicErr.Error())
}

// TestNormalActivityErrorNotTreatedAsDecodeError verifies that a real activity
// failure (returning an error from the activity) is NOT wrapped as a DecodeError.
type FailingActivityParams struct {
	Input string
}

type FailingActivityResult struct {
	Value string
}

var queueFailingActivity = tempts.NewQueue("failing_activity")
var activityTypeFailing = tempts.NewActivity[FailingActivityParams, FailingActivityResult](queueFailingActivity, "failing_activity")

type FailingWorkflowParams struct {
	Input string
}
type FailingWorkflowResult struct {
	Output string
}

var workflowTypeFailing = tempts.NewWorkflow[FailingWorkflowParams, FailingWorkflowResult](queueFailingActivity, "failing_workflow")

func activityFailing(ctx context.Context, params FailingActivityParams) (FailingActivityResult, error) {
	return FailingActivityResult{}, fmt.Errorf("activity error: something went wrong")
}

func workflowFailing(ctx workflow.Context, params FailingWorkflowParams) (FailingWorkflowResult, error) {
	result, err := activityTypeFailing.Run(ctx, FailingActivityParams{Input: params.Input})
	if err != nil {
		return FailingWorkflowResult{}, fmt.Errorf("activity failed: %w", err)
	}
	return FailingWorkflowResult{Output: result.Value}, nil
}

func TestNormalActivityErrorNotTreatedAsDecodeError(t *testing.T) {
	wf := workflowTypeFailing.WithImplementation(workflowFailing)
	wrk, err := tempts.NewWorker(queueFailingActivity, []tempts.Registerable{
		activityTypeFailing.WithImplementation(activityFailing),
		wf,
	})
	if err != nil {
		t.Fatal(err)
	}

	ts := testsuite.WorkflowTestSuite{}
	ts.SetDisableRegistrationAliasing(true)
	we := ts.NewTestWorkflowEnvironment()
	wrk.Register(we)

	_, err = wf.ExecuteInTest(we, FailingWorkflowParams{Input: "world"})
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	// A normal activity error should produce an ApplicationError, NOT a PanicError.
	var panicErr *temporal.PanicError
	if errors.As(err, &panicErr) {
		t.Fatalf("normal activity error was treated as decode error (got PanicError): %v", err)
	}

	var appErr *temporal.ApplicationError
	if !errors.As(err, &appErr) {
		t.Fatalf("expected ApplicationError for normal activity failure, got: %T: %v", err, err)
	}

	t.Logf("Got expected ApplicationError: %v", appErr.Error())
}
