package tempts_test

import (
	"testing"

	"github.com/vikstrous/tempts"
	"go.temporal.io/sdk/testsuite"
	"go.temporal.io/sdk/workflow"
)

func TestNewWorkflowSignal(t *testing.T) {
	q := tempts.NewQueue("sig-new-q")
	type WP struct{ V string }
	type WR struct{ V string }
	type SP struct{ S string }

	wf := tempts.NewWorkflow[WP, WR](q, "sig-wf")
	sig := tempts.NewWorkflowSignal[SP](&wf, "test-signal")
	if sig.Name() != "test-signal" {
		t.Fatalf("expected name 'test-signal', got %q", sig.Name())
	}
}

func TestNewWorkflowSignal_EmptyName(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic for empty signal name")
		}
	}()
	q := tempts.NewQueue("sig-empty-q")
	type WP struct{ V string }
	type WR struct{ V string }
	type SP struct{ S string }

	wf := tempts.NewWorkflow[WP, WR](q, "sig-empty-wf")
	tempts.NewWorkflowSignal[SP](&wf, "")
}

func TestWorkflowSignal_Receive(t *testing.T) {
	q := tempts.NewQueue("sig-recv-q")
	type WP struct{}
	type WR struct{ V string }
	type SP struct{ S string }

	wf := tempts.NewWorkflow[WP, WR](q, "sig-recv-wf")
	sig := tempts.NewWorkflowSignal[SP](&wf, "recv-signal")

	wfImpl := wf.WithImplementation(func(ctx workflow.Context, _ WP) (WR, error) {
		s := sig.Receive(ctx)
		return WR{V: s.S}, nil
	})

	wrk, err := tempts.NewWorker(q, []tempts.Registerable{wfImpl})
	if err != nil {
		t.Fatal(err)
	}

	ts := testsuite.WorkflowTestSuite{}
	ts.SetDisableRegistrationAliasing(true)
	env := ts.NewTestWorkflowEnvironment()
	t.Cleanup(func() { env.AssertExpectations(t) })
	wrk.Register(env)

	env.RegisterDelayedCallback(func() {
		env.SignalWorkflow(sig.Name(), SP{S: "hello-signal"})
	}, 0)

	result, err := wfImpl.ExecuteInTest(env, WP{})
	if err != nil {
		t.Fatal(err)
	}
	if result.V != "hello-signal" {
		t.Fatalf("expected 'hello-signal', got %q", result.V)
	}
}

func TestWorkflowSignal_TryReceive(t *testing.T) {
	q := tempts.NewQueue("sig-try-q")
	type WP struct{}
	type WR struct{ V string }
	type SP struct{ S string }

	wf := tempts.NewWorkflow[WP, WR](q, "sig-try-wf")
	sig := tempts.NewWorkflowSignal[SP](&wf, "try-signal")

	wfImpl := wf.WithImplementation(func(ctx workflow.Context, _ WP) (WR, error) {
		_, ok := sig.TryReceive(ctx)
		if ok {
			return WR{V: "unexpected"}, nil
		}
		return WR{V: "none"}, nil
	})

	wrk, err := tempts.NewWorker(q, []tempts.Registerable{wfImpl})
	if err != nil {
		t.Fatal(err)
	}

	ts := testsuite.WorkflowTestSuite{}
	ts.SetDisableRegistrationAliasing(true)
	env := ts.NewTestWorkflowEnvironment()
	t.Cleanup(func() { env.AssertExpectations(t) })
	wrk.Register(env)

	result, err := wfImpl.ExecuteInTest(env, WP{})
	if err != nil {
		t.Fatal(err)
	}
	if result.V != "none" {
		t.Fatalf("expected 'none', got %q", result.V)
	}
}

func TestWorkflowSignal_AddToSelector(t *testing.T) {
	q := tempts.NewQueue("sig-sel-q")
	type WP struct{}
	type WR struct{ V string }
	type SP struct{ S string }

	wf := tempts.NewWorkflow[WP, WR](q, "sig-sel-wf")
	sig := tempts.NewWorkflowSignal[SP](&wf, "sel-signal")

	wfImpl := wf.WithImplementation(func(ctx workflow.Context, _ WP) (WR, error) {
		selector := workflow.NewSelector(ctx)
		var received string
		sig.AddToSelector(ctx, selector, func(p SP) {
			received = p.S
		})
		selector.Select(ctx)
		return WR{V: received}, nil
	})

	wrk, err := tempts.NewWorker(q, []tempts.Registerable{wfImpl})
	if err != nil {
		t.Fatal(err)
	}

	ts := testsuite.WorkflowTestSuite{}
	ts.SetDisableRegistrationAliasing(true)
	env := ts.NewTestWorkflowEnvironment()
	t.Cleanup(func() { env.AssertExpectations(t) })
	wrk.Register(env)

	env.RegisterDelayedCallback(func() {
		env.SignalWorkflow(sig.Name(), SP{S: "selector-works"})
	}, 0)

	result, err := wfImpl.ExecuteInTest(env, WP{})
	if err != nil {
		t.Fatal(err)
	}
	if result.V != "selector-works" {
		t.Fatalf("expected 'selector-works', got %q", result.V)
	}
}
