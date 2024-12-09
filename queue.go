package tempts

import (
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"
)

// Queue is the declaration of a temporal, queue, which is used for routing workflows and activities to workers.
type Queue struct {
	name       string
	activities map[string]any
	workflows  map[string]any
}

type registry interface {
	worker.ActivityRegistry
	worker.WorkflowRegistry
	SetActivityTaskQueue(string, ...any)
}

// NewQueue declares the existence of a queue
func NewQueue(name string) *Queue {
	return &Queue{name: name, activities: map[string]any{}, workflows: map[string]any{}}
}

func (q *Queue) registerActivity(activityName string, fn any) {
	q.activities[activityName] = fn
}

func (q *Queue) registerWorkflow(workflowName string, fn any) {
	q.workflows[workflowName] = fn
}

// RegisterMockFallbacks registers fake activities and workflows for queue in the context of a unit test. This is necessary so that the test environment knows their types and they can be mocked.
// Any unmocked activities or workflows trigger a panic and fail the test with a descriptive error message.
func (q *Queue) RegisterMockFallbacks(r registry) {
	for k, fn := range q.activities {
		r.RegisterActivityWithOptions(fn, activity.RegisterOptions{Name: k})
		r.SetActivityTaskQueue(q.name, k)
	}
	for k, fn := range q.workflows {
		r.RegisterWorkflowWithOptions(fn, workflow.RegisterOptions{Name: k})
	}
}
