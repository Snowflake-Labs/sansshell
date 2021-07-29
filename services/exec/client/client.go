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

func (*execCmd) Name() string     { return "exec" }
func (*execCmd) Synopsis() string { return "Execute provided command and return a response." }
func (*execCmd) Usage() string {
	return `exec <command> [<args>...]:
  Execute a command remotely and return the response

  Note: This is not optimized for long running commands.  If it doesn't fit in memory in
  a single proto field or if it doesnt complete within 30 secs, you'll have a bad time 
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

	resp, err := c.Exec(ctx, &pb.ExecRequest{Command: f.Args()})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Could not execute: %v\n", err)
		return subcommands.ExitFailure
	}
	if len(resp.Error) > 0 {
		fmt.Fprintf(os.Stderr, "Command execution failure: %v\n", string(resp.Error))
		return subcommands.ExitFailure
	}
	fmt.Println(string(resp.GetOutput()))

	return subcommands.ExitSuccess
}
