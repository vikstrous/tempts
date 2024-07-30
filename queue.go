package tempts

// Queue is the declaration of a temporal, queue, which is used for routing workflows and activities to workers.
type Queue struct {
	name       string
	activities map[string]any
	workflows  map[string]any
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
