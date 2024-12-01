package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/vikstrous/tempts"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"
)

var queueMain = tempts.NewQueue("main")

type FormatNameParams struct {
	Name string
}

type FormatNameResult struct {
	Name string
}

var activityTypeFormatName = tempts.NewActivity[FormatNameParams, FormatNameResult](queueMain, "format_name")

type FormatAndGreetParams struct {
	Name string
}

type FormatAndGreetResult struct {
	Name string
}

var workflowTypeFormatAndGreet = tempts.NewWorkflow[FormatAndGreetParams, FormatAndGreetResult](queueMain, "format_and_greet")

type FormatAndGreetGetNameResult struct {
	Name string
}

var workflowTypeFormatAndGreetGetName = tempts.NewQueryHandler[struct{}, FormatAndGreetGetNameResult]("get_formatted_name")

type JustGreetParams struct {
	Name string
}

type JustGreetResult struct {
	Name string
}

var workflowTypeJustGreet = tempts.NewWorkflow[JustGreetParams, JustGreetResult](queueMain, "greet")

type GreetParams struct {
	Name string
}

type GreetResult struct {
	Name string
}

var activityTypeGreet = tempts.NewActivity[GreetParams, GreetResult](queueMain, "greet")

func main() {
	c, err := tempts.Dial(client.Options{})
	if err != nil {
		panic(err)
	}
	defer c.Close()
	wrk, err := tempts.NewWorker(queueMain, []tempts.Registerable{
		activityTypeFormatName.WithImplementation(activityFormatName),
		activityTypeGreet.WithImplementation(activityGreet),
		workflowTypeFormatAndGreet.WithImplementation(workflowFormatAndGreet),
		workflowTypeJustGreet.WithImplementation(workflowJustGreet),
	})
	if err != nil {
		panic(err)
	}
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	go func() {
		err = wrk.Run(ctx, c, worker.Options{})
		if err != nil {
			panic(err)
		}
	}()
	workflowHandle, err := workflowTypeFormatAndGreet.Execute(ctx, c, client.StartWorkflowOptions{}, FormatAndGreetParams{Name: "Viktor"})
	if err != nil {
		panic(err)
	}
	queryResult, err := workflowTypeFormatAndGreetGetName.Query(ctx, c, workflowHandle.GetID(), workflowHandle.GetRunID(), struct{}{})
	if err != nil {
		panic(err)
	}
	fmt.Printf("Expecting unknown name from the query: %s\n", queryResult.Name)

	var result FormatAndGreetResult
	err = workflowHandle.Get(ctx, &result)
	if err != nil {
		panic(err)
	}
	fmt.Printf("Name returned from the workflow: %s\n", result.Name)
	err = workflowTypeFormatAndGreet.SetSchedule(ctx, c, client.ScheduleOptions{
		ID: "every5s",
		Spec: client.ScheduleSpec{
			Intervals: []client.ScheduleIntervalSpec{
				{
					Every: time.Second * 5,
				},
			},
		},
	}, FormatAndGreetParams{Name: "Viktor"})
	if err != nil {
		panic(err)
	}
	time.Sleep(time.Second * 10)
}

func activityFormatName(ctx context.Context, input FormatNameParams) (FormatNameResult, error) {
	return FormatNameResult{Name: strings.ToUpper(input.Name)}, nil
}

func workflowFormatAndGreet(ctx workflow.Context, params FormatAndGreetParams) (FormatAndGreetResult, error) {
	newName := "unknown"
	workflowTypeFormatAndGreetGetName.SetHandler(ctx, func(struct{}) (FormatAndGreetGetNameResult, error) {
		return FormatAndGreetGetNameResult{Name: newName}, nil
	})

	// Give the example code a chance to read the "unknown" name with the query and update it
	workflow.Sleep(ctx, time.Second*1)

	formatNameResult, err := activityTypeFormatName.Run(ctx, FormatNameParams{Name: params.Name})
	if err != nil {
		return FormatAndGreetResult{}, fmt.Errorf("failed to format name: %w", err)
	}
	newName = formatNameResult.Name

	// Give the caller a chance to do an update
	workflow.Sleep(ctx, time.Second*1)

	final, err := workflowTypeJustGreet.RunChild(ctx, workflow.ChildWorkflowOptions{}, JustGreetParams{Name: newName})
	if err != nil {
		return FormatAndGreetResult{}, fmt.Errorf("failed to greet: %w", err)
	}
	fmt.Println("final", final)
	return FormatAndGreetResult{Name: newName}, nil
}

func workflowJustGreet(ctx workflow.Context, params JustGreetParams) (JustGreetResult, error) {
	name, err := activityTypeGreet.Run(ctx, GreetParams{Name: params.Name})
	return JustGreetResult{Name: name.Name}, err
}

func activityGreet(ctx context.Context, params GreetParams) (GreetResult, error) {
	fmt.Printf("Hello %s\n", params.Name)
	return GreetResult{Name: params.Name}, nil
}
