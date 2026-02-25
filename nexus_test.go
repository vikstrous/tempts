package tempts

import (
	"context"
	"testing"

	"github.com/nexus-rpc/sdk-go/nexus"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/temporalnexus"
	"go.temporal.io/sdk/workflow"
)

type testInput struct {
	Value string
}

type testOutput struct {
	Result string
}

func TestWithImplementations(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		svc := NewService("test-svc")
		syncOp := NewSyncOperation[testInput, testOutput](svc, "sync")
		asyncOp := NewAsyncOperation[testInput, testOutput](svc, "async")

		result, err := svc.WithImplementations(
			syncOp.WithImplementation(func(ctx context.Context, input testInput, opts nexus.StartOperationOptions) (testOutput, error) {
				return testOutput{}, nil
			}),
			asyncOp.WithImplementation(
				func(ctx workflow.Context, input testInput) (testOutput, error) { return testOutput{}, nil },
				func(ctx context.Context, input testInput, opts nexus.StartOperationOptions) (client.StartWorkflowOptions, error) {
					return client.StartWorkflowOptions{}, nil
				},
			),
		)
		require.NoError(t, err)
		require.NotNil(t, result)
	})

	t.Run("missing implementation", func(t *testing.T) {
		svc := NewService("test-svc")
		NewSyncOperation[testInput, testOutput](svc, "op1")
		op2 := NewSyncOperation[testInput, testOutput](svc, "op2")

		_, err := svc.WithImplementations(
			op2.WithImplementation(func(ctx context.Context, input testInput, opts nexus.StartOperationOptions) (testOutput, error) {
				return testOutput{}, nil
			}),
		)
		require.ErrorContains(t, err, "missing implementation for operation op1")
	})

	t.Run("duplicate implementation", func(t *testing.T) {
		svc := NewService("test-svc")
		syncOp := NewSyncOperation[testInput, testOutput](svc, "op")

		impl := syncOp.WithImplementation(func(ctx context.Context, input testInput, opts nexus.StartOperationOptions) (testOutput, error) {
			return testOutput{}, nil
		})

		_, err := svc.WithImplementations(impl, impl)
		require.ErrorContains(t, err, "duplicate implementation for operation op")
	})

	t.Run("wrong service", func(t *testing.T) {
		svc1 := NewService("svc1")
		svc2 := NewService("svc2")
		op := NewSyncOperation[testInput, testOutput](svc1, "op")
		NewSyncOperation[testInput, testOutput](svc2, "op2")

		_, err := svc2.WithImplementations(
			op.WithImplementation(func(ctx context.Context, input testInput, opts nexus.StartOperationOptions) (testOutput, error) {
				return testOutput{}, nil
			}),
		)
		require.ErrorContains(t, err, "operation op belongs to service svc1, not svc2")
	})
}

func TestDuplicateOperationName(t *testing.T) {
	t.Run("sync", func(t *testing.T) {
		svc := NewService("test-svc")
		NewSyncOperation[testInput, testOutput](svc, "op")
		require.PanicsWithValue(t, "operation op already declared on service test-svc", func() {
			NewSyncOperation[testInput, testOutput](svc, "op")
		})
	})

	t.Run("async", func(t *testing.T) {
		svc := NewService("test-svc")
		NewAsyncOperation[testInput, testOutput](svc, "op")
		require.PanicsWithValue(t, "operation op already declared on service test-svc", func() {
			NewAsyncOperation[testInput, testOutput](svc, "op")
		})
	})

	t.Run("sync then async same name", func(t *testing.T) {
		svc := NewService("test-svc")
		NewSyncOperation[testInput, testOutput](svc, "op")
		require.PanicsWithValue(t, "operation op already declared on service test-svc", func() {
			NewAsyncOperation[testInput, testOutput](svc, "op")
		})
	})
}

func TestNewWorkerWithNexusOperations(t *testing.T) {
	t.Run("operations registered directly", func(t *testing.T) {
		q := NewQueue("test-q")
		svc := NewService("test-svc")
		syncOp := NewSyncOperation[testInput, testOutput](svc, "sync")
		asyncOp := NewAsyncOperation[testInput, testOutput](svc, "async")

		wrk, err := NewWorker(q, []Registerable{
			syncOp.WithImplementation(func(ctx context.Context, input testInput, opts nexus.StartOperationOptions) (testOutput, error) {
				return testOutput{}, nil
			}),
			asyncOp.WithImplementation(
				func(ctx workflow.Context, input testInput) (testOutput, error) { return testOutput{}, nil },
				func(ctx context.Context, input testInput, opts nexus.StartOperationOptions) (client.StartWorkflowOptions, error) {
					return client.StartWorkflowOptions{}, nil
				},
			),
		})
		require.NoError(t, err)
		require.NotNil(t, wrk)
	})

	t.Run("missing operation detected", func(t *testing.T) {
		q := NewQueue("test-q")
		svc := NewService("test-svc")
		NewSyncOperation[testInput, testOutput](svc, "op1")
		op2 := NewSyncOperation[testInput, testOutput](svc, "op2")

		_, err := NewWorker(q, []Registerable{
			op2.WithImplementation(func(ctx context.Context, input testInput, opts nexus.StartOperationOptions) (testOutput, error) {
				return testOutput{}, nil
			}),
		})
		require.ErrorContains(t, err, "missing implementation for operation op1")
	})

	t.Run("duplicate operation detected", func(t *testing.T) {
		q := NewQueue("test-q")
		svc := NewService("test-svc")
		syncOp := NewSyncOperation[testInput, testOutput](svc, "op")

		impl := syncOp.WithImplementation(func(ctx context.Context, input testInput, opts nexus.StartOperationOptions) (testOutput, error) {
			return testOutput{}, nil
		})

		_, err := NewWorker(q, []Registerable{impl, impl})
		require.ErrorContains(t, err, "duplicate implementation for operation op")
	})
}

func TestAsyncHandlerOperationDeclaration(t *testing.T) {
	t.Run("basic declaration", func(t *testing.T) {
		svc := NewService("test-svc")
		op := NewAsyncHandlerOperation[testInput, testOutput](svc, "handler-op")
		require.Equal(t, "handler-op", op.Name())
	})

	t.Run("panics on non-struct param", func(t *testing.T) {
		svc := NewService("test-svc")
		require.Panics(t, func() {
			NewAsyncHandlerOperation[string, testOutput](svc, "op")
		})
	})

	t.Run("panics on duplicate name", func(t *testing.T) {
		svc := NewService("test-svc")
		NewAsyncHandlerOperation[testInput, testOutput](svc, "op")
		require.PanicsWithValue(t, "operation op already declared on service test-svc", func() {
			NewAsyncHandlerOperation[testInput, testOutput](svc, "op")
		})
	})

	t.Run("conflicts with other operation types", func(t *testing.T) {
		svc := NewService("test-svc")
		NewSyncOperation[testInput, testOutput](svc, "op")
		require.PanicsWithValue(t, "operation op already declared on service test-svc", func() {
			NewAsyncHandlerOperation[testInput, testOutput](svc, "op")
		})
	})
}

func TestAsyncHandlerOperationWithImplementation(t *testing.T) {
	t.Run("via WithImplementations", func(t *testing.T) {
		svc := NewService("test-svc")
		op := NewAsyncHandlerOperation[testInput, testOutput](svc, "handler-op")

		result, err := svc.WithImplementations(
			op.WithImplementation(func(ctx context.Context, input testInput, opts nexus.StartOperationOptions) (temporalnexus.WorkflowHandle[testOutput], error) {
				return nil, nil
			}),
		)
		require.NoError(t, err)
		require.NotNil(t, result)
	})

	t.Run("via NewWorker", func(t *testing.T) {
		q := NewQueue("test-q")
		svc := NewService("test-svc")
		op := NewAsyncHandlerOperation[testInput, testOutput](svc, "handler-op")

		wrk, err := NewWorker(q, []Registerable{
			op.WithImplementation(func(ctx context.Context, input testInput, opts nexus.StartOperationOptions) (temporalnexus.WorkflowHandle[testOutput], error) {
				return nil, nil
			}),
		})
		require.NoError(t, err)
		require.NotNil(t, wrk)
	})

	t.Run("missing handler op detected by NewWorker", func(t *testing.T) {
		q := NewQueue("test-q")
		svc := NewService("test-svc")
		NewAsyncHandlerOperation[testInput, testOutput](svc, "op1")
		op2 := NewSyncOperation[testInput, testOutput](svc, "op2")

		_, err := NewWorker(q, []Registerable{
			op2.WithImplementation(func(ctx context.Context, input testInput, opts nexus.StartOperationOptions) (testOutput, error) {
				return testOutput{}, nil
			}),
		})
		require.ErrorContains(t, err, "missing implementation for operation op1")
	})
}

func TestAsyncHandlerOperationClientValidation(t *testing.T) {
	t.Run("panics on mismatched service", func(t *testing.T) {
		svcA := NewService("service-a")
		svcB := NewService("service-b")

		opA := NewAsyncHandlerOperation[testInput, testOutput](svcA, "op")
		_ = NewAsyncHandlerOperation[testInput, testOutput](svcB, "op")

		clientB := &NexusClient{serviceName: svcB.name}

		require.PanicsWithValue(
			t,
			"cannot execute operation op on service service-a with client for service service-b",
			func() {
				opA.Execute(nil, clientB, testInput{}, workflow.NexusOperationOptions{})
			},
		)
	})

	t.Run("panics on nil client", func(t *testing.T) {
		svc := NewService("service-a")
		op := NewAsyncHandlerOperation[testInput, testOutput](svc, "op")

		require.PanicsWithValue(
			t,
			"cannot execute operation op on service service-a with nil Nexus client",
			func() {
				op.Execute(nil, nil, testInput{}, workflow.NexusOperationOptions{})
			},
		)
	})
}

func TestOperationPanicIfNotStruct(t *testing.T) {
	t.Run("sync non-struct param", func(t *testing.T) {
		svc := NewService("test-svc")
		require.Panics(t, func() {
			NewSyncOperation[string, testOutput](svc, "op")
		})
	})

	t.Run("async non-struct param", func(t *testing.T) {
		svc := NewService("test-svc")
		require.Panics(t, func() {
			NewAsyncOperation[int, testOutput](svc, "op")
		})
	})
}

func TestOperationExecutePanicsOnMismatchedClientService(t *testing.T) {
	t.Run("sync operation with client from different service", func(t *testing.T) {
		svcA := NewService("service-a")
		svcB := NewService("service-b")

		syncA := NewSyncOperation[testInput, testOutput](svcA, "shared-op")
		_ = NewSyncOperation[testInput, testOutput](svcB, "shared-op")

		clientB := &NexusClient{serviceName: svcB.name}

		require.PanicsWithValue(
			t,
			"cannot execute operation shared-op on service service-a with client for service service-b",
			func() {
				syncA.Execute(nil, clientB, testInput{}, workflow.NexusOperationOptions{})
			},
		)
	})

	t.Run("async operation with client from different service", func(t *testing.T) {
		svcA := NewService("service-a")
		svcB := NewService("service-b")

		asyncA := NewAsyncOperation[testInput, testOutput](svcA, "shared-op")
		_ = NewAsyncOperation[testInput, testOutput](svcB, "shared-op")

		clientB := &NexusClient{serviceName: svcB.name}

		require.PanicsWithValue(
			t,
			"cannot execute operation shared-op on service service-a with client for service service-b",
			func() {
				asyncA.Execute(nil, clientB, testInput{}, workflow.NexusOperationOptions{})
			},
		)
	})

	t.Run("nil client", func(t *testing.T) {
		svc := NewService("service-a")
		syncOp := NewSyncOperation[testInput, testOutput](svc, "op")

		require.PanicsWithValue(
			t,
			"cannot execute operation op on service service-a with nil Nexus client",
			func() {
				syncOp.Execute(nil, nil, testInput{}, workflow.NexusOperationOptions{})
			},
		)
	})
}
