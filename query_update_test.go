package tempts_test

import (
	"testing"
	"time"

	"github.com/vikstrous/tempts"
	"go.temporal.io/sdk/testsuite"
	"go.temporal.io/sdk/workflow"
)

func TestNewQueryHandler(t *testing.T) {
	type QP struct{}
	type QR struct{ Value string }

	qh := tempts.NewQueryHandler[QP, QR]("test-query")
	if qh == nil {
		t.Fatal("expected non-nil QueryHandler")
	}
}

func TestNewQueryHandler_EmptyName(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic for empty query name")
		}
	}()
	type QP struct{}
	type QR struct{ Value string }
	tempts.NewQueryHandler[QP, QR]("")
}

func TestQueryHandler_SetHandlerAndQuery(t *testing.T) {
	q := tempts.NewQueue("query-test-q")
	type WP struct{}
	type WR struct{}
	type QP struct{}
	type QR struct{ Value string }

	wf := tempts.NewWorkflow[WP, WR](q, "query-wf")
	qh := tempts.NewQueryHandler[QP, QR]("query-handler")

	wfImpl := wf.WithImplementation(func(ctx workflow.Context, _ WP) (WR, error) {
		qh.SetHandler(ctx, func(_ QP) (QR, error) {
			return QR{Value: "query-result"}, nil
		})
		// Sleep to flush the command queue so the query handler is registered
		// before the delayed callback fires
		workflow.Sleep(ctx, time.Millisecond)
		// Keep the workflow alive for the query
		workflow.GetSignalChannel(ctx, "done").Receive(ctx, nil)
		return WR{}, nil
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
		result, err := env.QueryWorkflow("query-handler", QP{})
		if err != nil {
			t.Fatalf("query failed: %v", err)
		}
		var qr QR
		if err := result.Get(&qr); err != nil {
			t.Fatalf("get query result: %v", err)
		}
		if qr.Value != "query-result" {
			t.Fatalf("expected 'query-result', got %q", qr.Value)
		}
		// Signal workflow to complete
		env.SignalWorkflow("done", nil)
	}, time.Millisecond)

	_, err = wfImpl.ExecuteInTest(env, WP{})
	if err != nil {
		t.Fatal(err)
	}
}
