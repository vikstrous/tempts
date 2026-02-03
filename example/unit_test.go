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

// Test signal using GetChannel with a selector pattern
type ChannelTestParams struct{}
type ChannelTestResult struct {
	Value string
}

var queueChannelTest = tempts.NewQueue("channel_test_queue")
var workflowChannelTest = tempts.NewWorkflow[ChannelTestParams, ChannelTestResult](queueChannelTest, "channel_test")
var signalChannelTest = tempts.NewWorkflowSignal[UpdateSuffixParams](&workflowChannelTest, "channel_signal")

func workflowChannelTestImpl(ctx workflow.Context, _ ChannelTestParams) (ChannelTestResult, error) {
	ch := signalChannelTest.GetChannel(ctx)
	selector := workflow.NewSelector(ctx)

	var received string
	selector.AddReceive(ch.Raw(), func(c workflow.ReceiveChannel, more bool) {
		val, _ := ch.Receive(ctx)
		received = val.Suffix
	})

	selector.Select(ctx)
	return ChannelTestResult{Value: received}, nil
}

func TestSignalGetChannel(t *testing.T) {
	wf := workflowChannelTest.WithImplementation(workflowChannelTestImpl)
	wrk, err := tempts.NewWorker(queueChannelTest, []tempts.Registerable{wf})
	if err != nil {
		t.Fatal(err)
	}

	ts := testsuite.WorkflowTestSuite{}
	ts.SetDisableRegistrationAliasing(true)
	we := ts.NewTestWorkflowEnvironment()
	wrk.Register(we)

	we.RegisterDelayedCallback(func() {
		we.SignalWorkflow(signalChannelTest.Name(), UpdateSuffixParams{Suffix: "channel-works"})
	}, 0)

	result, err := wf.ExecuteInTest(we, ChannelTestParams{})
	if err != nil {
		t.Fatal(err)
	}
	if result.Value != "channel-works" {
		t.Fatalf("Expected 'channel-works', got %s", result.Value)
	}
}
