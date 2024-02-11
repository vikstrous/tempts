package main

import (
	"context"
	_ "embed"
	"flag"
	"fmt"
	"os"
	"testing"

	"github.com/vikstrous/tstemporal"
	"go.temporal.io/sdk/client"
)

var record bool

func init() {
	flag.BoolVar(&record, "tstemporal.record", false, "set this to update temporal history fixtures")
}

func TestFormatAndGreetReplayability(t *testing.T) {
	workflowImpl := workflowTypeFormatAndGreet.WithImplementation(workflowFormatAndGreet)
	filename := fmt.Sprintf("histories/%s.json", workflowTypeFormatAndGreet.Name)

	testReplayability(t, workflowImpl, filename)
}

func testReplayability(t *testing.T, workflowImpl *tstemporal.WorkflowWithImpl, filename string) {
	var historiesData []byte
	if record {
		ctx := context.Background()
		c, err := tstemporal.Dial(client.Options{})
		if err != nil {
			t.Fatal(err)
		}
		historiesData, err = tstemporal.GetWorkflowHistoriesBundle(ctx, c, workflowImpl)
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

	err := tstemporal.ReplayWorkflow(historiesData, workflowImpl)
	if err != nil {
		t.Fatal(err)
	}
}
