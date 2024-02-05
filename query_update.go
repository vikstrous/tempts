package tstemporal

import (
	"context"

	"go.temporal.io/sdk/workflow"
)

// TODO: there's nothing that ensures that query handlers are defined by the workflows they should be defined on. We might be able to ensure that during the initial steps of a workflow, necessary query handlers are registered

type QueryHandler0[Return any] struct {
	name string
}

func NewQueryHandler0[Return any](queryName string) *QueryHandler0[Return] {
	return &QueryHandler0[Return]{name: queryName}
}

func (q *QueryHandler0[Return]) SetHandler(ctx workflow.Context, fn func() (Return, error)) {
	// This can't error because the type is enforced by the signature of this function
	workflow.SetQueryHandler(ctx, q.name, fn)
}

func (q *QueryHandler0[Return]) Query(ctx context.Context, temporalClient *Client, workflowID, runID string) (Return, error) {
	var value Return
	response, err := temporalClient.Client.QueryWorkflow(ctx, workflowID, runID, q.name)
	if err != nil {
		return value, err
	}
	err = response.Get(&value)
	if err != nil {
		return value, err
	}
	return value, nil
}

type QueryHandler1R[Param, Return any] struct {
	name string
}

func NewQueryHandler1R[Param, Return any](queryName string) *QueryHandler1R[Param, Return] {
	return &QueryHandler1R[Param, Return]{name: queryName}
}

func (q *QueryHandler1R[Param, Return]) SetHandler(ctx workflow.Context, fn func(Param) (Return, error)) {
	// This can't error because the type is enforced by the signature of this function
	workflow.SetQueryHandler(ctx, q.name, fn)
}

func (q *QueryHandler1R[Param, Return]) Query(ctx context.Context, temporalClient *Client, workflowID, runID string, p Param) (Return, error) {
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

type UpdateHandler1R[Param, Return any] struct {
	name string
}

func NewUpdateHandler1R[Param, Return any](queryName string) *UpdateHandler1R[Param, Return] {
	return &UpdateHandler1R[Param, Return]{name: queryName}
}

func (q *UpdateHandler1R[Param, Return]) SetHandler(ctx workflow.Context, fn func(workflow.Context, Param) (Return, error)) {
	// This can't error because the type is enforced by the signature of this function
	workflow.SetUpdateHandler(ctx, q.name, fn)
}

func (q *UpdateHandler1R[Param, Return]) SetHandlerWithValidator(ctx workflow.Context, fn func(workflow.Context, Param) (Return, error), validator func(workflow.Context, Param) error) {
	// This can't error because the type is enforced by the signature of this function
	workflow.SetUpdateHandlerWithOptions(ctx, q.name, fn, workflow.UpdateHandlerOptions{
		Validator: validator,
	})
}

func (q *UpdateHandler1R[Param, Return]) Update(ctx context.Context, temporalClient *Client, workflowID, runID string, p Param) (Return, error) {
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
