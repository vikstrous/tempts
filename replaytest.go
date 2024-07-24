package tempts

import (
	"context"
	"encoding/json"
	"fmt"

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
	if client.namespace != w.getQueue().namespace.name {
		return nil, fmt.Errorf("namespace for client %s doesn't match namespace for workflow %s", client.namespace, w.getQueue().namespace.name)
	}
	closedExecutions, err := client.Client.WorkflowService().ListClosedWorkflowExecutions(ctx, &workflowservice.ListClosedWorkflowExecutionsRequest{
		Namespace:       w.getQueue().namespace.name,
		MaximumPageSize: 10,
		Filters: &workflowservice.ListClosedWorkflowExecutionsRequest_TypeFilter{
			TypeFilter: &filter.WorkflowTypeFilter{Name: w.Name()},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get closed executions: %w", err)
	}
	openExecutions, err := client.Client.WorkflowService().ListOpenWorkflowExecutions(ctx, &workflowservice.ListOpenWorkflowExecutionsRequest{
		Namespace:       w.getQueue().namespace.name,
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

// ReplayWorkflow is meant to be used in tests with the output of GetWorkflowHistoriesBundle to check if the given workflow implementation (fn) is compatible with previous executions captured at the time when GetWorkflowHistoriesBundle was run.
func ReplayWorkflow(historiesBytes []byte, fn any, opts worker.WorkflowReplayerOptions) error {
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
		opts := worker.ReplayWorkflowHistoryOptions{}
		opts.OriginalExecution.ID = histData.WorkflowID
		opts.OriginalExecution.RunID = histData.RunID

		err = replayer.ReplayWorkflowHistoryWithOptions(nil, &h, opts)
		if err != nil {
			return fmt.Errorf("failed to replay workflow %s: %w", historiesData.WorkflowName, err)
		}
	}
	return nil
}
