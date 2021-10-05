package xflags

import (
	"fmt"
	"io"
	"strings"
	"text/tabwriter"
)

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
	if err := formatFlags(aw, info.Flags); err != nil {
		return err
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

func getRegularFlags(flags []*FlagInfo) []*FlagInfo {
	out := make([]*FlagInfo, 0, len(flags))
	for _, flag := range flags {
		if flag.Hidden || flag.Positional {
			continue
		}
		out = append(out, flag)
	}
	return out
}

func getPosFlag(flags []*FlagInfo) *FlagInfo {
	for _, flag := range flags {
		if flag.Positional && !flag.Hidden {
			return flag
		}
	}
	return nil
}

func formatUsage(w io.Writer, info *CommandInfo) error {
	fullName := info.Name
	for p := info.Parent; p != nil; p = p.Parent {
		fullName = fmt.Sprintf("%s %s", p.Name, fullName)
	}
	fmt.Fprintf(w, "Usage: %s", fullName)
	printFlags := getRegularFlags(info.Flags)
	if len(printFlags) > 0 {
		fmt.Fprintf(w, " [OPTIONS]")
	}
	if len(info.Subcommands) > 0 {
		fmt.Fprintf(w, " COMMAND")
	}
	if flag := getPosFlag(info.Flags); flag != nil {
		name := strings.ToUpper(flag.Name)
		for i := 0; i < flag.MinCount; i++ {
			fmt.Fprintf(w, " %s", name)
		}
		if flag.MaxCount == 0 || flag.MaxCount > flag.MinCount {
			fmt.Fprintf(w, " [%s...]", name)
		}
	}
	fmt.Fprintf(w, "\n")
	return nil
}

func formatFlags(w io.Writer, flags []*FlagInfo) error {
	// TODO: wrap final column to term width
	posFlag := getPosFlag(flags)
	printFlags := getRegularFlags(flags)
	if posFlag != nil {
		fmt.Fprintf(w, "\nPositional arguments:\n")
		fmt.Fprintf(w, "  %s", strings.ToUpper(posFlag.Name))
		if posFlag.Usage != "" {
			fmt.Fprintf(w, "  %s", posFlag.Usage)
		}
		fmt.Fprintf(w, "\n")
	}
	if len(printFlags) == 0 {
		return nil
	}
	fmt.Fprintf(w, "\nOptions:\n")
	tw := tabwriter.NewWriter(w, 0, 0, 1, ' ', 0)
	for _, flag := range printFlags {
		var name, ShortName string
		if flag.ShortName == "" {
			if len(flag.Name) == 1 {
				ShortName = fmt.Sprintf("-%s", flag.Name)
			} else {
				name = fmt.Sprintf("--%s", flag.Name)
			}
		} else {
			name = fmt.Sprintf("--%s", flag.Name)
			ShortName = fmt.Sprintf("-%s,", flag.ShortName)
		}
		fmt.Fprintf(tw, "  %s\t%s\t %s\n", ShortName, name, flag.Usage)
	}
	return tw.Flush()
}

func formatEnvVars(w io.Writer, flags []*FlagInfo) error {
	printFlags := make([]*FlagInfo, 0, len(flags))
	for _, flag := range flags {
		if !flag.Hidden && flag.EnvVar != "" {
			printFlags = append(printFlags, flag)
		}
	}
	if len(printFlags) == 0 {
		return nil
	}
	fmt.Fprintf(w, "\nEnvironment variables:\n")
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	for _, flagInfo := range printFlags {
		fmt.Fprintf(
			w,
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
	tw := tabwriter.NewWriter(w, 0, 0, 1, ' ', 0)
	for _, cmd := range subcommands {
		if cmd.Hidden {
			continue
		}
		fmt.Fprintf(tw, "  %s\t %s\n", cmd.Name, cmd.Usage)
	}
	return tw.Flush()
}
