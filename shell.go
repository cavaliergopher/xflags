package xflags

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"unicode"

	"github.com/cavaliergopher/xflags/internal/argv"
	"github.com/cavaliergopher/xflags/ir"
)

// completionEnvVar returns the name of the environment variable Run
// consults for shell completion, derived from rootName as EnableCompletion
// documents: uppercased, with every rune that is not a letter or digit
// mapped to '_', followed by "_COMPLETE".
func completionEnvVar(rootName string) string {
	var b strings.Builder
	for _, r := range rootName {
		r = unicode.ToUpper(r)
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			r = '_'
		}
		b.WriteRune(r)
	}
	b.WriteString("_COMPLETE")
	return b.String()
}

// shellFuncName returns prog reduced to characters legal in a bash or zsh
// function name, for the completion scripts to name their function after
// the program without risking a syntax error on an unusual program name.
func shellFuncName(prog string) string {
	var b strings.Builder
	for _, r := range prog {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' {
			b.WriteRune(r)
		} else {
			b.WriteRune('_')
		}
	}
	return b.String()
}

// completionHook answers c's shell completion environment variable when it
// is set to a value Run recognizes, reporting handled as false when it is
// unset or holds anything else, so Run falls through to parsing args as
// usual. A tree that does not compile answers nothing and reports handled
// as false, so the fault is reported when Run goes on to compile it. See
// EnableCompletion and (*Command).Run.
func completionHook(c *Command) (code int, handled bool) {
	// Which variable to consult is a question about the tree, since it is
	// named from the root's name, so the tree compiles before the question
	// can be asked.
	//
	// TODO: this compiles the tree a second time on the path where no
	// completion is requested, which is every ordinary invocation of a
	// command that enabled it. It goes away once Run compiles once and
	// passes the compiled node down instead of each step compiling for
	// itself.
	node, err := c.Compile()
	if err != nil {
		return 0, false
	}
	rootName := node.Root.Name
	varName := completionEnvVar(rootName)
	val, ok := os.LookupEnv(varName)
	if !ok {
		return 0, false
	}

	switch val {
	case "bash_source":
		fmt.Fprint(node.Stdout, bashSourceScript(rootName, varName))
		return ExitCodeSuccess, true
	case "zsh_source":
		fmt.Fprint(node.Stdout, zshSourceScript(rootName, varName))
		return ExitCodeSuccess, true
	case "bash_complete", "zsh_complete":
		writeCompletionReply(node, node.Stdout)
		return ExitCodeSuccess, true
	default:
		return 0, false
	}
}

// writeCompletionReply answers one completion request read from
// COMP_WORDS and COMP_CWORD, in the protocol completionEnvVar's value
// selects.
//
// # Shell completion protocol
//
// COMP_WORDS holds the command line's words, one per line, starting with
// the program name; COMP_CWORD holds the index into COMP_WORDS of the word
// under the cursor, which may be empty and may be past the end of a line
// that has trailing whitespace. From these, args is the words strictly
// between the program name and the cursor, and word is the word at the
// cursor, or "" past the end of the line.
//
// The reply is written to w as newline-terminated lines, each one of:
//
//	plain,<candidate>   one candidate for the shell to offer
//	nofiles,            present exactly when the directive is
//	                    CompNoFileComp; its absence means CompDefault
//
// This always exits 0: a shell script evaluates the reply, and a nonzero
// exit or anything on stderr would surface as noise in the user's
// terminal rather than as a completion failure anyone can act on.
func writeCompletionReply(cmd *ir.Command, w io.Writer) {
	var words []string
	if s := os.Getenv("COMP_WORDS"); s != "" {
		words = strings.Split(s, "\n")
	}
	cword, _ := strconv.Atoi(os.Getenv("COMP_CWORD"))

	var args []string
	if cword > 0 && cword <= len(words) {
		args = words[1:cword]
	}
	var word string
	if cword >= 0 && cword < len(words) {
		word = words[cword]
	}

	cands, dir := argv.Complete(cmd, args, word)

	for _, cand := range cands {
		fmt.Fprintf(w, "plain,%s\n", cand)
	}
	if dir == ir.CompNoFileComp {
		fmt.Fprint(w, "nofiles,\n")
	}
}

// bashSourceScript returns the Bash completion script for prog, which
// reads envVar to tell the two ways it may be invoked apart: sourced, to
// print this script, or re-invoked by the script itself to answer one
// completion request.
func bashSourceScript(prog, envVar string) string {
	return fmt.Sprintf(bashCompletionScriptTmpl, prog, envVar, shellFuncName(prog))
}

// zshSourceScript is bashSourceScript's counterpart for Zsh.
func zshSourceScript(prog, envVar string) string {
	return fmt.Sprintf(zshCompletionScriptTmpl, prog, envVar, shellFuncName(prog))
}

// bashCompletionScriptTmpl is a fmt.Sprintf template taking, in order, the
// program name, the environment variable EnableCompletion derived for it,
// and a function-name-safe form of the program name.
const bashCompletionScriptTmpl = `# Bash completion for %[1]s. To enable it in the current shell, run:
#
#   source <(%[2]s=bash_source %[1]s 2>/dev/null)
#
# or add that line to your ~/.bashrc to enable it in every new shell.

_%[3]s_complete() {
	local response
	response=$(
		COMP_WORDS="$(printf '%%s\n' "${COMP_WORDS[@]}")" \
		COMP_CWORD="$COMP_CWORD" \
		%[2]s=bash_complete "${COMP_WORDS[0]}" 2>/dev/null
	)

	local nofiles=0
	COMPREPLY=()
	while IFS= read -r line; do
		case "$line" in
		plain,*)
			COMPREPLY+=("${line#plain,}")
			;;
		nofiles,*)
			nofiles=1
			;;
		esac
	done <<<"$response"

	if [ "$nofiles" -eq 0 ]; then
		COMPREPLY+=($(compgen -f -- "${COMP_WORDS[COMP_CWORD]}"))
	fi
}

complete -F _%[3]s_complete %[1]s
`

// zshCompletionScriptTmpl is bashCompletionScriptTmpl's counterpart for
// Zsh, taking the same three arguments in the same order.
const zshCompletionScriptTmpl = `#compdef %[1]s
# Zsh completion for %[1]s. To enable it in the current shell, run:
#
#   source <(%[2]s=zsh_source %[1]s 2>/dev/null)
#
# or add that line to your ~/.zshrc to enable it in every new shell.

_%[3]s_complete() {
	local -a plain
	local response nofiles=0

	response=$(
		COMP_WORDS="$(printf '%%s\n' "${words[@]}")" \
		COMP_CWORD=$((CURRENT - 1)) \
		%[2]s=zsh_complete "${words[1]}" 2>/dev/null
	)

	while IFS= read -r line; do
		case "$line" in
		plain,*)
			plain+=("${line#plain,}")
			;;
		nofiles,*)
			nofiles=1
			;;
		esac
	done <<<"$response"

	_describe -V '%[1]s completions' plain
	if [ "$nofiles" -eq 0 ]; then
		_files
	fi
}

compdef _%[3]s_complete %[1]s
`
