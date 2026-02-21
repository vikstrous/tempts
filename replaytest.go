package tempts

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/gogo/protobuf/proto"
	"go.temporal.io/api/enums/v1"
	"go.temporal.io/api/filter/v1"
	"go.temporal.io/api/history/v1"
	"go.temporal.io/api/workflowservice/v1"
	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"
)

// GetWorkflowHistoriesBundle connects to the temporal server and fetches the most recent 10 open and 10 closed executions.
// It returns a byte seralized piece of data that can be used immediately or in the future to call ReplayWorkflow.
func GetWorkflowHistoriesBundle(ctx context.Context, client *Client, w WorkflowDeclaration) ([]byte, error) {
	closedExecutions, err := client.Client.WorkflowService().ListClosedWorkflowExecutions(ctx, &workflowservice.ListClosedWorkflowExecutionsRequest{
		Namespace:       client.namespace,
		MaximumPageSize: 10,
		Filters: &workflowservice.ListClosedWorkflowExecutionsRequest_TypeFilter{
			TypeFilter: &filter.WorkflowTypeFilter{Name: w.Name()},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get closed executions: %w", err)
	}
	openExecutions, err := client.Client.WorkflowService().ListOpenWorkflowExecutions(ctx, &workflowservice.ListOpenWorkflowExecutionsRequest{
		Namespace:       client.namespace,
		MaximumPageSize: 10,
		Filters: &workflowservice.ListOpenWorkflowExecutionsRequest_TypeFilter{
			TypeFilter: &filter.WorkflowTypeFilter{Name: w.Name()},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get open executions: %w", err)
	}
	allExecutions := append(closedExecutions.Executions, openExecutions.Executions...)
	hists := []*history.History{}
	for _, e := range allExecutions {
		var hist history.History
		fmt.Println(e.Execution)
		iter := client.Client.GetWorkflowHistory(ctx, e.Execution.WorkflowId, e.Execution.RunId, false, enums.HISTORY_EVENT_FILTER_TYPE_ALL_EVENT)
		for iter.HasNext() {
			event, err := iter.Next()
			if err != nil {
				return nil, fmt.Errorf("failed to get history: %w", err)
			}
			hist.Events = append(hist.Events, event)
		}
		hists = append(hists, &hist)
	}

	historiesData := historiesData{
		WorkflowName: w.Name(),
	}
	for i, h := range hists {
		if len(h.Events) < 3 {
			// The relay code requires history to have at least 3 events, so 2 even histories are considered invalid.
			continue
		}
		hBytes, err := proto.Marshal(h)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal history: %s", err)
		}
		historiesData.Histories = append(historiesData.Histories, historyWithMetadata{
			WorkflowID:   allExecutions[i].Execution.WorkflowId,
			RunID:        allExecutions[i].Execution.RunId,
			HistoryBytes: hBytes,
		})
	}
	historiesBytes, err := json.Marshal(historiesData)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal histories: %w", err)
	}
	return historiesBytes, nil
}

type historyWithMetadata struct {
	WorkflowID   string
	RunID        string
	HistoryBytes []byte
}
type historiesData struct {
	Histories    []historyWithMetadata
	WorkflowName string
}

// FilterWorkflowHistoriesBundle filters a histories bundle to keep only executions where tryReplay
// returns nil. It returns the filtered bundle as serialized bytes. If no executions pass the filter,
// it returns an error.
func FilterWorkflowHistoriesBundle(bundle []byte, tryReplay func(singleExecutionBundle []byte) error) ([]byte, int, error) {
	var data historiesData
	if err := json.Unmarshal(bundle, &data); err != nil {
		return nil, 0, fmt.Errorf("failed to unmarshal histories for filtering: %w", err)
	}

	var compatible []historyWithMetadata
	for _, h := range data.Histories {
		singleBundle, err := json.Marshal(historiesData{
			WorkflowName: data.WorkflowName,
			Histories:    []historyWithMetadata{h},
		})
		if err != nil {
			return nil, 0, fmt.Errorf("failed to marshal single history for filtering: %w", err)
		}

		if tryReplay(singleBundle) == nil {
			compatible = append(compatible, h)
		}
	}

	if len(compatible) == 0 {
		return nil, len(data.Histories), fmt.Errorf(
			"no compatible histories found for %s (all %d executions failed replay)",
			data.WorkflowName, len(data.Histories),
		)
	}

	data.Histories = compatible
	filtered, err := json.Marshal(data)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to marshal filtered histories: %w", err)
	}

	return filtered, len(compatible), nil
}

// ReplayWorkflow is meant to be used in tests with the output of GetWorkflowHistoriesBundle to check if the given workflow implementation (fn) is compatible with previous executions captured at the time when GetWorkflowHistoriesBundle was run.
func ReplayWorkflow(historiesBytes []byte, fn any, opts worker.WorkflowReplayerOptions) error {
	return replayWorkflow(historiesBytes, fn, opts)
}

// ReplayWorkflow is a typed version of the package-level ReplayWorkflow that handles positional
// workflow replay automatically. For positional workflows, fn is wrapped to accept positional
// arguments matching the on-wire format before reconstructing the Param struct.
func (w Workflow[Param, Return]) ReplayWorkflow(historiesBytes []byte, fn func(workflow.Context, Param) (Return, error), opts worker.WorkflowReplayerOptions) error {
	if !w.positional {
		return replayWorkflow(historiesBytes, fn, opts)
	}

	paramType := reflect.TypeOf((*Param)(nil)).Elem()
	var fieldTypes []reflect.Type
	if paramType.Kind() == reflect.Ptr {
		fieldTypes = extractFieldTypes(paramType.Elem())
	} else {
		fieldTypes = extractFieldTypes(paramType)
	}

	fnVal := reflect.ValueOf(fn)
	wrapper := reflect.MakeFunc(
		reflect.FuncOf(
			append([]reflect.Type{reflect.TypeOf((*workflow.Context)(nil)).Elem()}, fieldTypes...),
			[]reflect.Type{reflect.TypeOf((*Return)(nil)).Elem(), reflect.TypeOf((*error)(nil)).Elem()},
			false,
		),
		func(args []reflect.Value) []reflect.Value {
			var paramVal reflect.Value
			if paramType.Kind() == reflect.Ptr {
				paramVal = reflect.New(paramType.Elem())
				for i := 0; i < paramType.Elem().NumField(); i++ {
					paramVal.Elem().Field(i).Set(args[i+1])
				}
			} else {
				paramVal = reflect.New(paramType).Elem()
				for i := 0; i < paramType.NumField(); i++ {
					paramVal.Field(i).Set(args[i+1])
				}
			}
			return fnVal.Call([]reflect.Value{args[0], paramVal})
		},
	)

	return replayWorkflow(historiesBytes, wrapper.Interface(), opts)
}

func replayWorkflow(historiesBytes []byte, fn any, opts worker.WorkflowReplayerOptions) error {
	var historiesData historiesData
	err := json.Unmarshal(historiesBytes, &historiesData)
	if err != nil {
		return fmt.Errorf("failed to unmarshal histories: %w", err)
	}
	if len(historiesData.Histories) == 0 {
		return fmt.Errorf("no histories available")
	}
	replayer, err := worker.NewWorkflowReplayerWithOptions(opts)
	if err != nil {
		return fmt.Errorf("failed to create replayer: %w", err)
	}
	replayer.RegisterWorkflowWithOptions(fn, workflow.RegisterOptions{
		Name: historiesData.WorkflowName,
	})
	for _, histData := range historiesData.Histories {
		var h history.History
		err := proto.Unmarshal(histData.HistoryBytes, &h)
		if err != nil {
			return fmt.Errorf("failed to unmarshal history: %w", err)
		}
		replayOpts := worker.ReplayWorkflowHistoryOptions{}
		replayOpts.OriginalExecution.ID = histData.WorkflowID
		replayOpts.OriginalExecution.RunID = histData.RunID

		err = replayer.ReplayWorkflowHistoryWithOptions(nil, &h, replayOpts)
		if err != nil {
			return fmt.Errorf("failed to replay workflow %s: %w", historiesData.WorkflowName, err)
		}
	}
	return nil
}
