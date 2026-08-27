package xflags

import (
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"github.com/cavaliergopher/xflags/desc"
)

// FormatFunc is a function that prints a help message for a described
// command.
type FormatFunc func(w io.Writer, cmd *desc.Command) error

// Format is the default FormatFunc to print help messages for a commands.
func Format(w io.Writer, cmd *desc.Command) error {
	if err := printUsage(w, cmd); err != nil {
		return err
	}
	if cmd.Summary != "" {
		if _, err := fmt.Fprintf(w, "\n%s\n", cmd.Summary); err != nil {
			return err
		}
	}
	if err := detailPositionals(w, cmd); err != nil {
		return err
	}
	for _, group := range cmd.FlagGroups {
		if err := detailFlagGroup(w, group); err != nil {
			return err
		}
	}
	if err := detailSubcommands(w, cmd.Subcommands); err != nil {
		return err
	}
	if err := detailEnvVars(w, cmd); err != nil {
		return err
	}
	if cmd.Description != "" {
		if _, err := fmt.Fprintf(w, "\n%s\n", cmd.Description); err != nil {
			return err
		}
	}
	return nil
}

func getPositionals(cmd *desc.Command) []*desc.Flag {
	a := make([]*desc.Flag, 0, 8)
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

func hasOptions(cmd *desc.Command) bool {
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
	return hasOptions(cmd.Parent)
}

// printUsage writes the usage line, which it assembles in full before
// writing so the whole line is one write with one error to report.
func printUsage(w io.Writer, cmd *desc.Command) error {
	fullName := cmd.Name
	for p := cmd.Parent; p != nil; p = p.Parent {
		fullName = fmt.Sprintf("%s %s", p.Name, fullName)
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Usage: %s", fullName)
	if hasOptions(cmd) {
		b.WriteString(" [OPTIONS]")
	}
	if len(cmd.Subcommands) > 0 {
		b.WriteString(" COMMAND")
	}
	for _, flag := range getPositionals(cmd) {
		name := strings.ToUpper(flag.Name)
		if flag.MinCount == 0 {
			if flag.MaxCount == 1 {
				fmt.Fprintf(&b, " [%s]", name)
			} else {
				fmt.Fprintf(&b, " [%s...]", name)
			}
		} else {
			if flag.MinCount == 1 && flag.MaxCount == 1 {
				fmt.Fprintf(&b, " %s", name)
			} else {
				fmt.Fprintf(&b, " %s...", name)
			}
		}
	}
	b.WriteString("\n")
	_, err := io.WriteString(w, b.String())
	return err
}

func detailPositionals(w io.Writer, cmd *desc.Command) error {
	flags := getPositionals(cmd)
	if len(flags) == 0 {
		return nil
	}
	if _, err := fmt.Fprintf(w, "\nPositional arguments:\n"); err != nil {
		return err
	}
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	for _, flag := range flags {
		var row strings.Builder
		fmt.Fprintf(&row, "  %s", strings.ToUpper(flag.Name))
		if flag.Usage != "" {
			fmt.Fprintf(&row, "\t%s", flag.Usage)
			if flag.ShowDefault {
				fmt.Fprintf(&row, " (default: %s)", flag.Default)
			}
		}
		row.WriteString("\n")
		if _, err := io.WriteString(tw, row.String()); err != nil {
			return err
		}
	}
	return tw.Flush()
}

func filterOptions(flags []*desc.Flag) []*desc.Flag {
	a := make([]*desc.Flag, 0, 8)
	for _, flag := range flags {
		if flag.Hidden || flag.Positional {
			continue
		}
		a = append(a, flag)
	}
	return a
}

func detailFlagGroup(w io.Writer, group *desc.FlagGroup) error {
	flags := filterOptions(group.Flags)
	if len(flags) == 0 {
		return nil
	}
	if _, err := fmt.Fprintf(w, "\n%s:\n", group.Title); err != nil {
		return err
	}
	tw := tabwriter.NewWriter(w, 0, 0, 1, ' ', 0)
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
		var row strings.Builder
		fmt.Fprintf(&row, "  %s\t%s\t %s", shortName, name, flag.Usage)
		if flag.ShowDefault {
			fmt.Fprintf(&row, " (default: %s)", flag.Default)
		}
		row.WriteString("\n")
		if _, err := io.WriteString(tw, row.String()); err != nil {
			return err
		}
	}
	return tw.Flush()
}

func getEnvVars(a []*desc.Flag, cmd *desc.Command) []*desc.Flag {
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

func detailEnvVars(w io.Writer, cmd *desc.Command) error {
	flags := getEnvVars(nil, cmd)
	if len(flags) == 0 {
		return nil
	}
	if _, err := fmt.Fprintf(w, "\nEnvironment variables:\n"); err != nil {
		return err
	}
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	for _, flag := range flags {
		_, err := fmt.Fprintf(
			tw,
			"  %s\t%s\n",
			strings.ToUpper(flag.EnvVar),
			flag.Usage,
		)
		if err != nil {
			return err
		}
	}
	return tw.Flush()
}

func detailSubcommands(w io.Writer, subcommands []*desc.Command) error {
	// TODO: wrap final column to terminal
	if len(subcommands) == 0 {
		return nil
	}
	if _, err := fmt.Fprintf(w, "\nCommands:\n"); err != nil {
		return err
	}
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	for _, cmd := range subcommands {
		if cmd.Hidden {
			continue
		}
		if _, err := fmt.Fprintf(tw, "  %s\t%s\n", cmd.Name, cmd.Summary); err != nil {
			return err
		}
	}
	return tw.Flush()
}
