package tstemporal

type Namespace struct {
	name string
}

func NewNamespace(name string) *Namespace {
	return &Namespace{name: name}
}
