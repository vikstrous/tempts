package tempts

import (
	"context"
	"testing"

	"go.temporal.io/sdk/testsuite"
	"go.temporal.io/sdk/workflow"
)

// Define test types
type TestWorkflowParams struct {
	Input string
}

type TestWorkflowResult struct {
	Output string
}

type TestActivityParams struct {
	Data string
}

type TestActivityResult struct {
	ProcessedData string
}

// Test the creation and execution of a workflow using tempts
func TestCreateWorkflow(t *testing.T) {
	// Create a test queue
	testQueue := NewQueue("test-queue")

	// Define workflow and activity types
	workflowType := NewWorkflow[TestWorkflowParams, TestWorkflowResult](testQueue, "TestWorkflow")
	activityType := NewActivity[TestActivityParams, TestActivityResult](testQueue, "TestActivity")

	// Define workflow implementation
	workflowImpl := func(ctx workflow.Context, params TestWorkflowParams) (TestWorkflowResult, error) {
		// Call activity
		activityResult, err := activityType.Run(ctx, TestActivityParams{Data: params.Input})
		if err != nil {
			return TestWorkflowResult{}, err
		}
		
		return TestWorkflowResult{Output: activityResult.ProcessedData}, nil
	}

	// Define activity implementation
	activityImpl := func(ctx context.Context, params TestActivityParams) (TestActivityResult, error) {
		// Simple processing - append text to input
		return TestActivityResult{ProcessedData: params.Data + " processed"}, nil
	}

	// Create worker with workflow and activity registrations
	worker, err := NewWorker(testQueue, []Registerable{
		workflowType.WithImplementation(workflowImpl),
		activityType.WithImplementation(activityImpl),
	})
	if err != nil {
		t.Fatalf("Failed to create worker: %v", err)
	}

	// Set up test environment
	ts := testsuite.WorkflowTestSuite{}
	ts.SetDisableRegistrationAliasing(true)
	env := ts.NewTestWorkflowEnvironment()
	
	// Register worker with test environment
	worker.Register(env)

	// Execute workflow in test environment
	wf := workflowType.WithImplementation(workflowImpl)
	result, err := wf.ExecuteInTest(env, TestWorkflowParams{Input: "test-input"})
	if err != nil {
		t.Fatalf("Failed to execute workflow: %v", err)
	}

	// Verify result
	expected := "test-input processed"
	if result.Output != expected {
		t.Errorf("Expected output '%s', got '%s'", expected, result.Output)
	}
}

// Test workflow with child workflow
func TestWorkflowWithChild(t *testing.T) {
	// Create a test queue
	testQueue := NewQueue("test-queue-child")

	// Define parent and child workflow types
	parentWorkflowType := NewWorkflow[TestWorkflowParams, TestWorkflowResult](testQueue, "ParentWorkflow")
	childWorkflowType := NewWorkflow[TestWorkflowParams, TestWorkflowResult](testQueue, "ChildWorkflow")

	// Define child workflow implementation
	childWorkflowImpl := func(ctx workflow.Context, params TestWorkflowParams) (TestWorkflowResult, error) {
		return TestWorkflowResult{Output: "Child: " + params.Input}, nil
	}

	// Define parent workflow implementation that calls child workflow
	parentWorkflowImpl := func(ctx workflow.Context, params TestWorkflowParams) (TestWorkflowResult, error) {
		// Call child workflow
		childResult, err := childWorkflowType.RunChild(ctx, workflow.ChildWorkflowOptions{}, 
			TestWorkflowParams{Input: params.Input})
		if err != nil {
			return TestWorkflowResult{}, err
		}
		
		return TestWorkflowResult{Output: "Parent processed: " + childResult.Output}, nil
	}

	// Create worker with workflow registrations
	worker, err := NewWorker(testQueue, []Registerable{
		parentWorkflowType.WithImplementation(parentWorkflowImpl),
		childWorkflowType.WithImplementation(childWorkflowImpl),
	})
	if err != nil {
		t.Fatalf("Failed to create worker: %v", err)
	}

	// Set up test environment
	ts := testsuite.WorkflowTestSuite{}
	ts.SetDisableRegistrationAliasing(true)
	env := ts.NewTestWorkflowEnvironment()
	
	// Register worker with test environment
	worker.Register(env)

	// Execute parent workflow in test environment
	parentWf := parentWorkflowType.WithImplementation(parentWorkflowImpl)
	result, err := parentWf.ExecuteInTest(env, TestWorkflowParams{Input: "nested-test"})
	if err != nil {
		t.Fatalf("Failed to execute workflow: %v", err)
	}

	// Verify result
	expected := "Parent processed: Child: nested-test"
	if result.Output != expected {
		t.Errorf("Expected output '%s', got '%s'", expected, result.Output)
	}
}