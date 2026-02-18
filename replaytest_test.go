package tempts_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/vikstrous/tempts"
	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"
)

func TestReplayWorkflow_InvalidJSON(t *testing.T) {
	err := tempts.ReplayWorkflow(
		[]byte("invalid json"),
		func(workflow.Context, struct{}) (struct{}, error) { return struct{}{}, nil },
		worker.WorkflowReplayerOptions{},
	)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestReplayWorkflow_EmptyHistories(t *testing.T) {
	data, err := json.Marshal(struct {
		Histories    []any
		WorkflowName string
	}{
		Histories:    []any{},
		WorkflowName: "test",
	})
	if err != nil {
		t.Fatal(err)
	}

	err = tempts.ReplayWorkflow(
		data,
		func(workflow.Context, struct{}) (struct{}, error) { return struct{}{}, nil },
		worker.WorkflowReplayerOptions{},
	)
	if err == nil {
		t.Fatal("expected error for empty histories")
	}
	if !strings.Contains(err.Error(), "no histories available") {
		t.Fatalf("expected 'no histories available', got: %v", err)
	}
}
