package main

import (
	"context"
	_ "embed"
	"flag"
	"fmt"
	"os"
	"testing"

	"github.com/vikstrous/tempts"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
)

var record bool

func init() {
	flag.BoolVar(&record, "tempts.record", false, "set this to update temporal history fixtures")
}

func TestFormatAndGreetReplayability(t *testing.T) {
	filename := fmt.Sprintf("histories/%s.json", workflowTypeFormatAndGreet.Name())

	var historiesData []byte
	if record {
		ctx := context.Background()
		c, err := tempts.Dial(client.Options{})
		if err != nil {
			t.Fatal(err)
		}
		historiesData, err = tempts.GetWorkflowHistoriesBundle(ctx, c, workflowTypeFormatAndGreet)
		if err != nil {
			t.Fatal(err)
		}

		err = os.WriteFile(filename, historiesData, 0o644)
		if err != nil {
			t.Fatal(err)
		}
	} else {
		var err error
		historiesData, err = os.ReadFile(filename)
		if err != nil {
			t.Fatal(err)
		}
	}

	err := workflowTypeFormatAndGreet.ReplayWorkflow(historiesData, workflowFormatAndGreet, worker.WorkflowReplayerOptions{})
	if err != nil {
		t.Fatal(err)
	}
}
