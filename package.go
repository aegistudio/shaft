// Package shaft is an alternative dependency injection framework
// of golang. It is mainly inspired by go.uber.org/fx and
// go.uber.org/dig, but comes with support for deferring (to close
// created resources), and a more simplified semantics.
//
// Just like the fx and dig conterparts, the shaft interpret
// dependency injection relationship by matching types. However,
// we apply these rules to simplify when we scan and transform
// provided functions:
//
//   1. If the parameter is slice kind `[]T`, we interpret it as
//      providing or consuming a group of `T`, which I find it
//      more intuitive. To provide and consume a slice instead,
//      it is recommended to define something like `type TSlice []T`.
//   2. If a parameter is consumed and provided, we interpret
//      it as a decorated paramter. Which means if the provided
//      function isn't providing any other type, it will be
//      called only after someone providing this type.
//   3. Because you can assign a name to type easily by defining
//      `type Name T`, and we would like to keep it as simple as
//      possible, we don't provide naming support here.
package shaft

import (
	"github.com/aegistudio/shaft/core"
)

// Option is just a simple forwarding of core.Option.
type Option = core.Option

// Run is just a simple forwarding of core.Run.
func Run(opts ...Option) error {
	return core.Run(opts...)
}

// Module is just a simple forwarding of core.Module.
func Module(opts ...Option) Option {
	return core.Module(opts...)
}
