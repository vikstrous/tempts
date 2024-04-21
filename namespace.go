package tempts

import "go.temporal.io/sdk/client"

// Namespace declares the name of a namespace for use later. This is a pre-requisite for defining any other resource such as workflows or activities.
type Namespace struct {
	name string
}

// DefaultNamespace a convenient way to refer to temporal's default namespace.
var DefaultNamespace = NewNamespace(client.DefaultNamespace)

// NewNamespace is used for declaring namespaces.
// Unless using namespaces in your application, you can use `tempts.DefaultNamespace` instead of defining one.
func NewNamespace(name string) *Namespace {
	return &Namespace{name: name}
}
