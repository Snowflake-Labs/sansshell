package client

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/google/subcommands"
	"google.golang.org/grpc"

	pb "github.com/snowflakedb/unshelled/services/localfile"
)

func init() {
	subcommands.Register(&readCmd{}, "raw")
}

type readCmd struct{}

func (*readCmd) Name() string     { return "read" }
func (*readCmd) Synopsis() string { return "Print a file to stdout." }
func (*readCmd) Usage() string {
	return `read <path> [<path>...]:
  Read from the remote file named by <path> and print it to stdout.

  Note: This is not optimized for large files.  If it doesn't fit in memory in
  a single proto field, you'll have a bad time.
`
}

func (p *readCmd) SetFlags(f *flag.FlagSet) {}

func (p *readCmd) Execute(ctx context.Context, f *flag.FlagSet, args ...interface{}) subcommands.ExitStatus {
	conn := args[0].(*grpc.ClientConn)
	if f.NArg() == 0 {
		fmt.Fprintf(os.Stderr, "Please specify a filename to read.\n")
		return subcommands.ExitUsageError
	}

	c := pb.NewLocalFileClient(conn)

	for _, filename := range f.Args() {
		resp, err := c.Read(ctx, &pb.ReadRequest{Filename: filename})
		if err != nil {
			fmt.Fprintf(os.Stderr, "Could not read file: %v\n", err)
			return subcommands.ExitFailure
		}
		fmt.Println(string(resp.GetContents()))
	}
	return subcommands.ExitSuccess
}
