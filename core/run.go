package core

import (
	"fmt"
	"reflect"
)

type option struct {
	g         *graph
	consumers []graphNode
}

// Option is the option for performing dependency injection.
type Option func(*option)

// Module aggregates a set of options as a single option.
func Module(opts ...Option) Option {
	return func(option *option) {
		for _, opt := range opts {
			opt(option)
		}
	}
}

type runAction func(state *runState, input, output []reflect.Value) error

type runState struct {
	pending []executionNode
}

func (rs *runState) run() error {
	for len(rs.pending) > 0 {
		var node executionNode
		node, rs.pending = rs.pending[0], rs.pending[1:]
		if userNode, ok := node.(*graphUserNode); ok {
			if err := userNode.value.(runAction)(
				rs, userNode.params.params, userNode.result.params,
			); err != nil {
				return err
			}
		} else {
			node.execute()
		}
	}
	return nil
}

// Run performs the dependency injection with specified options.
func Run(opts ...Option) error {
	g := newGraph()
	option := &option{
		g: g,
	}
	Module(opts...)(option)

	// Generate the execution plan for invoke first.
	nodes, err := g.toposort(option.consumers)
	if err != nil {
		return err
	}

	// Execute the created execution plan.
	return (&runState{pending: nodes}).run()
}

// Provide a normal constructor function for futher execution.
func Provide(
	f func([]reflect.Value) ([]reflect.Value, error),
	input, output []Spec, format fmt.Stringer,
) Option {
	return func(option *option) {
		option.g.insert(graphNode{
			input:  input,
			output: output,
			value: runAction(func(
				_ *runState, in, out []reflect.Value,
			) error {
				output, err := f(in)
				if err != nil {
					return err
				}
				copy(out, output)
				return nil
			}),
			format: format,
		})
	}
}

// Supply a series of objects to the graph.
func Supply(
	values []reflect.Value, output []Spec, format fmt.Stringer,
) Option {
	return func(option *option) {
		option.g.insert(graphNode{
			output: output,
			value: runAction(func(
				_ *runState, _, out []reflect.Value,
			) error {
				copy(out, values)
				return nil
			}),
			format: format,
		})
	}
}

// Stack a function providing objects by calling a callback.
func Stack(
	f func(func([]reflect.Value) error, []reflect.Value) error,
	input, output []Spec, format fmt.Stringer,
) Option {
	return func(option *option) {
		option.g.insert(graphNode{
			input:  input,
			output: output,
			value: runAction(func(
				rs *runState, in, out []reflect.Value,
			) error {
				return f(func(output []reflect.Value) error {
					copy(out, output)
					return rs.run()
				}, in)
			}),
			format: format,
		})
	}
}

// Invoke a function consuming object that has been provided.
func Invoke(
	f func([]reflect.Value) error,
	input []Spec, format fmt.Stringer,
) Option {
	return func(option *option) {
		option.consumers = append(option.consumers, graphNode{
			input: input,
			value: runAction(func(
				_ *runState, in, _ []reflect.Value,
			) error {
				return f(in)
			}),
			format: format,
		})
	}
}

// Populate to copy out objects into the output pointers.
func Populate(
	ptrs []reflect.Value, input []Spec, format fmt.Stringer,
) Option {
	return func(option *option) {
		option.consumers = append(option.consumers, graphNode{
			input: input,
			value: runAction(func(
				_ *runState, in, _ []reflect.Value,
			) error {
				for i := range in {
					ptrs[i].Elem().Set(in[i])
				}
				return nil
			}),
			format: format,
		})
	}
}
