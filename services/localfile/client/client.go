package client

import (
	"context"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"sort"
	"strings"

	"github.com/google/subcommands"
	"google.golang.org/grpc"

	pb "github.com/Snowflake-Labs/sansshell/services/localfile"
)

func init() {
	subcommands.Register(&readCmd{}, "raw")
	subcommands.Register(&statCmd{}, "raw")
	subcommands.Register(&sumCmd{}, "raw")
}

type readCmd struct {
	offset int64
	length int64
}

func (*readCmd) Name() string     { return "read" }
func (*readCmd) Synopsis() string { return "Print a file to stdout." }
func (*readCmd) Usage() string {
	return `read <path> [<path>...]:
  Read from the remote file named by <path> and print it to stdout.
`
}

func (p *readCmd) SetFlags(f *flag.FlagSet) {
	f.Int64Var(&p.offset, "offset", 0, "If positive bytes to skip before reading. If negative apply from the end of the file")
	f.Int64Var(&p.length, "length", 0, "If positive the maximum number of bytes to read")
}

func (p *readCmd) Execute(ctx context.Context, f *flag.FlagSet, args ...interface{}) subcommands.ExitStatus {
	conn := args[0].(*grpc.ClientConn)
	if f.NArg() == 0 {
		fmt.Fprintf(os.Stderr, "Please specify a filename to read.\n")
		return subcommands.ExitUsageError
	}

	for _, filename := range f.Args() {
		err := ReadFile(ctx, conn, filename, p.offset, p.length, os.Stdout)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Could not read file: %v\n", err)
			return subcommands.ExitFailure
		}

	}
	return subcommands.ExitSuccess
}

func ReadFile(ctx context.Context, conn *grpc.ClientConn, filename string, offset int64, length int64, writer io.Writer) error {
	c := pb.NewLocalFileClient(conn)
	stream, err := c.Read(ctx, &pb.ReadRequest{
		Filename: filename,
		Offset:   offset,
		Length:   length,
	})
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

type statCmd struct{}

func (s *statCmd) Name() string     { return "stat" }
func (s *statCmd) Synopsis() string { return "Stat file(s) to stdout." }
func (s *statCmd) Usage() string {
	return `stat <path> [<path>...]:
  Stat one or more remote fles and print the result to stdout.
  `
}

func (s *statCmd) SetFlags(f *flag.FlagSet) {}

func (s *statCmd) Execute(ctx context.Context, f *flag.FlagSet, args ...interface{}) subcommands.ExitStatus {
	conn := args[0].(*grpc.ClientConn)

	if f.NArg() == 0 {
		fmt.Fprintf(os.Stderr, "please specify at least one path to stat\n")
		return subcommands.ExitUsageError
	}
	client := pb.NewLocalFileClient(conn)
	stream, err := client.Stat(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "stat client error: %v\n", err)
		return subcommands.ExitFailure
	}
	// NB: we could gain more performance by asynchronously processing
	// requests and responses, at the expense of needing to manage a
	// goroutine, and non-straightline error handling.
	// Instead, for the sake of simplicity, we're treating the bidirectional
	// stream as a simple request/reply channel
	for _, filename := range f.Args() {
		if err := stream.Send(&pb.StatRequest{Filename: filename}); err != nil {
			fmt.Fprintf(os.Stderr, "stat: send error: %v\n", err)
			return subcommands.ExitFailure
		}
		reply, err := stream.Recv()
		if err != nil {
			fmt.Fprintf(os.Stderr, "stat: receive error: %v\n", err)
			return subcommands.ExitFailure
		}
		mode := os.FileMode(reply.Mode)
		outTmpl := "File: %s\nSize: %d\nType: %s\nAccess: %s Uid: %d Gid: %d\nModify: %s\n"
		fmt.Fprintf(os.Stdout, outTmpl, reply.Filename, reply.Size, fileTypeString(mode), mode, reply.Uid, reply.Gid, reply.Modtime.AsTime())
	}
	return subcommands.ExitSuccess
}

func fileTypeString(mode os.FileMode) string {
	typ := mode.Type()
	switch {
	case typ&fs.ModeDir != 0:
		return "directory"
	case typ&fs.ModeSymlink != 0:
		return "symbolic link"
	case typ&fs.ModeNamedPipe != 0:
		return "FIFO"
	case typ&fs.ModeSocket != 0:
		return "socket"
	case typ&fs.ModeCharDevice != 0:
		return "character device"
	case typ&fs.ModeDevice != 0:
		return "block device"
	case typ&fs.ModeIrregular != 0:
		return "irregular file"
	case typ&fs.ModeType == 0:
		return "regular file"
	default:
		return "UNKNOWN"
	}
}

type sumCmd struct {
	sumType string
}

func (s *sumCmd) Name() string     { return "sum" }
func (s *sumCmd) Synopsis() string { return "Calculate file sums" }
func (s *sumCmd) Usage() string {
	return `sum [--sumtype type ] <path> [<path>...]
  Calculate sums for one or more remotes paths and print the result (as a hexidecimal string) to stdout.
  `
}

func flagToType(val string) (pb.SumType, error) {
	v := fmt.Sprintf("SUM_TYPE_%s", strings.ToUpper(val))
	i, ok := pb.SumType_value[v]
	if !ok {
		return pb.SumType_SUM_TYPE_UNKNOWN, fmt.Errorf("no such sumtype value: %s", v)
	}
	return pb.SumType(i), nil
}

func (s *sumCmd) SetFlags(f *flag.FlagSet) {
	var shortNames []string
	for k := range pb.SumType_value {
		shortNames = append(shortNames, strings.TrimPrefix(k, "SUM_TYPE_"))
	}
	sort.Strings(shortNames)
	f.StringVar(&s.sumType, "sumtype", "SHA256", fmt.Sprintf("Type of sum to calculate (one of: [%s])", strings.Join(shortNames, ",")))
}

func (s *sumCmd) Execute(ctx context.Context, f *flag.FlagSet, args ...interface{}) subcommands.ExitStatus {
	conn := args[0].(*grpc.ClientConn)
	if f.NArg() == 0 {
		fmt.Fprintf(os.Stderr, "please specify a filename to sum\n")
		return subcommands.ExitUsageError
	}
	sumType, err := flagToType(s.sumType)
	if err != nil {
		fmt.Fprintf(os.Stderr, "flag error: %v\n", err)
		return subcommands.ExitUsageError
	}
	client := pb.NewLocalFileClient(conn)
	stream, err := client.Sum(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "sum client error: %v\n", err)
		return subcommands.ExitFailure
	}
	// NB: we could gain more performance by asynchronously processing
	// requests and responses, at the expense of needing to manage a
	// goroutine, and non-straightline error handling.
	// Instead, for the sake of simplicity, we're treating the bidirectional
	// stream as a simple request/reply channel
	for _, filename := range f.Args() {
		if err := stream.Send(&pb.SumRequest{Filename: filename, SumType: sumType}); err != nil {
			fmt.Fprintf(os.Stderr, "sum: send error: %v\n", err)
			return subcommands.ExitFailure
		}
		reply, err := stream.Recv()
		if err != nil {
			fmt.Fprintf(os.Stderr, "stat: receive error: %v\n", err)
			return subcommands.ExitFailure
		}
		fmt.Fprintf(os.Stdout, "%s %s\n", reply.Sum, reply.Filename)
	}
	return subcommands.ExitSuccess
}
