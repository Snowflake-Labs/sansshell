package client

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/Snowflake-Labs/sansshell/client"
	"github.com/google/subcommands"
)

// This file gives an easy way for clients to implement command line completion.
// To test manually, try running a command that sets the appropriate
// environmental variable.
// COMP_LINE="sanssh --" ./sanssh

type isBoolFlag interface {
	IsBoolFlag() bool
}

func flagIsBool(fl string, visitor func(fn func(*flag.Flag))) bool {
	var isBool bool
	visitor(func(f *flag.Flag) {
		if f.Name == fl {
			b, ok := f.Value.(isBoolFlag)
			isBool = isBool || ok && b.IsBoolFlag()
		}
	})
	return isBool
}

type completer interface {
	// ListCommandsAndEssentialFlags lists subcommands and important flags of the
	// current command. Flags should be prefixed with dashes.
	ListCommandsAndEssentialFlags() []string
	// GetCommandCompleter returns a completer for the provided subcommand.
	// The provided subcommand may not exist.
	GetCommandCompleter(cmd string) completer
	// ListFlags lists flags for the current completion level. Flags should be prefixed
	// with dashes.
	ListFlags() []string
	// GetFlag gives possible values for the provided flag.
	// Results are automatically filtered as needed.
	GetFlag(flag, prefix string) []string
	// IsBoolFlag is true if the flag may be set without any value
	IsBoolFlag(flag string) bool
	// GetArgs is called when ListCommands is empty. It's given everything
	// typed so far since the last subcommand.
	// Results are automatically filtered as needed.
	GetArgs(prefix string) []string
}

// cmdCompleter and subCompleter mutually recurse with each other to support
// subcommands of subcommands.
type cmdCompleter struct {
	commander       *subcommands.Commander
	flagPredictions map[string]Predictor
}

func (c *cmdCompleter) ListCommandsAndEssentialFlags() []string {
	var subs []string
	c.commander.VisitCommands(func(_ *subcommands.CommandGroup, cmd subcommands.Command) {
		// Remove some commands that seem unneccesary for command line completion.
		if cmd.Name() != c.commander.CommandsCommand().Name() && cmd.Name() != c.commander.FlagsCommand().Name() {
			subs = append(subs, cmd.Name())
		}
	})
	c.commander.VisitAllImportant(func(f *flag.Flag) {
		subs = append(subs, "--"+f.Name)
	})
	return subs
}
func (c *cmdCompleter) GetCommandCompleter(cmd string) completer {
	var found subcommands.Command
	c.commander.VisitCommands(func(_ *subcommands.CommandGroup, c subcommands.Command) {
		if c.Name() == cmd {
			found = c
		}
	})
	if found == nil {
		return nil
	}
	return &subCompleter{found}
}
func (c *cmdCompleter) ListFlags() []string {
	var flags []string
	c.commander.VisitAll(func(f *flag.Flag) {
		flags = append(flags, "--"+f.Name)
	})
	return flags
}
func (c *cmdCompleter) GetFlag(flag, prefix string) []string {
	if predictor := c.flagPredictions[flag]; predictor != nil {
		return predictor(prefix)
	}
	return []string{prefix}
}
func (c *cmdCompleter) GetArgs(prefix string) []string {
	return nil
}

func (c *cmdCompleter) IsBoolFlag(flag string) bool {
	return flagIsBool(flag, c.commander.VisitAll)
}

func fromSubpackage(s client.HasSubpackage) completer {
	return &cmdCompleter{commander: s.GetSubpackage(flag.NewFlagSet("", flag.ContinueOnError))}
}

type subCompleter struct {
	command subcommands.Command
}

func (c *subCompleter) ListCommandsAndEssentialFlags() []string {
	s, ok := c.command.(client.HasSubpackage)
	if !ok {
		// If there's no subcommands, it's probably useful to
		// suggest all the flags of the command.
		return c.ListFlags()
	}
	return fromSubpackage(s).ListCommandsAndEssentialFlags()
}
func (c *subCompleter) GetCommandCompleter(cmd string) completer {
	s, ok := c.command.(client.HasSubpackage)
	if !ok {
		return nil
	}
	return fromSubpackage(s).GetCommandCompleter(cmd)
}
func (c *subCompleter) ListFlags() []string {
	s := flag.NewFlagSet(c.command.Name(), flag.ContinueOnError)
	c.command.SetFlags(s)
	var flags []string
	s.VisitAll(func(f *flag.Flag) {
		flags = append(flags, "--"+f.Name)
	})
	return flags
}

func (c *subCompleter) GetFlag(flag, prefix string) []string {
	return []string{prefix}
}

func (c *subCompleter) IsBoolFlag(fl string) bool {
	s := flag.NewFlagSet(c.command.Name(), flag.ContinueOnError)
	c.command.SetFlags(s)
	return flagIsBool(fl, s.VisitAll)
}

func (c *subCompleter) GetArgs(prefix string) []string {
	if args, ok := c.command.(client.PredictArgs); ok {
		return args.PredictArgs(prefix)
	}
	return nil
}

func filterToPrefix(args []string, prefix string) []string {
	var s []string
	for _, a := range args {
		if strings.HasPrefix(a, prefix) {
			s = append(s, a)
		}
	}
	return s
}

var validBools = map[string]bool{
	"1": true, "0": true, "t": true, "f": true, "T": true, "F": true, "true": true, "false": true, "TRUE": true, "FALSE": true, "True": true, "False": true,
}

func predict(c completer, args []string) []string {
	// Flags require special handling
	if strings.HasPrefix(args[0], "-") {
		flagName := strings.TrimLeft(args[0], "-")
		switch len(args) {
		case 1:
			if strings.Contains(flagName, "=") {
				split := strings.SplitN(args[0], "=", 2)
				var suggestions []string
				for _, s := range filterToPrefix(c.GetFlag(strings.TrimLeft(split[0], "-"), split[1]), split[1]) {
					suggestions = append(suggestions, split[0]+"="+s)
				}
				return filterToPrefix(suggestions, args[0])
			}
			return filterToPrefix(c.ListFlags(), args[0])
		case 2:
			if c.IsBoolFlag(flagName) {
				return predict(c, args[1:])
			}
			return filterToPrefix(c.GetFlag(flagName, args[1]), args[1])
		default:
			if strings.Contains(flagName, "=") || (c.IsBoolFlag(flagName) && !validBools[args[1]]) {
				return predict(c, args[1:])
			}
			return predict(c, args[2:])
		}
	}

	if len(args) > 1 {
		next := c.GetCommandCompleter(args[0])
		if next == nil {
			prefix := strings.Join(args, " ")
			fmt.Fprintln(os.Stderr, prefix)
			return filterToPrefix(c.GetArgs(prefix), prefix)
		}
		return predict(next, args[1:])
	}
	return filterToPrefix(append(c.ListCommandsAndEssentialFlags(), c.GetArgs(args[0])...), args[0])
}

func predictLine(c completer, line string) []string {
	args := strings.Fields(line)
	if len(args) < 1 {
		return nil
	}
	args = args[1:] // Skip the self-arg
	if line[len(line)-1] == ' ' {
		args = append(args, "") // Append a blank arg at the end when we're starting a new word
	}

	return predict(c, args)
}

// Predictor can provide suggested values based on a provided prefix.
type Predictor func(prefix string) []string

// AddCommandLineCompletion adds command line completion for shells. It should be called after
// registering all subcommands and before calling flag.Parse().
//
// topLevelFlagPredictions gives optional finer-grained control over predictions on the top-level
// flags.
func AddCommandLineCompletion(topLevelFlagPredictions map[string]Predictor) {
	// If COMP_LINE is present, we're running as command completion.
	compLine, ok := os.LookupEnv("COMP_LINE")
	if !ok {
		return
	}

	c := &cmdCompleter{commander: subcommands.DefaultCommander, flagPredictions: topLevelFlagPredictions}

	for _, prediction := range predictLine(c, compLine) {
		fmt.Println(prediction)
	}

	os.Exit(0)
}
