package client

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/google/subcommands"
	"google.golang.org/grpc"

	pb "github.com/Snowflake-Labs/sansshell/services/healthcheck"
)

func init() {
	subcommands.Register(&healthcheckCmd{}, "raw")
}

type healthcheckCmd struct{}

func (*healthcheckCmd) Name() string     { return "healthcheck" }
func (*healthcheckCmd) Synopsis() string { return "Confirm connectivity to working server." }
func (*healthcheckCmd) Usage() string {
	return `healthcheck:
  Sends an empty request and expects an empty response.  Only prints errors.
`
}

func (p *healthcheckCmd) SetFlags(f *flag.FlagSet) {}

func (p *healthcheckCmd) Execute(ctx context.Context, f *flag.FlagSet, args ...interface{}) subcommands.ExitStatus {
	conn := args[0].(*grpc.ClientConn)
	c := pb.NewHealthCheckClient(conn)

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	_, err := c.Ok(ctx, &pb.Empty{})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Could not healthcheck server: %v\n", err)
		return subcommands.ExitFailure
	}
	return subcommands.ExitSuccess
}
