package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/vikstrous/tempts"
	enumspb "go.temporal.io/api/enums/v1"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/testsuite"
	"go.temporal.io/sdk/worker"
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

// --- Activity with switchable JSON decode (for integration test) ---

// switchableDecodeFailure controls whether SwitchableResult.UnmarshalJSON fails.
var switchableDecodeFailure atomic.Bool

type SwitchableParams struct {
	Input string
}

type SwitchableResult struct {
	Value string
}

func (s *SwitchableResult) UnmarshalJSON(data []byte) error {
	if switchableDecodeFailure.Load() {
		return fmt.Errorf("switchable JSON decode error")
	}
	type Alias SwitchableResult
	var a Alias
	if err := json.Unmarshal(data, &a); err != nil {
		return err
	}
	*s = SwitchableResult(a)
	return nil
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

// --- Integration test against real Temporal cluster ---

var queueSwitchable = tempts.NewQueue("switchable_decode")

var activityTypeSwitchable = tempts.NewActivity[SwitchableParams, SwitchableResult](queueSwitchable, "switchable_activity")

type SwitchableWorkflowParams struct {
	Input string
}

type SwitchableWorkflowResult struct {
	Output string
}

var workflowTypeSwitchable = tempts.NewWorkflow[SwitchableWorkflowParams, SwitchableWorkflowResult](queueSwitchable, "switchable_workflow")

func activitySwitchable(ctx context.Context, params SwitchableParams) (SwitchableResult, error) {
	return SwitchableResult{Value: "hello " + params.Input}, nil
}

func workflowSwitchable(ctx workflow.Context, params SwitchableWorkflowParams) (SwitchableWorkflowResult, error) {
	result, err := activityTypeSwitchable.Run(ctx, SwitchableParams{Input: params.Input})
	if err != nil {
		return SwitchableWorkflowResult{}, fmt.Errorf("activity failed: %w", err)
	}
	return SwitchableWorkflowResult{Output: result.Value}, nil
}

func startSwitchableWorker(t *testing.T, ctx context.Context, c *tempts.Client) {
	t.Helper()
	wrk, err := tempts.NewWorker(queueSwitchable, []tempts.Registerable{
		activityTypeSwitchable.WithImplementation(activitySwitchable),
		workflowTypeSwitchable.WithImplementation(workflowSwitchable),
	})
	if err != nil {
		t.Fatal(err)
	}
	go func() {
		if err := wrk.Run(ctx, c, worker.Options{}); err != nil {
			t.Logf("worker stopped: %v", err)
		}
	}()
}

// TestIntegrationDecodeErrorBlocksThenRecovers verifies against a real Temporal
// cluster that:
//  1. A decode error causes repeated WorkflowTaskFailed events (workflow stays Running).
//  2. After the decode error is fixed, the workflow recovers and completes.
func TestIntegrationDecodeErrorBlocksThenRecovers(t *testing.T) {
	c, err := tempts.Dial(client.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()

	// Phase 1: Start with broken decode
	switchableDecodeFailure.Store(true)

	ctx1, cancel1 := context.WithCancel(context.Background())
	startSwitchableWorker(t, ctx1, c)

	run, err := workflowTypeSwitchable.Execute(context.Background(), c, client.StartWorkflowOptions{}, SwitchableWorkflowParams{Input: "world"})
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("Workflow started: ID=%s RunID=%s", run.GetID(), run.GetRunID())

	// Wait for the activity to complete and workflow task to fail a few times
	time.Sleep(5 * time.Second)

	// Verify workflow is still Running
	desc, err := c.Client.DescribeWorkflowExecution(context.Background(), run.GetID(), run.GetRunID())
	if err != nil {
		t.Fatal(err)
	}
	if desc.WorkflowExecutionInfo.Status != enumspb.WORKFLOW_EXECUTION_STATUS_RUNNING {
		t.Fatalf("expected workflow to be Running, got %v", desc.WorkflowExecutionInfo.Status)
	}
	t.Log("Phase 1: Workflow is Running (blocked on decode error)")

	// Verify history contains WorkflowTaskFailed events
	iter := c.Client.GetWorkflowHistory(context.Background(), run.GetID(), run.GetRunID(), false, enumspb.HISTORY_EVENT_FILTER_TYPE_ALL_EVENT)
	taskFailedCount := 0
	activityCompleted := false
	for iter.HasNext() {
		event, err := iter.Next()
		if err != nil {
			t.Fatal(err)
		}
		if event.GetEventType() == enumspb.EVENT_TYPE_WORKFLOW_TASK_FAILED {
			taskFailedCount++
		}
		if event.GetEventType() == enumspb.EVENT_TYPE_ACTIVITY_TASK_COMPLETED {
			activityCompleted = true
		}
	}
	if !activityCompleted {
		t.Fatal("expected ActivityTaskCompleted in history")
	}
	if taskFailedCount == 0 {
		t.Fatal("expected at least one WorkflowTaskFailed in history")
	}
	t.Logf("Phase 1: History has %d WorkflowTaskFailed events and ActivityTaskCompleted", taskFailedCount)

	// Phase 2: Stop the broken worker, fix decode, start a new worker
	cancel1()
	time.Sleep(1 * time.Second) // let old worker drain

	switchableDecodeFailure.Store(false)
	t.Log("Phase 2: Decode fixed, starting new worker")

	ctx2, cancel2 := context.WithCancel(context.Background())
	defer cancel2()
	startSwitchableWorker(t, ctx2, c)

	// Wait for the workflow to complete
	var result SwitchableWorkflowResult
	err = run.Get(context.Background(), &result)
	if err != nil {
		t.Fatalf("expected workflow to complete after fix, got error: %v", err)
	}
	t.Logf("Phase 2: Workflow completed with result: %+v", result)

	if result.Output != "hello world" {
		t.Fatalf("expected output 'hello world', got '%s'", result.Output)
	}

	// Verify workflow is now Completed
	desc, err = c.Client.DescribeWorkflowExecution(context.Background(), run.GetID(), run.GetRunID())
	if err != nil {
		t.Fatal(err)
	}
	if desc.WorkflowExecutionInfo.Status != enumspb.WORKFLOW_EXECUTION_STATUS_COMPLETED {
		t.Fatalf("expected workflow to be Completed, got %v", desc.WorkflowExecutionInfo.Status)
	}
	t.Log("Phase 2: Workflow status is Completed")
}
