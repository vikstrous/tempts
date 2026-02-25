package tempts

import (
	"context"
	"fmt"

	"github.com/nexus-rpc/sdk-go/nexus"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/temporalnexus"
	"go.temporal.io/sdk/worker"
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
// It implements Registerable and can be passed directly into NewWorker, or used with
// Service.WithImplementations() for pre-validation.
type OperationWithImpl interface {
	getOperationName() string
	getService() *Service
	toNexusOperation() (nexus.RegisterableOperation, error)
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

func (op *SyncOperationWithImpl[Param, Return]) toNexusOperation() (nexus.RegisterableOperation, error) {
	return nexus.NewSyncOperation(op.op.name, op.handler), nil
}

func (op *SyncOperationWithImpl[Param, Return]) register(_ worker.Registry) {}

func (op *SyncOperationWithImpl[Param, Return]) validate(v *validationState) error {
	if _, ok := v.nexusServicesHandled[op.op.service.name]; ok {
		return fmt.Errorf("nexus service %s has both bundled and individual operation implementations; use one or the other", op.op.service.name)
	}
	v.nexusOps[op.op.service.name] = append(v.nexusOps[op.op.service.name], op)
	return nil
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

func (op *AsyncOperationWithImpl[Param, Return]) toNexusOperation() (nexus.RegisterableOperation, error) {
	return temporalnexus.NewWorkflowRunOperation(
		op.op.name,
		op.workflow,
		op.getOptions,
	), nil
}

func (op *AsyncOperationWithImpl[Param, Return]) register(_ worker.Registry) {}

func (op *AsyncOperationWithImpl[Param, Return]) validate(v *validationState) error {
	if _, ok := v.nexusServicesHandled[op.op.service.name]; ok {
		return fmt.Errorf("nexus service %s has both bundled and individual operation implementations; use one or the other", op.op.service.name)
	}
	v.nexusOps[op.op.service.name] = append(v.nexusOps[op.op.service.name], op)
	return nil
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

// AsyncHandlerOperation represents an asynchronous Nexus operation with a custom handler.
// Unlike AsyncOperation, the handler has full control over workflow execution,
// allowing the operation input type to differ from the workflow input type.
type AsyncHandlerOperation[Param, Return any] struct {
	name    string
	service *Service
}

// NewAsyncHandlerOperation declares an asynchronous handler operation on the given service.
func NewAsyncHandlerOperation[Param, Return any](s *Service, name string) AsyncHandlerOperation[Param, Return] {
	panicIfNotStruct[Param]("NewAsyncHandlerOperation")
	if _, exists := s.operations[name]; exists {
		panic(fmt.Sprintf("operation %s already declared on service %s", name, s.name))
	}
	s.operations[name] = struct{}{}
	return AsyncHandlerOperation[Param, Return]{
		name:    name,
		service: s,
	}
}

// Name returns the name of the operation.
func (op AsyncHandlerOperation[Param, Return]) Name() string {
	return op.name
}

// AsyncHandlerOperationWithImpl holds an async handler operation along with its handler.
type AsyncHandlerOperationWithImpl[Param, Return any] struct {
	op      AsyncHandlerOperation[Param, Return]
	handler func(context.Context, Param, nexus.StartOperationOptions) (temporalnexus.WorkflowHandle[Return], error)
}

func (op *AsyncHandlerOperationWithImpl[Param, Return]) getOperationName() string {
	return op.op.name
}

func (op *AsyncHandlerOperationWithImpl[Param, Return]) getService() *Service {
	return op.op.service
}

func (op *AsyncHandlerOperationWithImpl[Param, Return]) toNexusOperation() (nexus.RegisterableOperation, error) {
	nop, err := temporalnexus.NewWorkflowRunOperationWithOptions(
		temporalnexus.WorkflowRunOperationOptions[Param, Return]{
			Name:    op.op.name,
			Handler: op.handler,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create nexus operation %s: %w", op.op.name, err)
	}
	return nop, nil
}

func (op *AsyncHandlerOperationWithImpl[Param, Return]) register(_ worker.Registry) {}

func (op *AsyncHandlerOperationWithImpl[Param, Return]) validate(v *validationState) error {
	if _, ok := v.nexusServicesHandled[op.op.service.name]; ok {
		return fmt.Errorf("nexus service %s has both bundled and individual operation implementations; use one or the other", op.op.service.name)
	}
	v.nexusOps[op.op.service.name] = append(v.nexusOps[op.op.service.name], op)
	return nil
}

// WithImplementation attaches a handler to an async handler operation.
// The handler receives the operation input and start options, and returns a WorkflowHandle
// by calling temporalnexus.ExecuteWorkflow (or ExecuteUntypedWorkflow) to start any workflow
// with any input type.
func (op AsyncHandlerOperation[Param, Return]) WithImplementation(
	handler func(context.Context, Param, nexus.StartOperationOptions) (temporalnexus.WorkflowHandle[Return], error),
) *AsyncHandlerOperationWithImpl[Param, Return] {
	return &AsyncHandlerOperationWithImpl[Param, Return]{
		op:      op,
		handler: handler,
	}
}

// Run synchronously executes an async handler operation and returns the result.
func (op AsyncHandlerOperation[Param, Return]) Run(
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

// Execute asynchronously starts an async handler operation and returns a future.
func (op AsyncHandlerOperation[Param, Return]) Execute(
	ctx workflow.Context,
	c *NexusClient,
	param Param,
	opts workflow.NexusOperationOptions,
) workflow.NexusOperationFuture {
	if err := validateServiceMatch(op.name, op.service, c); err != nil {
		panic(err.Error())
	}
	return c.client.ExecuteOperation(ctx, op.name, param, opts)
}

// ServiceWithImpl holds a service along with all its operation implementations.
// It implements Registerable and can be passed directly into NewWorker alongside
// activities and workflows.
type ServiceWithImpl struct {
	service *Service
	impl    *nexus.Service
}

func (s *ServiceWithImpl) register(ar worker.Registry) {
	if rns, ok := ar.(interface{ RegisterNexusService(*nexus.Service) }); ok {
		rns.RegisterNexusService(s.impl)
	}
}

func (s *ServiceWithImpl) validate(v *validationState) error {
	if _, ok := v.nexusServicesHandled[s.service.name]; ok {
		return fmt.Errorf("duplicate registration for nexus service %s", s.service.name)
	}
	if _, ok := v.nexusOps[s.service.name]; ok {
		return fmt.Errorf("nexus service %s has both bundled and individual operation implementations; use one or the other", s.service.name)
	}
	v.nexusServicesHandled[s.service.name] = struct{}{}
	return nil
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
		if svc.name != s.name {
			return nil, fmt.Errorf("operation %s belongs to service %s, not %s", name, svc.name, s.name)
		}

		// Check for duplicates
		if _, exists := provided[name]; exists {
			return nil, fmt.Errorf("duplicate implementation for operation %s on service %s", name, s.name)
		}
		provided[name] = struct{}{}

		nop, err := op.toNexusOperation()
		if err != nil {
			return nil, err
		}
		if err := impl.Register(nop); err != nil {
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
func (s *Service) NewClient(endpoint string) *NexusClient {
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

// Execute starts a sync operation and returns a future for the result.
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
