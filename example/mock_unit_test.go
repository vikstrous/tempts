package main

import (
	"testing"

	"github.com/stretchr/testify/mock"
	"go.temporal.io/sdk/testsuite"
)

func TestMocks(t *testing.T) {
	ts := testsuite.WorkflowTestSuite{}
	ts.SetDisableRegistrationAliasing(true)
	we := ts.NewTestWorkflowEnvironment()
	queueMain.RegisterMockFallbacks(we)

	we.OnActivity(activityTypeFormatName.Name, mock.Anything, mock.Anything).Return(FormatNameResult{Name: "VIKTOR"}, nil)
	we.OnWorkflow(workflowTypeJustGreet.Name(), mock.Anything, mock.Anything).Return(JustGreetResult{Name: "Hello, VIKTOR"}, nil)

	wf := workflowTypeFormatAndGreet.WithImplementation(workflowFormatAndGreet)
	result, err := wf.ExecuteInTest(we, FormatAndGreetParams{Name: "viktor"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Name != "VIKTOR" {
		t.Fatal("Expected VIKTOR, got", result)
	}
}
