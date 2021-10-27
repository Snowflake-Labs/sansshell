package client

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/google/subcommands"
	"google.golang.org/grpc"

	pb "github.com/Snowflake-Labs/sansshell/services/ansible"
)

// A type for a custom flag for a list of strings in a comma separated list.
type stringList []*pb.Var

// String implements as needed for flag.Value
func (i *stringList) String() string {
	var out bytes.Buffer

	for _, v := range *i {
		out.WriteString(fmt.Sprintf("%s=%s,", v.Key, v.Value))
	}
	o := out.String()
	// Trim last , off the end
	if len(o) > 0 {
		o = o[0 : len(o)-1]
	}
	return o
}

// Set implements parsing for strings list flags as needed
// for flag.Value
func (i *stringList) Set(val string) error {
	if len(*i) > 0 {
		return errors.New("stringlist flag already set")
	}
	for _, kv := range strings.Split(val, ",") {
		item := strings.Split(kv, "=")
		if len(item) != 2 {
			return fmt.Errorf("bad key=value: %s", kv)
		}
		*i = append(*i, &pb.Var{
			Key:   item[0],
			Value: item[1],
		})
	}

	return nil
}

func init() {
	subcommands.Register(&ansibleCmd{}, "ansible")
}

type ansibleCmd struct {
	playbook string
	vars     stringList
	user     string
	check    bool
	diff     bool
	verbose  bool
}

func (*ansibleCmd) Name() string     { return "ansible" }
func (*ansibleCmd) Synopsis() string { return "Run an ansible playbook on the server." }
func (*ansibleCmd) Usage() string {
	return `ansible:
  Run an ansible playbook on the remote server.
`
}

func (a *ansibleCmd) SetFlags(f *flag.FlagSet) {
	f.StringVar(&a.playbook, "playbook", "", "The absolute path to the playbook to execute on the remote server.")
	f.Var(&a.vars, "vars", "Pass key=value (via -e) to ansible-playbook. Multiple values can be specified separated by commas")
	f.StringVar(&a.user, "user", "", "Run the playbook as this user")
	f.BoolVar(&a.check, "check", false, "If true the playbook will be run with --check passed as an argument")
	f.BoolVar(&a.diff, "diff", false, "If true the playbook will be run with --diff passed as an argument")
	f.BoolVar(&a.verbose, "verbose", false, "If true the playbook wiill be run with -vvv passed as an argument")
}

func (a *ansibleCmd) Execute(ctx context.Context, f *flag.FlagSet, args ...interface{}) subcommands.ExitStatus {
	if a.playbook == "" {
		fmt.Fprintln(os.Stderr, "--playbook is required")
		return subcommands.ExitFailure
	}

	conn := args[0].(grpc.ClientConnInterface)

	c := pb.NewPlaybookClient(conn)

	req := &pb.RunRequest{
		Playbook: a.playbook,
		Vars:     a.vars,
		User:     a.user,
		Check:    a.check,
		Diff:     a.diff,
		Verbose:  a.verbose,
	}

	resp, err := c.Run(ctx, req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Run returned error: %v\n", err)
		return subcommands.ExitFailure
	}
	log.Printf("Return code: %d\nStdout:%s\nStderr:%s\n", resp.ReturnCode, resp.Stdout, resp.Stderr)
	return subcommands.ExitSuccess
}
