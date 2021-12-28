package xflags

import (
	"fmt"
	"io"
	"strings"
	"text/tabwriter"
)

// Formatter is a function that prints a help message for a command.
type Formatter func(w io.Writer, cmd *Command) error

// DefaultFormatter is a Formatter function that prints a help message for a
// command.
func DefaultFormatter(w io.Writer, cmd *Command) error {
	aw := newAggregatedWriter(w)
	if err := printUsage(aw, cmd); err != nil {
		return err
	}
	if cmd.Usage != "" {
		fmt.Fprintf(aw, "\n%s\n", cmd.Usage)
	}
	if err := detailPositionals(aw, cmd); err != nil {
		return err
	}
	for _, group := range cmd.FlagGroups {
		if err := detailFlagGroup(aw, group); err != nil {
			return err
		}
	}
	if err := detailSubcommands(aw, cmd.Subcommands); err != nil {
		return err
	}
	if err := detailEnvVars(aw, cmd); err != nil {
		return err
	}
	if cmd.Synopsis != "" {
		fmt.Fprintf(aw, "\n%s\n", cmd.Synopsis)
	}
	return aw.Err()
}

func getPositionals(cmd *Command) []*Flag {
	a := make([]*Flag, 0, 8)
	for _, group := range cmd.FlagGroups {
		for _, flag := range group.Flags {
			if flag.Hidden || !flag.Positional {
				continue
			}
			a = append(a, flag)
		}
	}
	return a
}

func hasRegular(cmd *Command) bool {
	if cmd == nil {
		return false
	}
	for _, group := range cmd.FlagGroups {
		for _, flag := range group.Flags {
			if flag.Hidden || flag.Positional {
				continue
			}
			return true
		}
	}
	return hasRegular(cmd.Parent)
}

func printUsage(w io.Writer, cmd *Command) error {
	fullName := cmd.Name
	for p := cmd.Parent; p != nil; p = p.Parent {
		fullName = fmt.Sprintf("%s %s", p.Name, fullName)
	}
	fmt.Fprintf(w, "Usage: %s", fullName)
	if hasRegular(cmd) {
		fmt.Fprintf(w, " [OPTIONS]")
	}
	if len(cmd.Subcommands) > 0 {
		fmt.Fprintf(w, " COMMAND")
	}
	for _, flag := range getPositionals(cmd) {
		name := strings.ToUpper(flag.Name)
		if flag.MinCount == 0 {
			if flag.MaxCount == 1 {
				fmt.Fprintf(w, " [%s]", name)
			} else {
				fmt.Fprintf(w, " [%s...]", name)
			}
		} else {
			if flag.MinCount == 1 && flag.MaxCount == 1 {
				fmt.Fprintf(w, " %s", name)
			} else {
				fmt.Fprintf(w, " %s...", name)
			}
		}
	}
	fmt.Fprintf(w, "\n")
	return nil
}

func detailPositionals(w io.Writer, cmd *Command) error {
	flags := getPositionals(cmd)
	if len(flags) == 0 {
		return nil
	}
	fmt.Fprintf(w, "\nPositional arguments:\n")
	w = tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	for _, flag := range flags {
		fmt.Fprintf(w, "  %s", strings.ToUpper(flag.Name))
		if flag.Usage != "" {
			fmt.Fprintf(w, "\t%s", flag.Usage)
			if flag.ShowDefault {
				fmt.Fprintf(w, " (default: %s)", flag.Value)
			}
		}
		fmt.Fprintf(w, "\n")
	}
	return w.(*tabwriter.Writer).Flush()
}

func filterRegular(flags []*Flag) []*Flag {
	a := make([]*Flag, 0, 8)
	for _, flag := range flags {
		if flag.Hidden || flag.Positional {
			continue
		}
		a = append(a, flag)
	}
	return a
}

func detailFlagGroup(w io.Writer, group *FlagGroup) error {
	flags := filterRegular(group.Flags)
	if len(flags) == 0 {
		return nil
	}
	fmt.Fprintf(w, "\n%s:\n", group.Usage)
	w = tabwriter.NewWriter(w, 0, 0, 1, ' ', 0)
	for _, flag := range flags {
		var name, shortName string
		if flag.Name != "" {
			name = fmt.Sprintf("--%s", flag.Name)
		}
		if flag.ShortName != "" {
			if flag.Name != "" {
				shortName = fmt.Sprintf("-%s,", flag.ShortName)
			} else {
				shortName = fmt.Sprintf("-%s", flag.ShortName)
			}
		}
		fmt.Fprintf(w, "  %s\t%s\t %s", shortName, name, flag.Usage)
		if flag.ShowDefault {
			fmt.Fprintf(w, " (default: %s)", flag.Value)
		}
		fmt.Fprintf(w, "\n")
	}
	return w.(*tabwriter.Writer).Flush()
}

func getEnvVars(a []*Flag, cmd *Command) []*Flag {
	if cmd == nil {
		return a
	}
	a = getEnvVars(a, cmd.Parent)
	for _, group := range cmd.FlagGroups {
		for _, flag := range group.Flags {
			if flag.EnvVar == "" || flag.Hidden {
				continue
			}
			a = append(a, flag)
		}
	}
	return a
}

func detailEnvVars(w io.Writer, cmd *Command) error {
	flags := getEnvVars(nil, cmd)
	if len(flags) == 0 {
		return nil
	}
	fmt.Fprintf(w, "\nEnvironment variables:\n")
	w = tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	for _, flag := range flags {
		fmt.Fprintf(
			w,
			"  %s\t%s\n",
			strings.ToUpper(flag.EnvVar),
			flag.Usage,
		)
	}
	return w.(*tabwriter.Writer).Flush()
}

func detailSubcommands(w io.Writer, subcommands []*Command) error {
	// TODO: wrap final column to terminal
	if len(subcommands) == 0 {
		return nil
	}
	fmt.Fprintf(w, "\nCommands:\n")
	w = tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	for _, cmd := range subcommands {
		if cmd.Hidden {
			continue
		}
		fmt.Fprintf(w, "  %s\t%s\n", cmd.Name, cmd.Usage)
	}
	return w.(*tabwriter.Writer).Flush()
}
