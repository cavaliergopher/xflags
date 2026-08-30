package argv

import (
	"strings"
	"unicode/utf8"
)

// FormOf returns how a flag name is spelled on the command line: one
// character takes a single dash and anything longer takes two, which is
// POSIX guideline 3 and the GNU long-option convention between them. The
// shape of the name decides, not the slot it was declared in.
//
// This is the one place a name becomes a command line spelling, which is
// what a second argv dialect would replace; see
// docs/adr/posix-gnu-argv-dialect.md.
func FormOf(name string) string {
	if name == "" {
		return ""
	}
	if utf8.RuneCountInString(name) == 1 {
		return "-" + name
	}
	return "--" + name
}

// FormsOf returns the spelling of each name given, parallel to them so
// that an empty slot stays empty.
func FormsOf(names []string) []string {
	if len(names) == 0 {
		return nil
	}
	forms := make([]string, len(names))
	for i, name := range names {
		forms[i] = FormOf(name)
	}
	return forms
}

// ValueNameOf returns how the name of a flag's value is written where the
// flag is shown to a reader: upper-cased, with dashes as underscores, so
// that "log-level" reads as LOG_LEVEL. That is how a POSIX synopsis
// writes an operand or an option-argument, and it is this dialect's
// answer rather than a universal one -- Go's own flag package writes a
// value's type in lower case, and other conventions bracket it as
// <value>.
func ValueNameOf(name string) string {
	return strings.ToUpper(strings.ReplaceAll(name, "-", "_"))
}

// The forms that ask for a command's help message. A command may not
// declare either name for itself -- validation reserves both -- so the
// lexer answers them before it consults the option table at all.
const (
	helpShortForm = "-h"
	helpLongForm  = "--help"
)

// shortForm returns how a one-character name is spelled on the command
// line. The lexer builds a key this way while reading a clustered
// argument, which is the one place a form is spelled from something other
// than a declared name.
func shortForm(r rune) string {
	return "-" + string(r)
}

func isShortOption(arg string) bool {
	if len(arg) < 2 {
		return false
	}
	return arg[0] == '-' && arg[1] != '-'
}

func isLongOption(arg string) bool {
	if len(arg) < 3 {
		return false
	}
	return arg[0] == '-' && arg[1] == '-'
}

// isOperand reports whether arg is an operand rather than an option.
// Guideline 14: if it parses as an option, it is one, so the syntax alone
// decides -- a token is an operand only when it looks like neither form.
func isOperand(arg string) bool {
	return !isShortOption(arg) && !isLongOption(arg)
}
