package client

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/google/subcommands"

	pb "github.com/Snowflake-Labs/sansshell/services/healthcheck"
	"github.com/Snowflake-Labs/sansshell/services/util"
)

func init() {
	subcommands.Register(&healthcheckCmd{}, "healthcheck")
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
	state := args[0].(*util.ExecuteState)
	c := pb.NewHealthCheckClientProxy(state.Conn)

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	resp, err := c.OkOneMany(ctx, &pb.Empty{})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Could not healthcheck server: %v\n", err)
		return subcommands.ExitFailure
	}

	retCode := subcommands.ExitSuccess
	for r := range resp {
		if r.Error != nil {
			fmt.Fprintf(state.Out[r.Index], "Healthcheck for target %s (%d) returned error: %v\n", r.Target, r.Index, r.Error)
			retCode = subcommands.ExitFailure
		}
	}
	return retCode
}
