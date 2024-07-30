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

var (
	workflowTypeFormatAndGreet        = tempts.NewWorkflow[string, string](queueMain, "format_and_greet")
	workflowTypeFormatAndGreetGetName = tempts.NewQueryHandler[struct{}, string]("get_formatted_name")
	activityTypeFormatName            = tempts.NewActivity[string, string](queueMain, "format_name")
)

var (
	workflowTypeJustGreet = tempts.NewWorkflow[string, string](queueMain, "greet")
	activityTypeGreet     = tempts.NewActivity[string, string](queueMain, "greet")
)

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
	workflowHandle, err := workflowTypeFormatAndGreet.Execute(ctx, c, client.StartWorkflowOptions{}, "Viktor")
	if err != nil {
		panic(err)
	}
	newName, err := workflowTypeFormatAndGreetGetName.Query(ctx, c, workflowHandle.GetID(), workflowHandle.GetRunID(), struct{}{})
	if err != nil {
		panic(err)
	}
	fmt.Printf("Expecting unknown name from the query: %s\n", newName)

	err = workflowHandle.Get(ctx, &newName)
	if err != nil {
		panic(err)
	}
	fmt.Printf("Name returned from the workflow: %s\n", newName)
	err = workflowTypeFormatAndGreet.SetSchedule(ctx, c, client.ScheduleOptions{
		ID: "every5s",
		Spec: client.ScheduleSpec{
			Intervals: []client.ScheduleIntervalSpec{
				{
					Every: time.Second * 5,
				},
			},
		},
	}, "Viktor")
	if err != nil {
		panic(err)
	}
	time.Sleep(time.Second * 10)
}

func activityFormatName(ctx context.Context, input string) (string, error) {
	return strings.ToUpper(input), nil
}

func workflowFormatAndGreet(ctx workflow.Context, name string) (string, error) {
	newName := "unknown"
	workflowTypeFormatAndGreetGetName.SetHandler(ctx, func(struct{}) (string, error) {
		return newName, nil
	})

	// Give the example code a chance to read the "unknown" name with the query and update it
	workflow.Sleep(ctx, time.Second*1)

	newName, err := activityTypeFormatName.Run(ctx, name)
	if err != nil {
		return "", err
	}
	// Give the caller a chance to do an update
	workflow.Sleep(ctx, time.Second*1)

	final, err := workflowTypeJustGreet.RunChild(ctx, workflow.ChildWorkflowOptions{}, newName)
	if err != nil {
		return "", err
	}
	fmt.Println("final", final)
	return newName, nil
}

func workflowJustGreet(ctx workflow.Context, name string) (string, error) {
	return activityTypeGreet.Run(ctx, name)
}

func activityGreet(ctx context.Context, name string) (string, error) {
	fmt.Printf("Hello %s\n", name)
	return name, nil
}
