package tempts_test

import (
	"fmt"
	"sync"
	"testing"

	"github.com/vikstrous/tempts"
	"go.temporal.io/sdk/testsuite"
)

func TestNewQueue(t *testing.T) {
	q := tempts.NewQueue("test-queue")
	if q == nil {
		t.Fatal("expected non-nil queue")
	}
}

func TestNewQueue_EmptyName(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic for empty name")
		}
	}()
	tempts.NewQueue("")
}

func TestNewQueue_WhitespaceOnlyName(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic for whitespace-only name")
		}
	}()
	tempts.NewQueue("   ")
}

func TestQueue_ConcurrentRegistration(t *testing.T) {
	q := tempts.NewQueue("concurrent-queue")
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(2)
		actName := fmt.Sprintf("act-%d", i)
		wfName := fmt.Sprintf("wf-%d", i)
		go func() {
			defer wg.Done()
			tempts.NewActivity[struct{ N int }, struct{ R string }](q, actName)
		}()
		go func() {
			defer wg.Done()
			tempts.NewWorkflow[struct{ N int }, struct{ R string }](q, wfName)
		}()
	}
	wg.Wait()
}

func TestRegisterMockFallbacks(t *testing.T) {
	q := tempts.NewQueue("mock-fallback-queue")
	type P struct{ V string }
	type R struct{ V string }
	tempts.NewActivity[P, R](q, "test-activity")
	tempts.NewWorkflow[P, R](q, "test-workflow")

	ts := testsuite.WorkflowTestSuite{}
	ts.SetDisableRegistrationAliasing(true)
	env := ts.NewTestWorkflowEnvironment()
	t.Cleanup(func() { env.AssertExpectations(t) })

	// Should not panic — mock fallbacks registered successfully
	q.RegisterMockFallbacks(env)
}
