package ir

import (
	"fmt"
	"io"
	"strings"
	"text/tabwriter"
)

// FormatFunc is a function that prints a help message for a compiled
// command.
type FormatFunc func(w io.Writer, cmd *Command) error

// writeUsage implements (*Command).WriteUsage.
func writeUsage(c *Command, w io.Writer) error {
	// TODO: Usage formatting is a function of the chosen argv vocabulary
	// (POSIX/GNU, Go, Windows, etc.) so we'll need to break this API.
	f := c.FormatFunc
	for p := c; f == nil && p != nil; p = p.Parent {
		f = p.FormatFunc
	}
	if f == nil {
		f = Format
	}
	return f(w, c)
}

// Format is the default FormatFunc to print help messages for a command.
func Format(w io.Writer, cmd *Command) error {
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

func hasOptions(cmd *Command) bool {
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
func printUsage(w io.Writer, cmd *Command) error {
	var b strings.Builder
	fmt.Fprintf(&b, "Usage: %s", cmd.FullName)
	if hasOptions(cmd) {
		b.WriteString(" [OPTIONS]")
	}
	if len(cmd.Subcommands) > 0 {
		b.WriteString(" COMMAND")
	}
	for _, flag := range getPositionals(cmd) {
		name := flag.String()
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

func detailPositionals(w io.Writer, cmd *Command) error {
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
		fmt.Fprintf(&row, "  %s", flag)
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

// isLongForm reports whether a rendered form belongs in the long-option
// column. It reads the spelling it was handed rather than deciding one:
// how a name is spelled is settled before a formatter ever sees it, and
// this is only where the result lines up.
func isLongForm(form string) bool {
	if len(form) < 3 {
		return false
	}
	return form[0] == '-' && form[1] == '-'
}

func filterOptions(flags []*Flag) []*Flag {
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
	flags := filterOptions(group.Flags)
	if len(flags) == 0 {
		return nil
	}
	if _, err := fmt.Fprintf(w, "\n%s:\n", group.Title); err != nil {
		return err
	}
	tw := tabwriter.NewWriter(w, 0, 0, 1, ' ', 0)
	for _, flag := range flags {
		// Only the first two forms print; anything after is an alias,
		// matched on the command line but deliberately undocumented.
		// Which column a form lands in follows its shape rather than its
		// slot, so a short spelling stays with the short ones however it
		// was declared.
		var name, shortName string
		for _, form := range flag.Forms[:min(2, len(flag.Forms))] {
			switch {
			case form == "":
			case isLongForm(form):
				if name == "" {
					name = form
				}
			default:
				if shortName == "" {
					shortName = form
				}
			}
		}
		if shortName != "" && name != "" {
			shortName += ","
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

func detailSubcommands(w io.Writer, subcommands []*Command) error {
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
