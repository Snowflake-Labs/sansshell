package client

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/google/subcommands"
	"google.golang.org/grpc"

	pb "github.com/Snowflake-Labs/sansshell/services/process"
)

func init() {
	subcommands.Register(&processCmd{}, "raw")
}

type processCmd struct {
	pid int64
}

func (*processCmd) Name() string     { return "ps" }
func (*processCmd) Synopsis() string { return "Retrieve process list." }
func (*processCmd) Usage() string {
	return `ps:
  Read the process list from the remote machine.
`
}

func (p *processCmd) SetFlags(f *flag.FlagSet) {
	f.Int64Var(&p.pid, "pid", 0, "If positive restrict request to this pid only")
}

func (p *processCmd) Execute(ctx context.Context, f *flag.FlagSet, args ...interface{}) subcommands.ExitStatus {
	conn := args[0].(*grpc.ClientConn)

	c := pb.NewProcessClient(conn)

	req := &pb.ListRequest{}
	if p.pid > 0 {
		req.Pids = append(req.Pids, p.pid)
	}
	resp, err := c.List(ctx, req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "List returned error: %v\n", err)
		return subcommands.ExitFailure
	}

	for _, p := range resp.ProcessEntries {
		fmt.Printf("%+v\n", p)
	}

	return subcommands.ExitSuccess
}
