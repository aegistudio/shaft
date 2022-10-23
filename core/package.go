// Package core is the core and generic part of the dependency
// injection framework. And we can build dependency injection
// framework upon it. You can also implement the one of your
// favourable style if you want to.
package core

import (
	"fmt"
	"reflect"
)

// Spec defines specification of a provided or consumed type
// to the dependency injection container.
type Spec struct {
	Type reflect.Type

	// Name can be used to distinguish types and groups when
	// they are of the same type.
	Name string

	// Group specifies whether this port provides and consumes
	// a group of objects. The value corresponding to a group
	// will always be a slice.
	Group bool

	// Decorate specifies whether this port decorates a type
	// or a group of object.
	//
	// There must be a node that is not decorate node to
	// provide the type, and more than one node that is not
	// decorate node to consume the type, and the decorate
	// nodes will be called before passing it to any one of
	// the decorate nodes.
	//
	// Error will be generated if a decorate node will also
	// provides some required type, but no one provides the
	// type to decorate.
	Decorate bool
}

// ErrDependency indicates there's dependency error on node.
//
// Dependency error might be one stacking over another, it
// indicates a path from current node to the error node.
type ErrDependency struct {
	Node string
	Err  error
}

func (e *ErrDependency) Error() string {
	return fmt.Sprintf("node %q dependency error: %v", e.Node, e.Err)
}

func (e *ErrDependency) Unwrap() error {
	return e.Err
}

// ErrExecute indicates error generated while executing node.
type ErrExecute struct {
	Node string
	Err  error
}

func (e *ErrExecute) Error() string {
	return fmt.Sprintf("node %q execute error: %v", e.Node, e.Err)
}

func (e *ErrExecute) Unwrap() error {
	return e.Err
}
