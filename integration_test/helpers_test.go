package integration_test

import (
	"context"
	"testing"

	"github.com/vikstrous/tempts"
	"go.temporal.io/sdk/worker"
)

func startWorker(t *testing.T, queue *tempts.Queue, registerables []tempts.Registerable) {
	t.Helper()
	wrk, err := tempts.NewWorker(queue, registerables)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go func() {
		if err := wrk.Run(ctx, testClient, worker.Options{}); err != nil {
			t.Logf("worker stopped: %v", err)
		}
	}()
}
