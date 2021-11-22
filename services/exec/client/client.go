package client

import (
	"context"
	"flag"
	"fmt"
	"os"

	pb "github.com/Snowflake-Labs/sansshell/services/exec"
	"github.com/Snowflake-Labs/sansshell/services/util"
	"github.com/google/subcommands"
)

func init() {
	subcommands.Register(&execCmd{}, "exec")
}

type execCmd struct{}

func (*execCmd) Name() string     { return "run" }
func (*execCmd) Synopsis() string { return "Run provided command and return a response." }
func (*execCmd) Usage() string {
	return `run <command> [<args>...]:
  Run a command remotely and return the response

	Note: This is not optimized for large output or long running commands.  If
	the output doesn't fit in memory in a single proto message or if it doesnt
	complete within the timeout, you'll have a bad time.
`
}

func (p *execCmd) SetFlags(f *flag.FlagSet) {}

func (p *execCmd) Execute(ctx context.Context, f *flag.FlagSet, args ...interface{}) subcommands.ExitStatus {
	state := args[0].(*util.ExecuteState)
	if f.NArg() == 0 {
		fmt.Fprintf(os.Stderr, "Please specify a command to execute.\n")
		return subcommands.ExitUsageError
	}

	c := pb.NewExecClientProxy(state.Conn)

	resp, err := c.RunOneMany(ctx, &pb.ExecRequest{Command: f.Args()[0], Args: f.Args()[1:]})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Could not execute due to likely program failure: %v\n", err)
		return subcommands.ExitFailure
	}

	returnCode := subcommands.ExitSuccess
	for r := range resp {
		// TODO(jchacon): Is stderr output really an error? We should just depend on return code most likely.
		if r.Error != nil || len(r.Resp.Stderr) > 0 {
			fmt.Fprintf(state.Out[r.Index], "Command execution failure for target %s (%d) - error - %v\nStderr:\n%v\n", r.Target, r.Index, r.Error, string(r.Resp.Stderr))
			returnCode = subcommands.ExitFailure
			continue
		}
		fmt.Fprintf(state.Out[r.Index], "Command execution success: %v\n", string(r.Resp.Stdout))
	}
	return returnCode
}
