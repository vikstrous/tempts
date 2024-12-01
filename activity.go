package tempts

import (
	"context"
	"fmt"
	"reflect"

	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"
)

// ActivityWithImpl is a temporary struct that implements Registerable. It's meant to be passed into `tempts.NewWorker`.
type ActivityWithImpl struct {
	activityName string
	queue        *Queue
	fn           any
}

func (a ActivityWithImpl) register(ar worker.Registry) {
	ar.RegisterActivityWithOptions(a.fn, activity.RegisterOptions{Name: a.activityName})
}

func (a ActivityWithImpl) validate(q *Queue, v *validationState) error {
	if a.queue.name != q.name {
		return fmt.Errorf("activity for queue %s can't be registered on worker with queue %s", a.queue.name, q.name)
	}
	_, ok := v.activitiesValidated[a.activityName]
	if ok {
		return fmt.Errorf("duplicate activtity name %s for queue %s", a.activityName, q.name)
	}
	v.activitiesValidated[a.activityName] = struct{}{}
	return nil
}

// Activity is used for interacting with activities in a safe way that takes into account the input and output types, queue name and other properties.
type Activity[Param, Return any] struct {
	Name  string
	queue *Queue
	// If true, the Param struct fields will be passed as positional arguments
	positional bool
}

// NewActivity declares the existence of an activity on a given queue with a given name.
func NewActivity[Param, Return any](q *Queue, name string) Activity[Param, Return] {
	panicIfNotStruct[Param]("NewActivity")
	panicIfNotStruct[Return]("NewActivity")
	q.registerActivity(name, (func(context.Context, Param) (Return, error))(nil))
	return Activity[Param, Return]{Name: name, queue: q}
}

// NewActivityPositional declares the existence of an activity on a given queue with a given name.
// Instead of passing the Param struct directly to the activity, it passes each field of the struct
// as a separate positional argument in the order they are defined.
func NewActivityPositional[Param, Return any](q *Queue, name string) Activity[Param, Return] {
	panicIfNotStruct[Param]("NewActivityPositional")
	panicIfNotStruct[Return]("NewActivityPositional")

	// Get the type information for the Param struct
	paramType := reflect.TypeOf((*Param)(nil)).Elem()
	if paramType.Kind() == reflect.Ptr {
		paramType = paramType.Elem()
	}

	// Create a slice of function parameter types: (context.Context, field1Type, field2Type, ...)
	paramTypes := make([]reflect.Type, paramType.NumField()+1)
	paramTypes[0] = reflect.TypeOf((*context.Context)(nil)).Elem()
	for i := 0; i < paramType.NumField(); i++ {
		paramTypes[i+1] = paramType.Field(i).Type
	}

	// Create the function type: func(context.Context, field1Type, field2Type, ...) (Return, error)
	returnType := reflect.TypeOf((*Return)(nil)).Elem()
	errorType := reflect.TypeOf((*error)(nil)).Elem()
	fnType := reflect.FuncOf(paramTypes, []reflect.Type{returnType, errorType}, false)

	// Register a nil function of the correct type
	q.registerActivity(name, reflect.Zero(fnType).Interface())

	return Activity[Param, Return]{Name: name, queue: q, positional: true}
}

func panicIfNotStruct[Param any](funcName string) {
	paramType := reflect.TypeOf((*Param)(nil)).Elem()
	if paramType.Kind() == reflect.Ptr {
		paramType = paramType.Elem()
	}
	if paramType.Kind() != reflect.Struct {
		panic(fmt.Sprintf("%s requires a struct or pointer to struct type parameter, got %v", funcName, paramType.Kind()))
	}
}

func extractFieldTypes(structType reflect.Type) []reflect.Type {
	if structType.Kind() == reflect.Ptr {
		structType = structType.Elem()
	}
	if structType.Kind() != reflect.Struct {
		panic("extractFieldTypes called with non-struct type")
	}

	fieldTypes := make([]reflect.Type, structType.NumField())
	for i := 0; i < structType.NumField(); i++ {
		fieldTypes[i] = structType.Field(i).Type
	}
	return fieldTypes
}

// WithImplementation should be called to create the parameters for NewWorker(). It declares which function implements the activity.
func (a Activity[Param, Return]) WithImplementation(fn func(context.Context, Param) (Return, error)) *ActivityWithImpl {
	if !a.positional {
		return &ActivityWithImpl{activityName: a.Name, queue: a.queue, fn: fn}
	}

	// For positional activities, create a wrapper function that converts positional arguments to a struct
	paramType := reflect.TypeOf((*Param)(nil)).Elem()
	var fieldTypes []reflect.Type
	if paramType.Kind() == reflect.Ptr {
		fieldTypes = extractFieldTypes(paramType.Elem())
	} else {
		fieldTypes = extractFieldTypes(paramType)
	}

	wrapper := reflect.MakeFunc(
		reflect.FuncOf(
			append([]reflect.Type{reflect.TypeOf((*context.Context)(nil)).Elem()}, fieldTypes...),
			[]reflect.Type{reflect.TypeOf((*Return)(nil)).Elem(), reflect.TypeOf((*error)(nil)).Elem()},
			false,
		),
		func(args []reflect.Value) []reflect.Value {
			ctx := args[0]

			// Create a new instance of the Param struct
			var paramVal reflect.Value
			if paramType.Kind() == reflect.Ptr {
				paramVal = reflect.New(paramType.Elem())
				// Fill the struct fields with the positional arguments
				for i := 0; i < paramType.Elem().NumField(); i++ {
					paramVal.Elem().Field(i).Set(args[i+1])
				}
			} else {
				paramVal = reflect.New(paramType).Elem()
				// Fill the struct fields with the positional arguments
				for i := 0; i < paramType.NumField(); i++ {
					paramVal.Field(i).Set(args[i+1])
				}
			}

			// Call the implementation function with the context and constructed struct
			results := reflect.ValueOf(fn).Call([]reflect.Value{ctx, paramVal})
			return results
		},
	)

	return &ActivityWithImpl{activityName: a.Name, queue: a.queue, fn: wrapper.Interface()}
}

// Execute asynchronously executes the activity and returns a promise.
func (a Activity[Param, Return]) Execute(ctx workflow.Context, param Param) workflow.Future {
	if !a.positional {
		return workflow.ExecuteActivity(workflow.WithTaskQueue(ctx, a.queue.name), a.Name, param)
	}

	// For positional activities, extract struct fields into separate arguments
	paramVal := reflect.ValueOf(param)
	if paramVal.Kind() == reflect.Ptr {
		paramVal = paramVal.Elem()
	}

	args := make([]interface{}, paramVal.NumField())
	for i := 0; i < paramVal.NumField(); i++ {
		args[i] = paramVal.Field(i).Interface()
	}

	return workflow.ExecuteActivity(workflow.WithTaskQueue(ctx, a.queue.name), a.Name, args...)
}

// Run synchronously executes the activity and returns the result.
func (a Activity[Param, Return]) Run(ctx workflow.Context, param Param) (Return, error) {
	var result Return
	err := a.Execute(ctx, param).Get(ctx, &result)
	return result, err
}
