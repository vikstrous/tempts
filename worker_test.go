package tempts_test

import (
	"context"
	"strings"
	"testing"

	"github.com/vikstrous/tempts"
	"go.temporal.io/sdk/workflow"
)

func TestNewWorker_Valid(t *testing.T) {
	q := tempts.NewQueue("worker-valid-q")
	type P struct{ V string }
	type R struct{ V string }

	act := tempts.NewActivity[P, R](q, "worker-act")
	wf := tempts.NewWorkflow[P, R](q, "worker-wf")

	wrk, err := tempts.NewWorker(q, []tempts.Registerable{
		act.WithImplementation(func(_ context.Context, p P) (R, error) { return R{V: p.V}, nil }),
		wf.WithImplementation(func(_ workflow.Context, p P) (R, error) { return R{V: p.V}, nil }),
	})
	if err != nil {
		t.Fatal(err)
	}
	if wrk == nil {
		t.Fatal("expected non-nil worker")
	}
}

func TestNewWorker_MissingActivityImpl(t *testing.T) {
	q := tempts.NewQueue("worker-missing-act-q")
	type P struct{ V string }
	type R struct{ V string }

	tempts.NewActivity[P, R](q, "missing-act")
	wf := tempts.NewWorkflow[P, R](q, "present-wf")

	_, err := tempts.NewWorker(q, []tempts.Registerable{
		wf.WithImplementation(func(_ workflow.Context, p P) (R, error) { return R{}, nil }),
	})
	if err == nil {
		t.Fatal("expected error for missing activity implementation")
	}
	if !strings.Contains(err.Error(), "missing-act") {
		t.Fatalf("expected error to mention activity name, got: %v", err)
	}
}

func TestNewWorker_MissingWorkflowImpl(t *testing.T) {
	q := tempts.NewQueue("worker-missing-wf-q")
	type P struct{ V string }
	type R struct{ V string }

	act := tempts.NewActivity[P, R](q, "present-act")
	tempts.NewWorkflow[P, R](q, "missing-wf")

	_, err := tempts.NewWorker(q, []tempts.Registerable{
		act.WithImplementation(func(_ context.Context, p P) (R, error) { return R{}, nil }),
	})
	if err == nil {
		t.Fatal("expected error for missing workflow implementation")
	}
	if !strings.Contains(err.Error(), "missing-wf") {
		t.Fatalf("expected error to mention workflow name, got: %v", err)
	}
}

func TestNewWorker_ExtraActivity(t *testing.T) {
	q1 := tempts.NewQueue("worker-extra-act-q")
	q2 := tempts.NewQueue("worker-extra-act-q") // same name, different queue object
	type P struct{ V string }
	type R struct{ V string }

	act1 := tempts.NewActivity[P, R](q1, "act1")
	act2 := tempts.NewActivity[P, R](q2, "act2") // registered on q2, not q1
	wf := tempts.NewWorkflow[P, R](q1, "wf1")

	_, err := tempts.NewWorker(q1, []tempts.Registerable{
		act1.WithImplementation(func(_ context.Context, p P) (R, error) { return R{}, nil }),
		act2.WithImplementation(func(_ context.Context, p P) (R, error) { return R{}, nil }),
		wf.WithImplementation(func(_ workflow.Context, p P) (R, error) { return R{}, nil }),
	})
	if err == nil {
		t.Fatal("expected error for extra activity")
	}
	if !strings.Contains(err.Error(), "act2") {
		t.Fatalf("expected error to mention extra activity, got: %v", err)
	}
}

func TestNewWorker_ExtraWorkflow(t *testing.T) {
	q1 := tempts.NewQueue("worker-extra-wf-q")
	q2 := tempts.NewQueue("worker-extra-wf-q") // same name, different queue object
	type P struct{ V string }
	type R struct{ V string }

	wf1 := tempts.NewWorkflow[P, R](q1, "wf1")
	wf2 := tempts.NewWorkflow[P, R](q2, "wf2") // registered on q2, not q1

	_, err := tempts.NewWorker(q1, []tempts.Registerable{
		wf1.WithImplementation(func(_ workflow.Context, p P) (R, error) { return R{}, nil }),
		wf2.WithImplementation(func(_ workflow.Context, p P) (R, error) { return R{}, nil }),
	})
	if err == nil {
		t.Fatal("expected error for extra workflow")
	}
	if !strings.Contains(err.Error(), "wf2") {
		t.Fatalf("expected error to mention extra workflow, got: %v", err)
	}
}

func TestNewWorker_DuplicateActivity(t *testing.T) {
	q := tempts.NewQueue("worker-dup-act-q")
	type P struct{ V string }
	type R struct{ V string }

	act := tempts.NewActivity[P, R](q, "dup-act")
	wf := tempts.NewWorkflow[P, R](q, "dup-wf")

	_, err := tempts.NewWorker(q, []tempts.Registerable{
		act.WithImplementation(func(_ context.Context, p P) (R, error) { return R{}, nil }),
		act.WithImplementation(func(_ context.Context, p P) (R, error) { return R{}, nil }),
		wf.WithImplementation(func(_ workflow.Context, p P) (R, error) { return R{}, nil }),
	})
	if err == nil {
		t.Fatal("expected error for duplicate activity")
	}
	if !strings.Contains(err.Error(), "duplicate") {
		t.Fatalf("expected 'duplicate' in error, got: %v", err)
	}
}

func TestNewWorker_DuplicateWorkflow(t *testing.T) {
	q := tempts.NewQueue("worker-dup-wf-q")
	type P struct{ V string }
	type R struct{ V string }

	act := tempts.NewActivity[P, R](q, "dup-wf-act")
	wf := tempts.NewWorkflow[P, R](q, "dup-wf")

	_, err := tempts.NewWorker(q, []tempts.Registerable{
		act.WithImplementation(func(_ context.Context, p P) (R, error) { return R{}, nil }),
		wf.WithImplementation(func(_ workflow.Context, p P) (R, error) { return R{}, nil }),
		wf.WithImplementation(func(_ workflow.Context, p P) (R, error) { return R{}, nil }),
	})
	if err == nil {
		t.Fatal("expected error for duplicate workflow")
	}
	if !strings.Contains(err.Error(), "duplicate") {
		t.Fatalf("expected 'duplicate' in error, got: %v", err)
	}
}

func TestNewWorker_WrongQueue(t *testing.T) {
	q1 := tempts.NewQueue("queue-1")
	q2 := tempts.NewQueue("queue-2")
	type P struct{ V string }
	type R struct{ V string }

	act := tempts.NewActivity[P, R](q2, "wrong-q-act") // on q2
	wf := tempts.NewWorkflow[P, R](q1, "wrong-q-wf")

	_, err := tempts.NewWorker(q1, []tempts.Registerable{
		act.WithImplementation(func(_ context.Context, p P) (R, error) { return R{}, nil }),
		wf.WithImplementation(func(_ workflow.Context, p P) (R, error) { return R{}, nil }),
	})
	if err == nil {
		t.Fatal("expected error for wrong queue")
	}
}
