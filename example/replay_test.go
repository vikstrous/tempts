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
)

var record bool

func init() {
	flag.BoolVar(&record, "tempts.record", false, "set this to update temporal history fixtures")
}

func TestFormatAndGreetReplayability(t *testing.T) {
	filename := fmt.Sprintf("histories/%s.json", workflowTypeFormatAndGreet.Name())

	testReplayability(t, workflowTypeFormatAndGreet, workflowFormatAndGreet, filename)
}

func testReplayability(t *testing.T, workflowDeclaration tempts.WorkflowDeclaration, fn any, filename string) {
	var historiesData []byte
	if record {
		ctx := context.Background()
		c, err := tempts.Dial(client.Options{})
		if err != nil {
			t.Fatal(err)
		}
		historiesData, err = tempts.GetWorkflowHistoriesBundle(ctx, c, workflowDeclaration)
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

	err := tempts.ReplayWorkflow(historiesData, fn)
	if err != nil {
		t.Fatal(err)
	}
}
