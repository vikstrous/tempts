package tempts

import (
	"context"
	"testing"
	"time"

	"go.temporal.io/sdk/testsuite"
	"go.temporal.io/sdk/worker"
)

func TestWorkerRunReturnsOnContextCancel(t *testing.T) {
	srv, err := testsuite.StartDevServer(context.Background(), testsuite.DevServerOptions{
		LogLevel: "error",
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { srv.Stop() })

	c, err := NewFromSDK(srv.Client(), "default")
	if err != nil {
		t.Fatal(err)
	}

	q := NewQueue("worker-cancel-test")
	wrk, err := NewWorker(q, nil)
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan error, 1)
	go func() {
		done <- wrk.Run(ctx, c, worker.Options{})
	}()

	cancel()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Worker.Run did not return after context cancellation")
	}
}
