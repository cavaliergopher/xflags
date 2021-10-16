package xflags

import (
	"fmt"
	"io"
	"strings"
	"text/tabwriter"
)

// TODO: wrap final column to term width

// Formatter is a function that prints a help message for a command.
type Formatter func(w io.Writer, info *CommandInfo) error

// DefaultFormatter is a Formatter function that prints a help message for a
// command.
func DefaultFormatter(w io.Writer, info *CommandInfo) error {
	aw := newAggregatedWriter(w)
	if err := formatUsage(aw, info); err != nil {
		return err
	}
	if info.Usage != "" {
		fmt.Fprintf(aw, "\n%s\n", info.Usage)
	}
	if err := formatPositionalFlags(aw, info.Flags); err != nil {
		return err
	}
	for _, group := range info.FlagGroups {
		if err := formatFlagGroup(aw, group); err != nil {
			return err
		}
	}
	if err := formatSubcommands(aw, info.Subcommands); err != nil {
		return err
	}
	if err := formatEnvVars(aw, info.Flags); err != nil {
		return err
	}
	if info.Synopsis != "" {
		fmt.Fprintf(aw, "\n%s\n", info.Synopsis)
	}
	return aw.Err()
}

func filterPositionalFlags(flags []*FlagInfo) []*FlagInfo {
	a := make([]*FlagInfo, 0, 2)
	for _, flagInfo := range flags {
		if flagInfo.Hidden || !flagInfo.Positional {
			continue
		}
		a = append(a, flagInfo)
	}
	return a
}

func filterRegularFlags(flags []*FlagInfo) []*FlagInfo {
	a := make([]*FlagInfo, 0, len(flags)/2)
	for _, flagInfo := range flags {
		if flagInfo.Hidden || flagInfo.Positional {
			continue
		}
		a = append(a, flagInfo)
	}
	return a
}

func filterEnvironmentFlags(flags []*FlagInfo) []*FlagInfo {
	a := make([]*FlagInfo, 0, 2)
	for _, flagInfo := range flags {
		if flagInfo.Hidden || flagInfo.EnvVar == "" {
			continue
		}
		a = append(a, flagInfo)
	}
	return a
}

func formatUsage(w io.Writer, info *CommandInfo) error {
	fullName := info.Name
	for p := info.Parent; p != nil; p = p.Parent {
		fullName = fmt.Sprintf("%s %s", p.Name, fullName)
	}
	fmt.Fprintf(w, "Usage: %s", fullName)
	if len(filterRegularFlags(info.Flags)) > 0 {
		fmt.Fprintf(w, " [OPTIONS]")
	}
	if len(info.Subcommands) > 0 {
		fmt.Fprintf(w, " COMMAND")
	}
	for _, flagInfo := range filterPositionalFlags(info.Flags) {
		name := strings.ToUpper(flagInfo.Name)
		if flagInfo.MinCount == 0 {
			if flagInfo.MaxCount == 1 {
				fmt.Fprintf(w, " [%s]", name)
			} else {
				fmt.Fprintf(w, " [%s...]", name)
			}
		} else {
			if flagInfo.MinCount == 1 && flagInfo.MaxCount == 1 {
				fmt.Fprintf(w, " %s", name)
			} else {
				fmt.Fprintf(w, " %s...", name)
			}
		}
	}
	fmt.Fprintf(w, "\n")
	return nil
}

func formatPositionalFlags(w io.Writer, flags []*FlagInfo) error {
	flags = filterPositionalFlags(flags)
	if len(flags) == 0 {
		return nil
	}
	fmt.Fprintf(w, "\nPositional arguments:\n")
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	for _, flagInfo := range flags {
		fmt.Fprintf(tw, "  %s", strings.ToUpper(flagInfo.Name))
		if flagInfo.Usage != "" {
			fmt.Fprintf(tw, "\t%s", flagInfo.Usage)
			if flagInfo.ShowDefault {
				fmt.Fprintf(w, " (default: %s)", flagInfo.Value)
			}
		}
		fmt.Fprintf(tw, "\n")
	}
	return tw.Flush()
}

func formatFlagGroup(w io.Writer, group *FlagGroupInfo) error {
	flags := filterRegularFlags(group.Flags)
	if len(flags) == 0 {
		return nil
	}
	fmt.Fprintf(w, "\n%s:\n", group.Usage)
	tw := tabwriter.NewWriter(w, 0, 0, 1, ' ', 0)
	for _, flagInfo := range flags {
		var name, ShortName string
		if flagInfo.ShortName == "" {
			if len(flagInfo.Name) == 1 {
				ShortName = fmt.Sprintf("-%s", flagInfo.Name)
			} else {
				name = fmt.Sprintf("--%s", flagInfo.Name)
			}
		} else {
			name = fmt.Sprintf("--%s", flagInfo.Name)
			ShortName = fmt.Sprintf("-%s,", flagInfo.ShortName)
		}
		fmt.Fprintf(tw, "  %s\t%s\t %s", ShortName, name, flagInfo.Usage)
		if flagInfo.ShowDefault {
			fmt.Fprintf(w, " (default: %s)", flagInfo.Value)
		}
		fmt.Fprintf(tw, "\n")
	}
	return tw.Flush()
}

func formatEnvVars(w io.Writer, flags []*FlagInfo) error {
	flags = filterEnvironmentFlags(flags)
	if len(flags) == 0 {
		return nil
	}
	fmt.Fprintf(w, "\nEnvironment variables:\n")
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	for _, flagInfo := range flags {
		fmt.Fprintf(
			tw,
			"  %s\t%s\n",
			strings.ToUpper(flagInfo.EnvVar),
			flagInfo.Usage,
		)
	}
	return tw.Flush()
}

func formatSubcommands(w io.Writer, subcommands []*CommandInfo) error {
	// TODO: wrap final column to term
	if len(subcommands) == 0 {
		return nil
	}
	fmt.Fprintf(w, "\nCommands:\n")
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	for _, cmd := range subcommands {
		if cmd.Hidden {
			continue
		}
		fmt.Fprintf(tw, "  %s\t%s\n", cmd.Name, cmd.Usage)
	}
	return tw.Flush()
}
