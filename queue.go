package tstemporal

type Queue struct {
	name       string
	namespace  *Namespace
	activities map[string]any
	workflows  map[string]any
}

func NewQueue(namespace *Namespace, name string) *Queue {
	return &Queue{name: name, namespace: namespace, activities: map[string]any{}, workflows: map[string]any{}}
}

func (q *Queue) registerActivity(activityName string, fn any) {
	q.activities[activityName] = fn
}

func (q *Queue) registerWorkflow(workflowName string, fn any) {
	q.workflows[workflowName] = fn
}
