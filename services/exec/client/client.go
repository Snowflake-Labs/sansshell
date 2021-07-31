package client

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/google/subcommands"
	pb "github.com/snowflakedb/unshelled/services/exec"
	"google.golang.org/grpc"
)

func init() {
	subcommands.Register(&execCmd{}, "raw")
}

type execCmd struct{}

func (*execCmd) Name() string     { return "run" }
func (*execCmd) Synopsis() string { return "Run provided command and return a response." }
func (*execCmd) Usage() string {
	return `run <command> [<args>...]:
  Run a command remotely and return the response

  Note: This is not optimized for long running commands.  If it doesn't fit in memory in
  a single proto field or if it doesnt complete within a timeout, you'll have a bad time.
  Default timeout is 30 sec, you can change it on command line by providing value for
  timeout cli parameter
`
}

func (p *execCmd) SetFlags(f *flag.FlagSet) {}

func (p *execCmd) Execute(ctx context.Context, f *flag.FlagSet, args ...interface{}) subcommands.ExitStatus {
	conn := args[0].(*grpc.ClientConn)
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
