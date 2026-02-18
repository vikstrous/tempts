package tempts

import (
	"strings"
	"sync"

	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"
)

// Queue is the declaration of a temporal queue, which is used for routing workflows and activities to workers.
type Queue struct {
	mu         sync.Mutex
	name       string
	activities map[string]any
	workflows  map[string]any
}

type registry interface {
	worker.ActivityRegistry
	worker.WorkflowRegistry
	SetActivityTaskQueue(string, ...any)
}

// validateName panics on empty or whitespace-only names.
//
// Why panic instead of returning error: These constructors are designed for
// package-level var declarations (var q = tempts.NewQueue("main")) where
// error returns can't be handled. An empty name is always a programming
// mistake — it can never be correct at runtime — so it's the same class
// of invariant violation as panicIfNotStruct. The panic fires during
// process init, before any Temporal work is processed, giving immediate
// feedback during development. See regexp.MustCompile for the stdlib
// precedent of panicking on invalid compile-time-known inputs.
func validateName(name string) {
	if strings.TrimSpace(name) == "" {
		panic("tempts: name must not be empty or whitespace-only")
	}
}

// NewQueue declares the existence of a queue.
func NewQueue(name string) *Queue {
	validateName(name)
	return &Queue{name: name, activities: map[string]any{}, workflows: map[string]any{}}
}

func (q *Queue) registerActivity(activityName string, fn any) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.activities[activityName] = fn
}

func (q *Queue) registerWorkflow(workflowName string, fn any) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.workflows[workflowName] = fn
}

// RegisterMockFallbacks registers fake activities and workflows for queue in the context of a unit test. This is necessary so that the test environment knows their types and they can be mocked.
// Any unmocked activities or workflows trigger a panic and fail the test with a descriptive error message.
func (q *Queue) RegisterMockFallbacks(r registry) {
	q.mu.Lock()
	defer q.mu.Unlock()
	for k, fn := range q.activities {
		r.RegisterActivityWithOptions(fn, activity.RegisterOptions{Name: k})
		r.SetActivityTaskQueue(q.name, k)
	}
	for k, fn := range q.workflows {
		r.RegisterWorkflowWithOptions(fn, workflow.RegisterOptions{Name: k})
	}
}
