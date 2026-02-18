package tempts_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/vikstrous/tempts"
	"go.temporal.io/sdk/testsuite"
	"go.temporal.io/sdk/workflow"
)

func TestNewWorkflow(t *testing.T) {
	q := tempts.NewQueue("wf-new-q")
	type P struct{ V string }
	type R struct{ V string }

	wf := tempts.NewWorkflow[P, R](q, "test-wf")
	if wf.Name() != "test-wf" {
		t.Fatalf("expected name 'test-wf', got %q", wf.Name())
	}
}

func TestNewWorkflow_EmptyName(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic for empty workflow name")
		}
	}()
	q := tempts.NewQueue("wf-empty-q")
	type P struct{ V string }
	type R struct{ V string }
	tempts.NewWorkflow[P, R](q, "")
}

func TestNewWorkflow_NonStructParam(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic for non-struct param")
		}
	}()
	q := tempts.NewQueue("wf-nonstruct-q")
	tempts.NewWorkflow[string, string](q, "non-struct-wf")
}

func TestNewWorkflowPositional(t *testing.T) {
	q := tempts.NewQueue("wf-pos-q")
	type P struct {
		First string
		Last  string
	}
	type R struct{ Full string }

	wf := tempts.NewWorkflowPositional[P, R](q, "pos-wf")
	if wf.Name() != "pos-wf" {
		t.Fatalf("expected name 'pos-wf', got %q", wf.Name())
	}
}

func TestWorkflow_WithImplementation_NoDefaultTimeout(t *testing.T) {
	q := tempts.NewQueue("wf-no-timeout-q")
	type AP struct{ Name string }
	type AR struct{ Name string }
	type WP struct{ Name string }
	type WR struct{ Name string }

	act := tempts.NewActivity[AP, AR](q, "no-timeout-act")
	wf := tempts.NewWorkflow[WP, WR](q, "no-timeout-wf")

	// Workflow that does NOT set activity options (no timeout)
	wfImpl := wf.WithImplementation(func(ctx workflow.Context, p WP) (WR, error) {
		r, err := act.Run(ctx, AP{Name: p.Name})
		return WR{Name: r.Name}, err
	})

	wrk, err := tempts.NewWorker(q, []tempts.Registerable{
		act.WithImplementation(func(_ context.Context, p AP) (AR, error) {
			return AR{Name: p.Name}, nil
		}),
		wfImpl,
	})
	if err != nil {
		t.Fatal(err)
	}

	ts := testsuite.WorkflowTestSuite{}
	ts.SetDisableRegistrationAliasing(true)
	env := ts.NewTestWorkflowEnvironment()
	t.Cleanup(func() { env.AssertExpectations(t) })
	wrk.Register(env)

	// Without explicit activity timeout, activity execution should fail
	_, err = wfImpl.ExecuteInTest(env, WP{Name: "test"})
	if err == nil {
		t.Fatal("expected error when no activity timeout is set")
	}
}

func TestWorkflow_ExecuteInTest(t *testing.T) {
	q := tempts.NewQueue("wf-exec-q")
	type AP struct{ Name string }
	type AR struct{ Name string }
	type WP struct{ Name string }
	type WR struct{ Result string }

	act := tempts.NewActivity[AP, AR](q, "exec-act")
	wf := tempts.NewWorkflow[WP, WR](q, "exec-wf")

	wfImpl := wf.WithImplementation(func(ctx workflow.Context, p WP) (WR, error) {
		ctx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
			StartToCloseTimeout: 5 * time.Second,
		})
		r, err := act.Run(ctx, AP{Name: p.Name})
		return WR{Result: r.Name}, err
	})

	wrk, err := tempts.NewWorker(q, []tempts.Registerable{
		act.WithImplementation(func(_ context.Context, p AP) (AR, error) {
			return AR{Name: strings.ToUpper(p.Name)}, nil
		}),
		wfImpl,
	})
	if err != nil {
		t.Fatal(err)
	}

	ts := testsuite.WorkflowTestSuite{}
	ts.SetDisableRegistrationAliasing(true)
	env := ts.NewTestWorkflowEnvironment()
	t.Cleanup(func() { env.AssertExpectations(t) })
	wrk.Register(env)

	result, err := wfImpl.ExecuteInTest(env, WP{Name: "hello"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Result != "HELLO" {
		t.Fatalf("expected 'HELLO', got %q", result.Result)
	}
}

func TestWorkflow_RunChild(t *testing.T) {
	q := tempts.NewQueue("wf-child-q")
	type CP struct{ V string }
	type CR struct{ V string }
	type PP struct{ V string }
	type PR struct{ V string }

	childWf := tempts.NewWorkflow[CP, CR](q, "child-wf")
	parentWf := tempts.NewWorkflow[PP, PR](q, "parent-wf")

	parentImpl := parentWf.WithImplementation(func(ctx workflow.Context, p PP) (PR, error) {
		result, err := childWf.RunChild(ctx, workflow.ChildWorkflowOptions{}, CP{V: p.V})
		return PR{V: result.V}, err
	})
	childImpl := childWf.WithImplementation(func(_ workflow.Context, p CP) (CR, error) {
		return CR{V: "child-" + p.V}, nil
	})

	wrk, err := tempts.NewWorker(q, []tempts.Registerable{parentImpl, childImpl})
	if err != nil {
		t.Fatal(err)
	}

	ts := testsuite.WorkflowTestSuite{}
	ts.SetDisableRegistrationAliasing(true)
	env := ts.NewTestWorkflowEnvironment()
	t.Cleanup(func() { env.AssertExpectations(t) })
	wrk.Register(env)

	result, err := parentImpl.ExecuteInTest(env, PP{V: "test"})
	if err != nil {
		t.Fatal(err)
	}
	if result.V != "child-test" {
		t.Fatalf("expected 'child-test', got %q", result.V)
	}
}

func TestWorkflowWithImpl_Name(t *testing.T) {
	q := tempts.NewQueue("wf-impl-name-q")
	type P struct{ V string }
	type R struct{ V string }

	wf := tempts.NewWorkflow[P, R](q, "named-wf")
	impl := wf.WithImplementation(func(_ workflow.Context, p P) (R, error) {
		return R{V: p.V}, nil
	})
	if impl.Name() != "named-wf" {
		t.Fatalf("expected 'named-wf', got %q", impl.Name())
	}
}

func TestWorkflow_MockStubReturnsError(t *testing.T) {
	q := tempts.NewQueue("wf-mock-stub-q")
	type P struct{ V string }
	type R struct{ V string }

	wf := tempts.NewWorkflow[P, R](q, "mock-stub-wf")

	ts := testsuite.WorkflowTestSuite{}
	ts.SetDisableRegistrationAliasing(true)
	env := ts.NewTestWorkflowEnvironment()

	// Register mock fallbacks (the workflow stub returns an error, not a panic)
	q.RegisterMockFallbacks(env)

	// Execute the workflow by name — uses the mock stub
	env.ExecuteWorkflow(wf.Name(), P{V: "test"})
	err := env.GetWorkflowError()
	if err == nil {
		t.Fatal("expected error from unmocked workflow")
	}
	if !strings.Contains(err.Error(), "not mocked") {
		t.Fatalf("expected 'not mocked' in error, got: %v", err)
	}
}
