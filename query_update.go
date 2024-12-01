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
	panicIfNotStruct[Param]("NewQuery")
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
