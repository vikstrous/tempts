package tempts

import (
	"context"
	"fmt"

	"github.com/nexus-rpc/sdk-go/nexus"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/temporalnexus"
	"go.temporal.io/sdk/workflow"
)

// Service represents a Nexus service declaration, analogous to Queue for activities/workflows.
// It tracks which operations belong to the service and ensures type safety at compile time.
type Service struct {
	name       string
	operations map[string]struct{}
}

// NewService declares a new Nexus service with the given name.
func NewService(name string) *Service {
	return &Service{
		name:       name,
		operations: make(map[string]struct{}),
	}
}

// Name returns the name of the service.
func (s *Service) Name() string {
	return s.name
}

// OperationWithImpl is implemented by operations that have an implementation attached.
// It's used as a parameter to Service.WithImplementations().
type OperationWithImpl interface {
	getOperationName() string
	getService() *Service
	toNexusOperation() nexus.RegisterableOperation
}

// SyncOperation represents a synchronous Nexus operation (simple RPC-style).
// It is parameterized by input and output types for compile-time type safety.
type SyncOperation[Param, Return any] struct {
	name    string
	service *Service
}

// NewSyncOperation declares a synchronous operation on the given service.
// Sync operations are simple request-response style handlers.
func NewSyncOperation[Param, Return any](s *Service, name string) SyncOperation[Param, Return] {
	panicIfNotStruct[Param]("NewSyncOperation")
	if _, exists := s.operations[name]; exists {
		panic(fmt.Sprintf("operation %s already declared on service %s", name, s.name))
	}
	s.operations[name] = struct{}{}
	return SyncOperation[Param, Return]{
		name:    name,
		service: s,
	}
}

// Name returns the name of the operation.
func (op SyncOperation[Param, Return]) Name() string {
	return op.name
}

// SyncOperationWithImpl holds a sync operation along with its implementation.
type SyncOperationWithImpl[Param, Return any] struct {
	op      SyncOperation[Param, Return]
	handler func(context.Context, Param, nexus.StartOperationOptions) (Return, error)
}

func (op *SyncOperationWithImpl[Param, Return]) getOperationName() string {
	return op.op.name
}

func (op *SyncOperationWithImpl[Param, Return]) getService() *Service {
	return op.op.service
}

func (op *SyncOperationWithImpl[Param, Return]) toNexusOperation() nexus.RegisterableOperation {
	return nexus.NewSyncOperation(op.op.name, op.handler)
}

// WithImplementation attaches an implementation to a sync operation.
// The handler receives the input parameter and start options, and returns the result.
func (op SyncOperation[Param, Return]) WithImplementation(
	handler func(context.Context, Param, nexus.StartOperationOptions) (Return, error),
) *SyncOperationWithImpl[Param, Return] {
	return &SyncOperationWithImpl[Param, Return]{
		op:      op,
		handler: handler,
	}
}

// AsyncOperation represents an asynchronous Nexus operation (workflow-backed).
// It is parameterized by input and output types for compile-time type safety.
type AsyncOperation[Param, Return any] struct {
	name    string
	service *Service
}

// NewAsyncOperation declares an asynchronous operation on the given service.
// Async operations are backed by workflows and can run for extended periods.
func NewAsyncOperation[Param, Return any](s *Service, name string) AsyncOperation[Param, Return] {
	panicIfNotStruct[Param]("NewAsyncOperation")
	if _, exists := s.operations[name]; exists {
		panic(fmt.Sprintf("operation %s already declared on service %s", name, s.name))
	}
	s.operations[name] = struct{}{}
	return AsyncOperation[Param, Return]{
		name:    name,
		service: s,
	}
}

// Name returns the name of the operation.
func (op AsyncOperation[Param, Return]) Name() string {
	return op.name
}

// AsyncOperationWithImpl holds an async operation along with its workflow implementation.
type AsyncOperationWithImpl[Param, Return any] struct {
	op         AsyncOperation[Param, Return]
	workflow   func(workflow.Context, Param) (Return, error)
	getOptions func(context.Context, Param, nexus.StartOperationOptions) (client.StartWorkflowOptions, error)
}

func (op *AsyncOperationWithImpl[Param, Return]) getOperationName() string {
	return op.op.name
}

func (op *AsyncOperationWithImpl[Param, Return]) getService() *Service {
	return op.op.service
}

func (op *AsyncOperationWithImpl[Param, Return]) toNexusOperation() nexus.RegisterableOperation {
	return temporalnexus.NewWorkflowRunOperation(
		op.op.name,
		op.workflow,
		op.getOptions,
	)
}

// WithImplementation attaches a workflow implementation to an async operation.
// The workflow function handles the operation logic, and getOptions provides
// workflow options based on the input and start options.
func (op AsyncOperation[Param, Return]) WithImplementation(
	workflow func(workflow.Context, Param) (Return, error),
	getOptions func(context.Context, Param, nexus.StartOperationOptions) (client.StartWorkflowOptions, error),
) *AsyncOperationWithImpl[Param, Return] {
	return &AsyncOperationWithImpl[Param, Return]{
		op:         op,
		workflow:   workflow,
		getOptions: getOptions,
	}
}

// ServiceWithImpl holds a service along with all its operation implementations.
// It is passed to NewWorker to register the Nexus service.
type ServiceWithImpl struct {
	service *Service
	impl    *nexus.Service
}

// WithImplementations validates and bundles operation implementations for the service.
// It ensures:
// - All declared operations have implementations
// - No extra implementations are provided
// - No duplicate implementations
func (s *Service) WithImplementations(ops ...OperationWithImpl) (*ServiceWithImpl, error) {
	provided := make(map[string]struct{})

	impl := nexus.NewService(s.name)

	for _, op := range ops {
		name := op.getOperationName()
		svc := op.getService()

		// Verify the operation belongs to this service
		if svc != s {
			return nil, fmt.Errorf("operation %s belongs to service %s, not %s", name, svc.name, s.name)
		}

		// Check for duplicates
		if _, exists := provided[name]; exists {
			return nil, fmt.Errorf("duplicate implementation for operation %s on service %s", name, s.name)
		}
		provided[name] = struct{}{}

		if err := impl.Register(op.toNexusOperation()); err != nil {
			return nil, fmt.Errorf("failed to register operation %s: %w", name, err)
		}
	}

	// Verify all declared operations have implementations
	for opName := range s.operations {
		if _, exists := provided[opName]; !exists {
			return nil, fmt.Errorf("missing implementation for operation %s on service %s", opName, s.name)
		}
	}

	return &ServiceWithImpl{
		service: s,
		impl:    impl,
	}, nil
}

// NexusClient is used to call Nexus operations from within a workflow.
type NexusClient struct {
	client      workflow.NexusClient
	serviceName string
}

// NewClient creates a NexusClient for calling operations on this service.
// The endpoint parameter specifies the Nexus endpoint to use.
func (s *Service) NewClient(ctx workflow.Context, endpoint string) *NexusClient {
	return &NexusClient{
		client:      workflow.NewNexusClient(endpoint, s.name),
		serviceName: s.name,
	}
}

func validateServiceMatch(opName string, opService *Service, c *NexusClient) error {
	if c == nil {
		return fmt.Errorf("cannot execute operation %s on service %s with nil Nexus client", opName, opService.name)
	}
	if c.serviceName != opService.name {
		return fmt.Errorf(
			"cannot execute operation %s on service %s with client for service %s",
			opName,
			opService.name,
			c.serviceName,
		)
	}
	return nil
}

// Run synchronously executes a sync operation and returns the result.
func (op SyncOperation[Param, Return]) Run(
	ctx workflow.Context,
	c *NexusClient,
	param Param,
	opts workflow.NexusOperationOptions,
) (Return, error) {
	var result Return
	future := op.Execute(ctx, c, param, opts)
	err := future.Get(ctx, &result)
	return result, err
}

// Execute asynchronously starts a sync operation and returns a future.
func (op SyncOperation[Param, Return]) Execute(
	ctx workflow.Context,
	c *NexusClient,
	param Param,
	opts workflow.NexusOperationOptions,
) workflow.NexusOperationFuture {
	// ExecuteOperation does not return an error, so we panic with an explicit
	// message for invalid client/service pairing to fail fast during workflow execution.
	if err := validateServiceMatch(op.name, op.service, c); err != nil {
		panic(err.Error())
	}
	return c.client.ExecuteOperation(ctx, op.name, param, opts)
}

// Run synchronously executes an async operation and returns the result.
func (op AsyncOperation[Param, Return]) Run(
	ctx workflow.Context,
	c *NexusClient,
	param Param,
	opts workflow.NexusOperationOptions,
) (Return, error) {
	var result Return
	future := op.Execute(ctx, c, param, opts)
	err := future.Get(ctx, &result)
	return result, err
}

// Execute asynchronously starts an async operation and returns a future.
func (op AsyncOperation[Param, Return]) Execute(
	ctx workflow.Context,
	c *NexusClient,
	param Param,
	opts workflow.NexusOperationOptions,
) workflow.NexusOperationFuture {
	// ExecuteOperation does not return an error, so we panic with an explicit
	// message for invalid client/service pairing to fail fast during workflow execution.
	if err := validateServiceMatch(op.name, op.service, c); err != nil {
		panic(err.Error())
	}
	return c.client.ExecuteOperation(ctx, op.name, param, opts)
}
