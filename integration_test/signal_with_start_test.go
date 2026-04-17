package integration_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/vikstrous/tempts"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/workflow"
)

// -- Positional workflow declarations --

var queuePositionalSignal = tempts.NewQueue("positional_signal_test")

type PositionalSignalParams struct {
	FirstName string
	LastName  string
}

type PositionalSignalResult struct {
	Greeting string
}

var workflowPositionalSignal = tempts.NewWorkflowPositional[PositionalSignalParams, PositionalSignalResult](queuePositionalSignal, "positional_signal_wf")

type SignalSuffix struct {
	Suffix string
}

var signalPositional = tempts.NewWorkflowSignal[SignalSuffix](&workflowPositionalSignal, "suffix_signal")

func workflowPositionalSignalImpl(ctx workflow.Context, params PositionalSignalParams) (PositionalSignalResult, error) {
	greeting := fmt.Sprintf("Hello %s %s", params.FirstName, params.LastName)
	if sig, ok := signalPositional.TryReceive(ctx); ok {
		greeting += sig.Suffix
	}
	return PositionalSignalResult{Greeting: greeting}, nil
}

// -- Non-positional workflow declarations (control) --

var queueNonPositionalSignal = tempts.NewQueue("non_positional_signal_test")

type NonPositionalSignalParams struct {
	FirstName string
	LastName  string
}

type NonPositionalSignalResult struct {
	Greeting string
}

var workflowNonPositionalSignal = tempts.NewWorkflow[NonPositionalSignalParams, NonPositionalSignalResult](queueNonPositionalSignal, "non_positional_signal_wf")

var signalNonPositional = tempts.NewWorkflowSignal[SignalSuffix](&workflowNonPositionalSignal, "suffix_signal")

func workflowNonPositionalSignalImpl(ctx workflow.Context, params NonPositionalSignalParams) (NonPositionalSignalResult, error) {
	greeting := fmt.Sprintf("Hello %s %s", params.FirstName, params.LastName)
	if sig, ok := signalNonPositional.TryReceive(ctx); ok {
		greeting += sig.Suffix
	}
	return NonPositionalSignalResult{Greeting: greeting}, nil
}

// -- Tests --

func TestSignalWithStartPositional(t *testing.T) {
	startWorker(t, queuePositionalSignal, []tempts.Registerable{
		workflowPositionalSignal.WithImplementation(workflowPositionalSignalImpl),
	})

	run, err := signalPositional.SignalWithStart(
		context.Background(), testClient,
		client.StartWorkflowOptions{ID: "test-positional-signal-with-start"},
		PositionalSignalParams{FirstName: "Alice", LastName: "Smith"},
		SignalSuffix{Suffix: "!"},
	)
	if err != nil {
		t.Fatal(err)
	}

	var result PositionalSignalResult
	if err := run.Get(context.Background(), &result); err != nil {
		t.Fatalf("workflow failed (likely positional arg mismatch on wire): %v", err)
	}

	expected := "Hello Alice Smith!"
	if result.Greeting != expected {
		t.Fatalf("expected %q, got %q", expected, result.Greeting)
	}
}

func TestSignalWithStartNonPositional(t *testing.T) {
	startWorker(t, queueNonPositionalSignal, []tempts.Registerable{
		workflowNonPositionalSignal.WithImplementation(workflowNonPositionalSignalImpl),
	})

	run, err := signalNonPositional.SignalWithStart(
		context.Background(), testClient,
		client.StartWorkflowOptions{ID: "test-non-positional-signal-with-start"},
		NonPositionalSignalParams{FirstName: "Bob", LastName: "Jones"},
		SignalSuffix{Suffix: "?"},
	)
	if err != nil {
		t.Fatal(err)
	}

	var result NonPositionalSignalResult
	if err := run.Get(context.Background(), &result); err != nil {
		t.Fatalf("workflow failed: %v", err)
	}

	expected := "Hello Bob Jones?"
	if result.Greeting != expected {
		t.Fatalf("expected %q, got %q", expected, result.Greeting)
	}
}
