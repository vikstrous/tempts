package main

import (
	"testing"

	"github.com/vikstrous/tempts"
	"go.temporal.io/sdk/testsuite"
	"go.temporal.io/sdk/workflow"
)

func TestExample(t *testing.T) {
	wf := workflowTypeFormatAndGreet.WithImplementation(workflowFormatAndGreet)
	wrk, err := tempts.NewWorker(queueMain, []tempts.Registerable{
		activityTypeFormatName.WithImplementation(activityFormatName),
		activityTypeGreet.WithImplementation(activityGreet),
		wf,
		workflowTypeJustGreet.WithImplementation(workflowJustGreet),
	})
	if err != nil {
		t.Fatal(err)
	}

	ts := testsuite.WorkflowTestSuite{}
	ts.SetDisableRegistrationAliasing(true)
	we := ts.NewTestWorkflowEnvironment()
	wrk.Register(we)

	result, err := wf.ExecuteInTest(we, FormatAndGreetParams{Name: "viktor"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Name != "VIKTOR" {
		t.Fatal("Expected VIKTOR, got", result)
	}
}

func TestSignal(t *testing.T) {
	wf := workflowTypeFormatAndGreet.WithImplementation(workflowFormatAndGreet)
	wrk, err := tempts.NewWorker(queueMain, []tempts.Registerable{
		activityTypeFormatName.WithImplementation(activityFormatName),
		activityTypeGreet.WithImplementation(activityGreet),
		wf,
		workflowTypeJustGreet.WithImplementation(workflowJustGreet),
	})
	if err != nil {
		t.Fatal(err)
	}

	ts := testsuite.WorkflowTestSuite{}
	ts.SetDisableRegistrationAliasing(true)
	we := ts.NewTestWorkflowEnvironment()
	wrk.Register(we)

	// Register a callback to send a signal after the workflow starts
	we.RegisterDelayedCallback(func() {
		we.SignalWorkflow(signalUpdateSuffix.Name(), UpdateSuffixParams{Suffix: "!!!"})
	}, 0)

	result, err := wf.ExecuteInTest(we, FormatAndGreetParams{Name: "viktor"})
	if err != nil {
		t.Fatal(err)
	}
	// The signal should have updated the suffix
	if result.Name != "VIKTOR!!!" {
		t.Fatalf("Expected VIKTOR!!!, got %s", result.Name)
	}
}

// Test signal using AddToSelector
type SelectorTestParams struct{}
type SelectorTestResult struct {
	Value string
}

var queueSelectorTest = tempts.NewQueue("selector_test_queue")
var workflowSelectorTest = tempts.NewWorkflow[SelectorTestParams, SelectorTestResult](queueSelectorTest, "selector_test")
var signalSelectorTest = tempts.NewWorkflowSignal[UpdateSuffixParams](&workflowSelectorTest, "selector_signal")

func workflowSelectorTestImpl(ctx workflow.Context, _ SelectorTestParams) (SelectorTestResult, error) {
	selector := workflow.NewSelector(ctx)

	var received string
	signalSelectorTest.AddToSelector(ctx, selector, func(params UpdateSuffixParams) {
		received = params.Suffix
	})

	selector.Select(ctx)
	return SelectorTestResult{Value: received}, nil
}

func TestSignalAddToSelector(t *testing.T) {
	wf := workflowSelectorTest.WithImplementation(workflowSelectorTestImpl)
	wrk, err := tempts.NewWorker(queueSelectorTest, []tempts.Registerable{wf})
	if err != nil {
		t.Fatal(err)
	}

	ts := testsuite.WorkflowTestSuite{}
	ts.SetDisableRegistrationAliasing(true)
	we := ts.NewTestWorkflowEnvironment()
	wrk.Register(we)

	we.RegisterDelayedCallback(func() {
		we.SignalWorkflow(signalSelectorTest.Name(), UpdateSuffixParams{Suffix: "selector-works"})
	}, 0)

	result, err := wf.ExecuteInTest(we, SelectorTestParams{})
	if err != nil {
		t.Fatal(err)
	}
	if result.Value != "selector-works" {
		t.Fatalf("Expected 'selector-works', got %s", result.Value)
	}
}

// Test signal using Receive (blocking)
type ReceiveTestParams struct{}
type ReceiveTestResult struct {
	Value string
}

var queueReceiveTest = tempts.NewQueue("receive_test_queue")
var workflowReceiveTest = tempts.NewWorkflow[ReceiveTestParams, ReceiveTestResult](queueReceiveTest, "receive_test")
var signalReceiveTest = tempts.NewWorkflowSignal[UpdateSuffixParams](&workflowReceiveTest, "receive_signal")

func workflowReceiveTestImpl(ctx workflow.Context, _ ReceiveTestParams) (ReceiveTestResult, error) {
	params := signalReceiveTest.Receive(ctx)
	return ReceiveTestResult{Value: params.Suffix}, nil
}

func TestSignalReceive(t *testing.T) {
	wf := workflowReceiveTest.WithImplementation(workflowReceiveTestImpl)
	wrk, err := tempts.NewWorker(queueReceiveTest, []tempts.Registerable{wf})
	if err != nil {
		t.Fatal(err)
	}

	ts := testsuite.WorkflowTestSuite{}
	ts.SetDisableRegistrationAliasing(true)
	we := ts.NewTestWorkflowEnvironment()
	wrk.Register(we)

	we.RegisterDelayedCallback(func() {
		we.SignalWorkflow(signalReceiveTest.Name(), UpdateSuffixParams{Suffix: "receive-works"})
	}, 0)

	result, err := wf.ExecuteInTest(we, ReceiveTestParams{})
	if err != nil {
		t.Fatal(err)
	}
	if result.Value != "receive-works" {
		t.Fatalf("Expected 'receive-works', got %s", result.Value)
	}
}
