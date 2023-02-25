package client

import (
	"context"
	"flag"
	"reflect"
	"sort"
	"testing"

	"github.com/Snowflake-Labs/sansshell/client"
	"github.com/google/subcommands"
)

type subCmd struct{ name string }

func (s *subCmd) Name() string           { return s.name }
func (*subCmd) Synopsis() string         { return "" }
func (*subCmd) Usage() string            { return "" }
func (*subCmd) SetFlags(f *flag.FlagSet) { f.String("foo", "", "") }
func (s *subCmd) Execute(ctx context.Context, f *flag.FlagSet, args ...interface{}) subcommands.ExitStatus {
	return s.GetSubpackage(f).Execute(ctx, args...)
}
func (s *subCmd) GetSubpackage(f *flag.FlagSet) *subcommands.Commander {
	return client.SetupSubpackage(s.name, f)
}

type emptyCmd struct{ name string }

func (*emptyCmd) Name() string             { return "empty" }
func (*emptyCmd) Synopsis() string         { return "" }
func (*emptyCmd) Usage() string            { return "" }
func (*emptyCmd) SetFlags(f *flag.FlagSet) { f.Bool("ebool", false, "") }
func (*emptyCmd) Execute(context.Context, *flag.FlagSet, ...interface{}) subcommands.ExitStatus {
	return subcommands.ExitSuccess
}
func (*emptyCmd) PredictArgs(string) []string { return []string{"emptyargv"} }

func TestPredict(t *testing.T) {
	topLevelFlags := flag.NewFlagSet("", flag.ContinueOnError)
	topLevelFlags.String("important", "", "")
	topLevelFlags.String("unimportant", "", "")
	topLevelFlags.Bool("boolean", false, "")
	cmdr := subcommands.NewCommander(topLevelFlags, "sanssh")
	cmdr.Register(&subCmd{name: "sub"}, "")
	cmdr.Register(&emptyCmd{}, "")
	cmdr.ImportantFlag("important")
	c := &cmdCompleter{commander: cmdr, flagPredictions: map[string]Predictor{
		"unimportant": func(string) []string { return []string{"ab", "cd"} },
	}}

	for _, tc := range []struct {
		desc string
		line string
		want []string
	}{
		{
			desc: "empty line",
			line: "",
			want: nil,
		},
		{
			desc: "command start",
			line: "sanssh ",
			want: []string{"--important", "empty", "sub"},
		},
		{
			desc: "choosing flags",
			line: "sanssh -",
			want: []string{"--boolean", "--important", "--unimportant"},
		},
		{
			desc: "choosing flag values",
			line: "sanssh --unimportant ",
			want: []string{"ab", "cd"},
		},
		{
			desc: "choosing flag values, no prediction",
			line: "sanssh --important s",
			want: []string{"s"},
		},
		{
			desc: "choosing flag values, equals",
			line: "sanssh --unimportant=",
			want: []string{"--unimportant=ab", "--unimportant=cd"},
		},
		{
			desc: "choosing flag values, equals and partial",
			line: "sanssh --unimportant=a",
			want: []string{"--unimportant=ab"},
		},
		{
			desc: "completing subcommand",
			line: "sanssh s",
			want: []string{"sub"},
		},
		{
			desc: "completing subsubcommand",
			line: "sanssh sub ",
			want: []string{"help"},
		},
		{
			desc: "subcommand flag",
			line: "sanssh sub --fo",
			want: []string{"--foo"},
		},
		{
			desc: "subcommand flag val",
			line: "sanssh sub --foo bar",
			want: []string{"bar"},
		},
		{
			desc: "finished subsubcommand",
			line: "sanssh sub help ",
			want: nil,
		},
		{
			desc: "trying wrong subsubcommand",
			line: "sanssh sub help args ",
			want: nil,
		},
		{
			desc: "flag and subcommand",
			line: "sanssh --important foo s",
			want: []string{"sub"},
		},
		{
			desc: "bool flag and subcommand",
			line: "sanssh -boolean --important foo s",
			want: []string{"sub"},
		},
		{
			desc: "fake subcommand",
			line: "sanssh notfound ",
			want: nil,
		},
		{
			desc: "empty subcommand",
			line: "sanssh empty ",
			want: []string{"--ebool", "emptyargv"},
		},
		{
			desc: "empty args",
			line: "sanssh empty e",
			want: []string{"emptyargv"},
		},
		{
			desc: "empty args with flag",
			line: "sanssh empty -ebool e",
			want: []string{"emptyargv"},
		},
		{
			desc: "empty args with not-subcommand",
			line: "sanssh empty args ",
			want: nil,
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			got := predictLine(c, tc.line)
			sort.Strings(got)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("got %v, want %v", got, tc.want)
			}

		})
	}
}
