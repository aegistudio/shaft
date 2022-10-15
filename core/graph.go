package core

import (
	"fmt"
	"reflect"
)

// graphNodeKey represents the input and output of a node.
//
// It is used for indexing the graph so that we will be able
// to determine the order of node to call later.
type graphNodeKey struct {
	typ   reflect.Type
	name  string
	group bool
}

func (k graphNodeKey) String() string {
	result := k.name
	if result == "" {
		result = k.typ.String()
	}
	if k.group {
		result = fmt.Sprintf("[%s]", result)
	}
	return result
}

func extractGraphKey(spec Spec) graphNodeKey {
	return graphNodeKey{
		typ:   spec.Type,
		name:  spec.Name,
		group: spec.Group,
	}
}

// graphNodeOutputSlot represents an output slot of the key.
type graphNodeOutputSlot struct {
	id    int
	index int
}

// graphNode is an virtual representation of the dependency.
//
// The graph node is only used for keeping track of those
// preceeding the node, and the virtual action to be taken
// while this node is visited.
type graphNode struct {
	input  []Spec
	output []Spec
	value  interface{}

	// format is used to provide display string of current
	// graph node. This is useful when we don't want to
	// create and format a bundle of strings at start.
	//
	// If this value is not present, we will use index of
	// the graph node instead.
	format fmt.Stringer
}

func (g graphNode) String(id int) string {
	if g.format != nil {
		return g.format.String()
	} else {
		return fmt.Sprintf("#%d", id)
	}
}

// graph is the map of dependency initialization.
//
// To execute the graph, it must be toposorted into an
// execution plan, and only then will we know if there
// were cyclic dependencies there.
type graph struct {
	nodes    []graphNode
	provide  map[graphNodeKey][]graphNodeOutputSlot
	decorate map[graphNodeKey][]graphNodeOutputSlot
}

func newGraph() *graph {
	return &graph{
		provide:  make(map[graphNodeKey][]graphNodeOutputSlot),
		decorate: make(map[graphNodeKey][]graphNodeOutputSlot),
	}
}

// insert a graph node into the graph, updating the
// object provision indices.
func (g *graph) insert(node graphNode) {
	id := len(g.nodes)
	g.nodes = append(g.nodes, node)
	for index, item := range node.output {
		key := extractGraphKey(item)
		if item.Decorate {
			g.decorate[key] = append(g.decorate[key], graphNodeOutputSlot{
				id:    id,
				index: index,
			})
		} else {
			g.provide[key] = append(g.provide[key], graphNodeOutputSlot{
				id:    id,
				index: index,
			})
		}
	}
}

// executionParam is the parameters or results for the
// execution of a series of execution node.
type executionParam struct {
	params []reflect.Value
}

// executionCollect collects the result from the strip
// of the parameter.
type executionCollect struct {
	result *executionParam
	index  int
}

func (c *executionCollect) collect() reflect.Value {
	return c.result.params[c.index]
}

// executionNode is a family of nodes as the result of
// the toposort function.
//
// Specifically, for the *graphUserNode, the caller must
// handle the execute manually since the container knows
// nothing about how to handle the value.
type executionNode interface {
	execute()
}

type graphUserNode struct {
	params *executionParam
	result *executionParam
	value  interface{}
}

func (graphUserNode) execute() {
	panic("graphUserNode.execute must not be invoked")
}

type collectParamNode struct {
	items  []executionCollect
	result *executionParam
}

func (c collectParamNode) execute() {
	for i, item := range c.items {
		c.result.params[i] = item.collect()
	}
}

type collectGroupNode struct {
	items  []executionCollect
	result *executionParam
}

func (c collectGroupNode) execute() {
	for _, item := range c.items {
		c.result.params[0] = reflect.AppendSlice(
			c.result.params[0], item.collect())
	}
}

// graphToposort keeps track of the instantiated graph nodes,
// so that we can generate an well-formed execution plan.
type graphToposort struct {
	outputs    map[int]*executionParam
	grouped    map[graphNodeKey]*executionParam
	decorated  map[graphNodeKey]executionCollect
	decorating map[graphNodeKey]executionCollect
	pending    map[int]struct{}
	result     []executionNode
}

func newGraphToposort() *graphToposort {
	return &graphToposort{
		outputs:    make(map[int]*executionParam),
		grouped:    make(map[graphNodeKey]*executionParam),
		decorated:  make(map[graphNodeKey]executionCollect),
		decorating: make(map[graphNodeKey]executionCollect),
		pending:    make(map[int]struct{}),
	}
}

// toposortGenerateGraphNodeID creates the execution param
// with graph internal node.
//
// It is just like toposortGenerateGraphNode, but adds the
// constrain of acyclic graph, and those what has been
// initialized will be reused.
func (g *graph) toposortGenerateGraphNodeID(
	tp *graphToposort, id int,
) (*executionParam, error) {
	params, ok := tp.outputs[id]
	if ok {
		return params, nil
	}
	tp.pending[id] = struct{}{}
	defer delete(tp.pending, id)
	params, err := g.toposortGenerateGraphNode(tp, g.nodes[id])
	if err != nil {
		return nil, err
	}
	tp.outputs[id] = params
	return params, nil
}

// toposortGenerateSingle generates the single collect
// corresponding to a node.
func (g *graph) toposortGenerateSingle(
	tp *graphToposort, item graphNodeKey,
) (executionCollect, error) {
	outputSlots := g.provide[item]
	if len(outputSlots) == 0 {
		return executionCollect{}, fmt.Errorf(
			"type %s missing dependency", item)
	}
	if len(outputSlots) != 1 {
		return executionCollect{}, fmt.Errorf(
			"type %s ambigious dependency", item)
	}
	outputSlot := outputSlots[0]
	id := outputSlot.id
	if _, ok := tp.pending[id]; ok {
		return executionCollect{}, fmt.Errorf(
			"type %s cyclic dependency on node %s",
			g.nodes[id].String(id), item)
	}
	params, err := g.toposortGenerateGraphNodeID(tp, id)
	if err != nil {
		return executionCollect{}, err
	}
	return executionCollect{
		result: params,
		index:  outputSlot.index,
	}, nil
}

// toposortGenerateGrouped generates the group collect
// node and returns the execution param of that group.
func (g *graph) toposortGenerateGrouped(
	tp *graphToposort, group graphNodeKey,
) (executionCollect, error) {
	if result, ok := tp.grouped[group]; ok {
		return executionCollect{
			result: result,
			index:  0,
		}, nil
	}
	result := &executionParam{
		params: []reflect.Value{
			reflect.MakeSlice(group.typ, 0, 0),
		},
	}
	node := &collectGroupNode{
		result: result,
	}
	outputSlots := g.provide[group]
	for _, outputSlot := range outputSlots {
		params, err := g.toposortGenerateGraphNodeID(tp, outputSlot.id)
		if err != nil {
			return executionCollect{}, err
		}
		node.items = append(node.items, executionCollect{
			result: params,
			index:  outputSlot.index,
		})
	}
	tp.result = append(tp.result, node)
	tp.grouped[group] = result
	return executionCollect{
		result: result,
		index:  0,
	}, nil
}

// toposortGenerateBaseCollect creates the basic collect
// for executing a graph node's parameter.
//
// This step fills in the tp.decorating which can be used in
// filling the undecorated one later.
func (g *graph) toposortGenerateBaseCollect(
	tp *graphToposort, key graphNodeKey,
) (executionCollect, error) {
	// Initialize the base collect object first.
	var baseCollect executionCollect
	var err error
	if key.group {
		baseCollect, err = g.toposortGenerateGrouped(tp, key)
	} else {
		baseCollect, err = g.toposortGenerateSingle(tp, key)
	}
	if err != nil {
		return executionCollect{}, err
	}

	// So the key is not decorated, and it is really likely
	// there's no need to decorate it, check for the case.
	outputSlots := g.decorate[key]
	if len(outputSlots) == 0 {
		return baseCollect, nil
	}

	// So we will need to decorate the object now. In this
	// case we initialize the tp.decorating if it has not
	// been initialized, otherwise we will just ignore it.
	if _, ok := tp.decorated[key]; ok {
		// XXX: don't initialize the decorating key again if
		// it has been initialized here.
		return baseCollect, nil
	}
	if _, ok := tp.decorating[key]; !ok {
		tp.decorating[key] = baseCollect
	}
	return baseCollect, nil
}

// toposortGenerateCollect creates the general collect for
// executing a graph node's parameter.
func (g *graph) toposortGenerateCollect(
	tp *graphToposort, spec Spec,
) (executionCollect, error) {
	key := extractGraphKey(spec)
	baseCollect, err := g.toposortGenerateBaseCollect(tp, key)
	if err != nil {
		return executionCollect{}, nil
	}

	// Check whether we are in the middle way of initializing
	// a decorated key here. This is always done by pushing
	// another decorated here.
	if spec.Decorate {
		collect, ok := tp.decorating[key]
		if !ok {
			// XXX: We detect bugs here, this cannot happen
			// since the decorating must be filled by those
			// who are requesting decorations.
			panic("invalid uninitialized decorate")
		}
		return collect, nil
	}

	// Check whether there's need to decorate, and just
	// return the base collect if there's no need to do so.
	outputSlots := g.decorate[key]
	if len(outputSlots) == 0 {
		return baseCollect, nil
	}

	// Now we will need to initialize the decorators. We
	// iterate through the output slots and initialize them
	// one by one, and finally return the decorated one.
	if decorated, ok := tp.decorated[key]; ok {
		// Return the decorated one if there's any.
		return decorated, nil
	}
	for _, outputSlot := range outputSlots {
		params, err := g.toposortGenerateGraphNodeID(
			tp, outputSlot.id)
		if err != nil {
			return executionCollect{}, err
		}
		tp.decorating[key] = executionCollect{
			result: params,
			index:  outputSlot.index,
		}
	}
	result := tp.decorating[key]
	tp.decorated[key] = result
	delete(tp.decorating, key)
	return result, nil
}

// toposortGenerateGraphNode generates the execution result
// of provided graph node.
func (g *graph) toposortGenerateGraphNode(
	tp *graphToposort, current graphNode,
) (*executionParam, error) {
	collectNode := &collectParamNode{
		result: &executionParam{
			params: make([]reflect.Value, len(current.input)),
		},
	}
	for _, input := range current.input {
		key := extractGraphKey(input)
		_, err := g.toposortGenerateBaseCollect(tp, key)
		if err != nil {
			return nil, err
		}
	}
	for _, input := range current.input {
		collect, err := g.toposortGenerateCollect(tp, input)
		if err != nil {
			return nil, err
		}
		collectNode.items = append(collectNode.items, collect)
	}
	tp.result = append(tp.result, collectNode)
	userNode := &graphUserNode{
		params: collectNode.result,
		result: &executionParam{
			params: make([]reflect.Value, len(current.output)),
		},
		value: current.value,
	}
	tp.result = append(tp.result, userNode)
	return userNode.result, nil
}

// toposort evaluates the execution plan for a series of
// invoked type. The strip of the last node will be the
// one to collect the values corresponding to the key.
func (g *graph) toposort(
	invokes []graphNode,
) ([]executionNode, error) {
	tp := newGraphToposort()
	for _, invoke := range invokes {
		_, err := g.toposortGenerateGraphNode(tp, invoke)
		if err != nil {
			return nil, err
		}
	}
	return tp.result, nil
}
