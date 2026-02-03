package main

import (
	"context"

	"github.com/nexus-rpc/sdk-go/nexus"
	"github.com/vikstrous/tempts"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/workflow"
)

// Service declaration - this would typically be in a shared API package
var myNexusService = tempts.NewService("my-nexus-service")

// EchoInput is the input for the echo operation
type EchoInput struct {
	Message string
}

// EchoOutput is the output from the echo operation
type EchoOutput struct {
	Message string
}

// Sync operation declaration - simple RPC-style
var echoOp = tempts.NewSyncOperation[EchoInput, EchoOutput](myNexusService, "echo")

// ProcessInput is the input for the process operation
type ProcessInput struct {
	Data string
}

// ProcessOutput is the output from the process operation
type ProcessOutput struct {
	Result string
}

// Async operation declaration - workflow-backed, can run for extended periods
var processOp = tempts.NewAsyncOperation[ProcessInput, ProcessOutput](myNexusService, "process")

// Handler implementations - this would typically be in a handler package

// echoHandler is the implementation for the echo sync operation
func echoHandler(ctx context.Context, input EchoInput, opts nexus.StartOperationOptions) (EchoOutput, error) {
	return EchoOutput{Message: "Echo: " + input.Message}, nil
}

// processWorkflow is the workflow that backs the async operation
func processWorkflow(ctx workflow.Context, input ProcessInput) (ProcessOutput, error) {
	// In a real implementation, this would do actual work
	return ProcessOutput{Result: "Processed: " + input.Data}, nil
}

// processGetOptions returns the workflow options for the async operation
func processGetOptions(ctx context.Context, input ProcessInput, opts nexus.StartOperationOptions) (client.StartWorkflowOptions, error) {
	return client.StartWorkflowOptions{
		// Use the RequestID to ensure idempotency
		ID: "process-" + opts.RequestID,
	}, nil
}

// createNexusService creates the service with all implementations
func createNexusService() (*tempts.ServiceWithImpl, error) {
	return myNexusService.WithImplementations(
		echoOp.WithImplementation(echoHandler),
		processOp.WithImplementation(processWorkflow, processGetOptions),
	)
}

// exampleCallerWorkflow demonstrates calling Nexus operations from a workflow
func exampleCallerWorkflow(ctx workflow.Context, message string) (string, error) {
	// Create a client for the Nexus service
	c := myNexusService.NewClient(ctx, "my-nexus-endpoint")

	// Call the sync operation
	echoResult, err := echoOp.Run(ctx, c, EchoInput{Message: message}, workflow.NexusOperationOptions{})
	if err != nil {
		return "", err
	}

	// Call the async operation
	processResult, err := processOp.Run(ctx, c, ProcessInput{Data: echoResult.Message}, workflow.NexusOperationOptions{})
	if err != nil {
		return "", err
	}

	return processResult.Result, nil
}

// exampleAsyncCallerWorkflow demonstrates async execution of Nexus operations
func exampleAsyncCallerWorkflow(ctx workflow.Context, message string) (string, error) {
	c := myNexusService.NewClient(ctx, "my-nexus-endpoint")

	// Start the operation asynchronously
	future := processOp.Execute(ctx, c, ProcessInput{Data: message}, workflow.NexusOperationOptions{})

	// Do other work here...

	// Wait for the result
	var result ProcessOutput
	if err := future.Get(ctx, &result); err != nil {
		return "", err
	}

	return result.Result, nil
}

// This example shows how to set up a worker with Nexus services
func exampleNexusWorkerSetup() {
	// Create the service with implementations
	svc, err := createNexusService()
	if err != nil {
		panic(err)
	}

	// Create a worker with the Nexus service
	// The processWorkflow needs to be registered as a regular workflow too
	_, err = tempts.NewWorker(queueMain, []tempts.Registerable{
		// Register any activities and workflows needed
	}, svc)
	if err != nil {
		panic(err)
	}
}
