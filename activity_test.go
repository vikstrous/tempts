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

func TestNewActivity(t *testing.T) {
	q := tempts.NewQueue("act-new-q")
	type P struct{ V string }
	type R struct{ V string }

	act := tempts.NewActivity[P, R](q, "test-act")
	if act.Name != "test-act" {
		t.Fatalf("expected name 'test-act', got %q", act.Name)
	}
}

func TestNewActivity_EmptyName(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic for empty activity name")
		}
	}()
	q := tempts.NewQueue("act-empty-q")
	type P struct{ V string }
	type R struct{ V string }
	tempts.NewActivity[P, R](q, "")
}

func TestNewActivity_NonStructParam(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic for non-struct param")
		}
	}()
	q := tempts.NewQueue("act-nonstruct-q")
	tempts.NewActivity[string, string](q, "non-struct-act")
}

func TestNewActivityPositional(t *testing.T) {
	q := tempts.NewQueue("act-pos-q")
	type P struct {
		First string
		Last  string
	}
	type R struct{ Full string }

	act := tempts.NewActivityPositional[P, R](q, "pos-act")
	if act.Name != "pos-act" {
		t.Fatalf("expected name 'pos-act', got %q", act.Name)
	}
}

func TestActivity_WithImplementation(t *testing.T) {
	q := tempts.NewQueue("act-impl-q")
	type P struct{ V string }
	type R struct{ V string }

	act := tempts.NewActivity[P, R](q, "impl-act")
	impl := act.WithImplementation(func(_ context.Context, p P) (R, error) {
		return R{V: p.V}, nil
	})
	if impl == nil {
		t.Fatal("expected non-nil ActivityWithImpl")
	}
}

func TestActivity_WithImplementationPositional(t *testing.T) {
	q := tempts.NewQueue("act-impl-pos-q")
	type P struct {
		First string
		Last  string
	}
	type R struct{ Full string }

	act := tempts.NewActivityPositional[P, R](q, "impl-pos-act")
	impl := act.WithImplementation(func(_ context.Context, p P) (R, error) {
		return R{Full: p.First + " " + p.Last}, nil
	})
	if impl == nil {
		t.Fatal("expected non-nil ActivityWithImpl")
	}
}

func TestActivity_RunInWorkflow(t *testing.T) {
	q := tempts.NewQueue("act-run-q")
	type AP struct{ Name string }
	type AR struct{ Name string }
	type WP struct{ Name string }
	type WR struct{ Result string }

	act := tempts.NewActivity[AP, AR](q, "run-act")
	wf := tempts.NewWorkflow[WP, WR](q, "run-wf")

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

	result, err := wfImpl.ExecuteInTest(env, WP{Name: "test"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Result != "TEST" {
		t.Fatalf("expected 'TEST', got %q", result.Result)
	}
}

func TestActivity_RunPositionalInWorkflow(t *testing.T) {
	q := tempts.NewQueue("act-run-pos-q")
	type AP struct {
		First string
		Last  string
	}
	type AR struct{ Full string }
	type WP struct{ Name string }
	type WR struct{ Result string }

	act := tempts.NewActivityPositional[AP, AR](q, "run-pos-act")
	wf := tempts.NewWorkflow[WP, WR](q, "run-pos-wf")

	wfImpl := wf.WithImplementation(func(ctx workflow.Context, p WP) (WR, error) {
		ctx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
			StartToCloseTimeout: 5 * time.Second,
		})
		r, err := act.Run(ctx, AP{First: "John", Last: "Doe"})
		return WR{Result: r.Full}, err
	})

	wrk, err := tempts.NewWorker(q, []tempts.Registerable{
		act.WithImplementation(func(_ context.Context, p AP) (AR, error) {
			return AR{Full: p.First + " " + p.Last}, nil
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

	result, err := wfImpl.ExecuteInTest(env, WP{Name: "test"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Result != "John Doe" {
		t.Fatalf("expected 'John Doe', got %q", result.Result)
	}
}

func TestActivity_MockStubReturnsError(t *testing.T) {
	q := tempts.NewQueue("act-mock-stub-q")
	type AP struct{ V string }
	type AR struct{ V string }
	type WP struct{ V string }
	type WR struct{ V string }

	act := tempts.NewActivity[AP, AR](q, "mock-stub-act")
	wf := tempts.NewWorkflow[WP, WR](q, "mock-stub-wf")

	// Workflow calls the unmocked activity
	wfImpl := wf.WithImplementation(func(ctx workflow.Context, p WP) (WR, error) {
		ctx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
			StartToCloseTimeout: 5 * time.Second,
		})
		r, err := act.Run(ctx, AP{V: p.V})
		return WR{V: r.V}, err
	})

	ts := testsuite.WorkflowTestSuite{}
	ts.SetDisableRegistrationAliasing(true)
	env := ts.NewTestWorkflowEnvironment()

	// Register mock fallbacks (not real implementations)
	q.RegisterMockFallbacks(env)

	// Execute the workflow — the activity stub should return an error, not panic
	_, err := wfImpl.ExecuteInTest(env, WP{V: "test"})
	if err == nil {
		t.Fatal("expected error from unmocked activity")
	}
	if !strings.Contains(err.Error(), "not mocked") {
		t.Fatalf("expected 'not mocked' in error, got: %v", err)
	}
}
