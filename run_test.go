package shaft_test

import (
	"fmt"
	"testing"

	"github.com/aegistudio/shaft"
	"github.com/stretchr/testify/assert"
)

type I interface {
	invoke(*[]string)
}

type A struct {
}

func (A) invoke(events *[]string) {
	*events = append(*events, "invoke a")
}

func provideObjectA(events *[]string, _ *D) []I {
	*events = append(*events, "provide a")
	return []I{&A{}}
}

type B struct {
	counter int
}

func (b B) invoke(events *[]string) {
	*events = append(*events,
		fmt.Sprintf("invoke b %d", b.counter))
}

func stackObjectB(f func(*B, []I) error, events *[]string) error {
	b := &B{}
	*events = append(*events, "stack b")
	defer func() { *events = append(*events, "defer b") }()
	return f(b, []I{b})
}

type C struct {
}

func redundantObjectC(events *[]string) (*C, error) {
	*events = append(*events, "provide c")
	return &C{}, nil
}

type D struct {
}

func (D) invoke(events *[]string) {
	*events = append(*events, "invoke d")
}

func decorateObjectD(events *[]string, b *B, val int) (*B, *D, []I) {
	*events = append(*events, "provide d")
	b.counter = val
	d := &D{}
	return b, d, []I{d}
}

func TestStandard(t *testing.T) {
	assert := assert.New(t)

	var events []string
	assert.NoError(shaft.Run(
		shaft.Provide(provideObjectA),
		shaft.Stack(stackObjectB),
		shaft.Provide(redundantObjectC),
		shaft.Provide(decorateObjectD),
		shaft.Module(
			shaft.Supply(&events, int(123456)),
			shaft.Invoke(func(inputs []I, events *[]string) {
				for _, input := range inputs {
					input.invoke(events)
				}
			}),
		),
	))
	assert.Equal(events, []string{
		"stack b",

		"provide d",
		"provide a",

		// XXX: group are collected in the same order of provision.
		"invoke a",
		"invoke b 123456",
		"invoke d",

		"defer b",
	})
}
