package main

import (
	"testing"

	"github.com/vikstrous/tempts"
	"go.temporal.io/sdk/testsuite"
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
