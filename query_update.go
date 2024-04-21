package tempts

import (
	"context"

	"go.temporal.io/sdk/workflow"
)

// QueryHandler is used for interacting with queries on workflows.
// Currently there's no way to enforce the connection between the query and the workflow it should be valid on.
type QueryHandler[Param, Return any] struct {
	name string
}

// NewQueryHandler declares the name and types for a query to a workflow.
func NewQueryHandler[Param, Return any](queryName string) *QueryHandler[Param, Return] {
	return &QueryHandler[Param, Return]{name: queryName}
}

// SetHandler should be called by a workflow to define how the query should be handled when sent to this workflow to execute.
func (q *QueryHandler[Param, Return]) SetHandler(ctx workflow.Context, fn func(Param) (Return, error)) {
	// This can't error because the type is enforced by the signature of this function
	workflow.SetQueryHandler(ctx, q.name, fn)
}

// Query executes the query and returns the response.
func (q *QueryHandler[Param, Return]) Query(ctx context.Context, temporalClient *Client, workflowID, runID string, p Param) (Return, error) {
	var value Return
	response, err := temporalClient.Client.QueryWorkflow(ctx, workflowID, runID, q.name, p)
	if err != nil {
		return value, err
	}
	err = response.Get(&value)
	if err != nil {
		return value, err
	}
	return value, nil
}

// UpdateHandler is used for interacting with updates on workflows.
// Currently there's no way to enforce the connection between the update and the workflow it should be valid on.
type UpdateHandler[Param, Return any] struct {
	name string
}

// NewUpdateHandler declares the name and types for an update to a workflow.
func NewUpdateHandler[Param, Return any](updateName string) *UpdateHandler[Param, Return] {
	return &UpdateHandler[Param, Return]{name: updateName}
}

// SetHandler should be called by a workflow to define how the update should be handled when sent to this workflow to execute.
func (q *UpdateHandler[Param, Return]) SetHandler(ctx workflow.Context, fn func(workflow.Context, Param) (Return, error)) {
	// This can't error because the type is enforced by the signature of this function
	workflow.SetUpdateHandler(ctx, q.name, fn)
}

// SetHandlerWithValidator should be called by a workflow to define how the query should be handled when sent to this workflow to execute.
// This is the same as SetHandler, except that it gives access to the experimental Validator function which can be used to validate the request before it's executed.
func (q *UpdateHandler[Param, Return]) SetHandlerWithValidator(ctx workflow.Context, fn func(workflow.Context, Param) (Return, error), validator func(workflow.Context, Param) error) {
	// This can't error because the type is enforced by the signature of this function
	workflow.SetUpdateHandlerWithOptions(ctx, q.name, fn, workflow.UpdateHandlerOptions{
		Validator: validator,
	})
}

// Update executes the update and returns the response.
func (q *UpdateHandler[Param, Return]) Update(ctx context.Context, temporalClient *Client, workflowID, runID string, p Param) (Return, error) {
	var value Return
	response, err := temporalClient.Client.UpdateWorkflow(ctx, workflowID, runID, q.name, p)
	if err != nil {
		return value, err
	}
	err = response.Get(ctx, &value)
	if err != nil {
		return value, err
	}
	return value, nil
}
