// Package serpent provides a way to combine the shaft framework
// with github.com/spf13/cobra nicely, providing a dependency
// injection style monolithc CLI interface.
package serpent

import (
	"context"
	"errors"

	"github.com/aegistudio/shaft"
	"github.com/aegistudio/shaft/core"
	"github.com/spf13/cobra"
)

type commandOptionKey struct{}

type commandOptionValue struct {
	options []core.Option
}

// retrieveOptionValue attempts to retrieve option value from the
// context. It returns the error when the command is not executed
// with serpent.ExecuteContext or serpent.Execute.
func retrieveOptionValue(cmd *cobra.Command) (*commandOptionValue, error) {
	dstCtx := context.Background()
	if ctx := cmd.Context(); ctx != nil {
		dstCtx = ctx
	}
	value, ok := dstCtx.Value(commandOptionKey{}).(*commandOptionValue)
	if !ok {
		return nil, errors.New(
			"must execute command with serpent.Execute or serpent.ExecuteContext")
	}
	return value, nil
}

// ExecuteContext sets up the command context and invoke the
// specified function.
func ExecuteContext(
	ctx context.Context, cmd *cobra.Command, options ...core.Option,
) error {
	ctx = context.WithValue(ctx, commandOptionKey{}, &commandOptionValue{
		options: options,
	})
	return cmd.ExecuteContext(ctx)
}

// Execute sets up the command context and invoke the specified
// functions.
func Execute(cmd *cobra.Command, options ...core.Option) error {
	return ExecuteContext(context.Background(), cmd, options...)
}

// AddOption attempts add options to the current command.
func AddOption(cmd *cobra.Command, options ...core.Option) error {
	value, err := retrieveOptionValue(cmd)
	if err != nil {
		return err
	}
	value.options = append(value.options, options...)
	return nil
}

// Executor is the executor for this command. We usually attach
// the executor's corresponding methods to cobra.Command's RunE
// or PersistentPreRunE field.
//
// When PersistentPreRunE is attached, the command provides
// dependencies specified in the executor to subcommands under
// its directory.
//
// When RunE is attached, the command collects all previously
// provided options up to this node and execute them.
type Executor core.Option

// CommandObject is the command object that is executed.
type CommandObject *cobra.Command

// CommandContext is the context of the executed command.
type CommandContext context.Context

// CommandArgs is the arguments passed in the command.
type CommandArgs []string

func (e Executor) PersistentRunE(cmd *cobra.Command, _ []string) error {
	return AddOption(cmd, core.Option(e))
}

func (e Executor) RunE(cmd *cobra.Command, args []string) error {
	value, err := retrieveOptionValue(cmd)
	if err != nil {
		return err
	}
	return core.Run(
		shaft.Supply(CommandObject(cmd)),
		shaft.Supply(CommandContext(cmd.Context())),
		shaft.Supply(CommandArgs(args)),
		core.Module(value.options...), core.Option(e),
	)
}
