package client

import (
	"context"
	"flag"
	"fmt"
	"io"
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
`
}

func (p *readCmd) SetFlags(f *flag.FlagSet) {}

func (p *readCmd) Execute(ctx context.Context, f *flag.FlagSet, args ...interface{}) subcommands.ExitStatus {
	conn := args[0].(*grpc.ClientConn)
	if f.NArg() == 0 {
		fmt.Fprintf(os.Stderr, "Please specify a filename to read.\n")
		return subcommands.ExitUsageError
	}

	for _, filename := range f.Args() {
		err := ReadFile(ctx, conn, filename, os.Stdout)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Could not read file: %v\n", err)
			return subcommands.ExitFailure
		}

	}
	return subcommands.ExitSuccess
}

func ReadFile(ctx context.Context, conn *grpc.ClientConn, filename string, writer io.Writer) error {
	c := pb.NewLocalFileClient(conn)
	stream, err := c.Read(ctx, &pb.ReadRequest{Filename: filename})
	if err != nil {
		return err
	}

	for {
		resp, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		contents := resp.GetContents()
		n, err := writer.Write(contents)
		if got, want := n, len(contents); got != want {
			return fmt.Errorf("can't write into buffer at correct length. Got %d want %d", got, want)
		}
		if err != nil {
			return err
		}
	}
	return nil
}
