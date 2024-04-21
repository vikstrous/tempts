package tempts

// Queue is the declaration of a temporal, queue, which is used for routing workflows and activities to workers.
type Queue struct {
	name       string
	namespace  *Namespace
	activities map[string]any
	workflows  map[string]any
}

// NewQueue declares the existence of a queue in a given namespace.
// A nil namespace is equivalent to using `tempts.DefaultNamespace`
func NewQueue(namespace *Namespace, name string) *Queue {
	if namespace == nil {
		namespace = DefaultNamespace
	}
	return &Queue{name: name, namespace: namespace, activities: map[string]any{}, workflows: map[string]any{}}
}

func (q *Queue) registerActivity(activityName string, fn any) {
	q.activities[activityName] = fn
}

func (q *Queue) registerWorkflow(workflowName string, fn any) {
	q.workflows[workflowName] = fn
}
