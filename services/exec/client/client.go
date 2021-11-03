package client

import (
	"context"
	"flag"
	"fmt"
	"os"

	pb "github.com/Snowflake-Labs/sansshell/services/exec"
	"github.com/google/subcommands"
	"google.golang.org/grpc"
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
	conn := args[0].(grpc.ClientConnInterface)
	if f.NArg() == 0 {
		fmt.Fprintf(os.Stderr, "Please specify a command to execute.\n")
		return subcommands.ExitUsageError
	}

	c := pb.NewExecClient(conn)

	resp, err := c.Run(ctx, &pb.ExecRequest{Command: f.Args()[0], Args: f.Args()[1:]})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Could not execute due to likely program failure: %v\n", err)
		return subcommands.ExitFailure
	}
	if len(resp.Stderr) > 0 {
		fmt.Fprintf(os.Stderr, "Command execution failure: %v\n", string(resp.Stderr))
		return subcommands.ExitStatus(resp.RetCode)
	}

	fmt.Fprintf(os.Stdout, "Command execution success: %v\n", string(resp.Stdout))
	return subcommands.ExitSuccess
}
