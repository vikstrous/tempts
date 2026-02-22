package tempts

import (
	"context"
	"testing"

	"github.com/nexus-rpc/sdk-go/nexus"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/client"
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
