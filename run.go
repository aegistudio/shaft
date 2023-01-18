package shaft

import (
	"fmt"
	"reflect"
	"runtime"
	"strings"

	"github.com/aegistudio/shaft/core"
)

func convertSingle(item reflect.Type) core.Spec {
	group := false
	if item.Kind() == reflect.Slice {
		group = true
	}
	// Decorate is filled while converting the whole function,
	// and we will simply leave the code there now.
	return core.Spec{
		Type:  item,
		Group: group,
	}
}

func convertFunc(args, rets []reflect.Type) (in, out []core.Spec) {
	inMap := make(map[core.Spec][]int)
	for i, arg := range args {
		spec := convertSingle(arg)
		in = append(in, spec)
		inMap[spec] = append(inMap[spec], i)
	}
	for i, ret := range rets {
		spec := convertSingle(ret)
		out = append(out, spec)
		if matches := inMap[spec]; len(matches) > 0 {
			out[i].Decorate = true
			for _, j := range matches {
				in[j].Decorate = true
			}
		}
	}
	return
}

// op is just stored to be converted into string.
type op int

const (
	opInvoke = op(iota)
	opProvide
	opStack
	opSupply
	opPopulate
)

func (o op) String() string {
	switch o {
	case opInvoke:
		return "Invoke"
	case opProvide:
		return "Provide"
	case opStack:
		return "Stack"
	case opSupply:
		return "Supply"
	case opPopulate:
		return "Populate"
	default:
		return "Unknown"
	}
}

// funcOp stores the function's pc alongside with op.
type funcOp struct {
	op op
	pc uintptr
}

func (o funcOp) String() string {
	name := "(unknown)"
	if fn := runtime.FuncForPC(o.pc); fn != nil {
		name = fn.Name()
	}
	return fmt.Sprintf("%s(%s)", o.op, name)
}

// valuesOp stores the values' type alongside with op.
type valuesOp struct {
	op    op
	types []reflect.Type
}

func (o valuesOp) String() string {
	var names []string
	for _, typ := range o.types {
		names = append(names, typ.String())
	}
	return fmt.Sprintf("%s(%s)", o.op, strings.Join(names, ","))
}

var typeError = reflect.TypeOf((*error)(nil)).Elem()

// Provide a function as constructor.
//
// The provided f must be a function, objects required by
// the function is present in the argument list, and the
// objects created by the function is in the result. And the
// function can return an error as last result optionally.
func Provide(f interface{}) Option {
	val := reflect.ValueOf(f)
	if val.Kind() != reflect.Func {
		panic(fmt.Sprintf("invalid non-func %T provided", f))
	}
	typ := val.Type()
	var args []reflect.Type
	numArgs := typ.NumIn()
	for i := 0; i < numArgs; i++ {
		args = append(args, typ.In(i))
	}
	var rets []reflect.Type
	numRets := typ.NumOut()
	for i := 0; i < numRets; i++ {
		rets = append(rets, typ.Out(i))
	}
	returnsError := false
	if len(rets) > 0 && rets[len(rets)-1] == typeError {
		rets = rets[:len(rets)-1]
		returnsError = true
	}
	if len(rets) == 0 {
		panic(fmt.Sprintf("func %v must provide result", f))
	}
	in, out := convertFunc(args, rets)
	return core.Provide(func(in []reflect.Value) ([]reflect.Value, error) {
		var err error
		out := val.Call(in)
		if returnsError {
			err, _ = out[len(out)-1].Interface().(error)
			out = out[:len(out)-1]
		}
		return out, err
	}, in, out, funcOp{op: opProvide, pc: val.Pointer()})
}

// Supply an objects to dependency injection.
//
// The infcs specifies what type would you like the object
// to be, it might be either be pointer type or slice type,
// corresponding to the case you want to specify a single
// object or a group object. Specifying infcs is mandatory
// when you want to provide the object as instance
// implementing an interface,  or you will lose the type
// information when you pass the object as parameter.
//
// You might also specify the interface types of this object
// when supplying, otherwise the actual underlying object
// will have been supplied to them.
func Supply(obj interface{}, infcs ...interface{}) Option {
	value := reflect.ValueOf(obj)
	var values []reflect.Value
	var types []reflect.Type
	var spec []core.Spec
	if len(infcs) == 0 {
		values = append(values, value)
		typ := value.Type()
		types = append(types, typ)
		spec = append(spec, convertSingle(typ))
	}
	for _, infc := range infcs {
		typ := reflect.TypeOf(infc)
		val := value
		switch typ.Kind() {
		case reflect.Ptr:
			typ = typ.Elem()
			val = value.Convert(typ)
		case reflect.Slice:
			val = reflect.MakeSlice(typ, 0, 1)
			val = reflect.Append(val, value.Convert(typ.Elem()))
		default:
			panic(fmt.Sprintf(
				"type %T must be pointer or slice", infc))
		}
		values = append(values, val)
		types = append(types, typ)
		spec = append(spec, convertSingle(typ))
	}
	return core.Supply(values, spec,
		valuesOp{op: opSupply, types: types})
}

// Invoke a function as consumer.
//
// The provided f must be a function, objects required by
// the function is present in the argument list. The result
// of this function is ignored, except for the last result
// being an error, and the error is returned then.
func Invoke(f interface{}) Option {
	val := reflect.ValueOf(f)
	if val.Kind() != reflect.Func {
		panic(fmt.Sprintf("invalid non-func %T provided", f))
	}
	typ := val.Type()
	var args []reflect.Type
	numArgs := typ.NumIn()
	for i := 0; i < numArgs; i++ {
		args = append(args, typ.In(i))
	}
	numRets := typ.NumOut()
	returnsError := false
	if numRets > 0 && typ.Out(numRets-1) == typeError {
		returnsError = true
	}
	in, _ := convertFunc(args, nil)
	return core.Invoke(func(in []reflect.Value) error {
		var err error
		out := val.Call(in)
		if returnsError {
			err, _ = out[len(out)-1].Interface().(error)
		}
		return err
	}, in, funcOp{op: opInvoke, pc: val.Pointer()})
}

// Populate objects from the dependency injection.
func Populate(objs ...interface{}) Option {
	var values []reflect.Value
	var types []reflect.Type
	var spec []core.Spec
	for _, obj := range objs {
		value := reflect.ValueOf(obj)
		values = append(values, value)
		typ := value.Type()
		types = append(types, typ)
		if typ.Kind() != reflect.Ptr {
			panic(fmt.Sprintf("invalid non-ptr %T requested", obj))
		}
		spec = append(spec, convertSingle(typ.Elem()))
	}
	return core.Populate(values, spec,
		valuesOp{op: opPopulate, types: types})
}

// Stack a function as constructor.
//
// The provided f must be a function, its first argument must
// be a function pointer accepting results generated by this
// function, which is a callback provided by the framework.
// And the remainder of the arguments must be the objects
// required by this function. The inner function pointer must
// return an error, and so do the function, as there could
// always be some module returning error.
func Stack(f interface{}) Option {
	val := reflect.ValueOf(f)
	if val.Kind() != reflect.Func {
		panic(fmt.Sprintf("invalid non-func %T provided", f))
	}
	typ := val.Type()
	var args []reflect.Type
	numArgs := typ.NumIn()
	var callbackTyp reflect.Type
	if numArgs > 0 {
		callbackTyp = typ.In(0)
	}
	if callbackTyp == nil || callbackTyp.Kind() != reflect.Func {
		panic(fmt.Sprintf(
			"func %v must accept callback as first argument", f))
	}
	for i := 1; i < numArgs; i++ {
		args = append(args, typ.In(i))
	}
	if typ.NumOut() != 1 || typ.Out(0) != typeError {
		panic(fmt.Sprintf("func %v must return an error", f))
	}
	if callbackTyp.NumOut() != 1 || callbackTyp.Out(0) != typeError {
		panic(fmt.Sprintf(
			"func %v callback must return just an error", f))
	}
	var rets []reflect.Type
	numRets := callbackTyp.NumIn()
	for i := 0; i < numRets; i++ {
		rets = append(rets, callbackTyp.In(i))
	}
	in, out := convertFunc(args, rets)
	return core.Stack(func(
		g func(out []reflect.Value) error, in []reflect.Value,
	) error {
		var args []reflect.Value
		args = append(args, reflect.MakeFunc(
			callbackTyp, func(out []reflect.Value) []reflect.Value {
				var result []reflect.Value
				val := reflect.ValueOf(g(out))
				if !val.IsValid() {
					val = reflect.Zero(typeError)
				}
				result = append(result, val)
				return result
			},
		))
		args = append(args, in...)
		out := val.Call(args)
		err, _ := out[0].Interface().(error)
		return err
	}, in, out, funcOp{op: opStack, pc: val.Pointer()})
}
