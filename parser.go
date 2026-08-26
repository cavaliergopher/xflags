package xflags

import (
	"os"
	"unicode/utf8"
)

// argument to terminate parsing of all remaining arguments
const terminator = "--"

type argParser struct {
	tokens            []string
	args              []string
	cmd               *Command
	path              []string
	forwarding        bool
	optionsEnded      bool
	helpRequested     bool
	flagsByName       map[string]*Flag
	subcommandsByName map[string]*Command
	flagsSeen         map[string]int
	positionals       []*Flag
}

func newArgParser(cmd *Command, tokens []string) *argParser {
	c := &argParser{
		tokens:            tokens,
		flagsByName:       make(map[string]*Flag),
		flagsSeen:         make(map[string]int),
		subcommandsByName: make(map[string]*Command),
	}
	c.setCommand(cmd)
	return c
}

// setCommand descends the parser into a new subcommand.
func (c *argParser) setCommand(cmd *Command) {
	// accumulate flags
	c.cmd = cmd
	c.path = append(c.path, cmd.name)
	c.positionals = make([]*Flag, 0)
	for _, group := range cmd.flagGroups {
		for _, flag := range group.flags {
			if flag.name != "" {
				c.flagsByName["--"+flag.name] = flag
			}
			if flag.shortName != "" {
				c.flagsByName["-"+flag.shortName] = flag
			}
			if flag.positional {
				c.positionals = append(c.positionals, flag)
			}
		}
	}

	// reset subcommands
	c.subcommandsByName = make(map[string]*Command)
	for _, cmd := range cmd.subcommands {
		c.subcommandsByName[cmd.name] = cmd
	}
}

func (c *argParser) Parse() (*Invocation, error) {
	for {
		arg, ok := c.next()
		if !ok {
			break
		}
		if err := c.parseOne(arg); err != nil {
			return nil, err
		}
		if c.helpRequested {
			// Asking for help abandons the rest of the command line. The
			// flag rules go unchecked so that help is still reported for a
			// command line the user has not finished writing.
			return c.invocation(), nil
		}
	}
	if err := c.parseEnvVars(); err != nil {
		return nil, err
	}
	if err := c.checkNArgs(); err != nil {
		return nil, err
	}
	return c.invocation(), nil
}

// invocation returns the Invocation describing the command line parsed so
// far, naming the deepest command the parser descended into.
func (c *argParser) invocation() *Invocation {
	return &Invocation{
		Cmd:           c.cmd,
		Path:          c.path,
		Forwarded:     c.args,
		HelpRequested: c.helpRequested,
		Stdin:         c.cmd.getStdin(),
		Stdout:        c.cmd.getStdout(),
		Stderr:        c.cmd.getStderr(),
	}
}

func (c *argParser) parseEnvVars() error {
	for _, flag := range c.flagsByName {
		if flag.envVar == "" {
			continue
		}
		n := c.flagsSeen[flag.keyName()]
		if n > 0 {
			continue
		}
		s, ok := os.LookupEnv(flag.envVar)
		if !ok {
			continue
		}
		c.observe(flag)
		if err := c.setFlag(flag, s); err != nil {
			return err
		}
	}
	return nil
}

// checkNArgs verifies each flag was given as many times as it requires.
//
// The offending flag goes at the end of these messages, which is where
// Go's flag package, argparse and getopt all put it when nothing follows
// it. A flag leads the message only when a wrapped error follows, as in
// "--ip: invalid IP: 256.0.0.1", where it scopes what comes after the
// colon; see docs/adr/human-readable-errors.md.
func (c *argParser) checkNArgs() error {
	for _, group := range c.cmd.flagGroups {
		for _, flag := range group.flags {
			n := c.flagsSeen[flag.keyName()]
			if flag.minCount > 0 && n < flag.minCount {
				switch {
				case flag.minCount == 1:
					return newArgumentErrorf(nil, c.cmd, flag, "",
						"missing required argument: %s", flag)
				case flag.minCount == flag.maxCount:
					return newArgumentErrorf(nil, c.cmd, flag, "",
						"expected %d arguments, got %d: %s",
						flag.minCount, n, flag)
				default:
					return newArgumentErrorf(nil, c.cmd, flag, "",
						"expected at least %d arguments, got %d: %s",
						flag.minCount, n, flag)
				}
			}
			if flag.maxCount > 0 && n > flag.maxCount {
				return newArgumentErrorf(nil, c.cmd, flag, "",
					"argument specified too many times: %s", flag)
			}
		}
	}
	return nil
}

func (c *argParser) peek() (token string, ok bool) {
	if len(c.tokens) == 0 {
		return
	}
	ok = true
	token = c.tokens[0]
	return
}

func (c *argParser) next() (token string, ok bool) {
	token, ok = c.peek()
	if ok {
		c.tokens = c.tokens[1:]
	}
	return
}

func (c *argParser) observe(flag *Flag) int {
	c.flagsSeen[flag.keyName()] += 1
	return c.flagsSeen[flag.keyName()]
}

// parseOne parses one token from the command line: an argument being
// forwarded past the terminator, the terminator itself, a help request,
// an operand, or an option.
//
// Guideline 10 gives "--" two readings and a command picks one. By default
// it ends option processing, so every argument after it is an operand
// however many dashes it starts with -- the escape hatch that lets a
// command take an operand named "-rf". A command that set ForwardArgs
// instead hands everything after it to the handler unparsed.
//
// Only the first "--" is special. Once options have ended, a later one is
// an ordinary operand, as is a "-h" that would otherwise ask for help.
func (c *argParser) parseOne(token string) error {
	if c.forwarding {
		if c.args == nil {
			c.args = make([]string, 0, 1)
		}
		c.args = append(c.args, token)
		return nil
	}
	if !c.optionsEnded {
		if token == terminator {
			if c.cmd.forwardArgs {
				c.forwarding = true
			} else {
				c.optionsEnded = true
			}
			return nil
		}
		if token == "-h" || token == "--help" {
			c.helpRequested = true
			return nil
		}
		if !isOperand(token) {
			return c.parseOption(token)
		}
	}
	return c.parseOperand(token)
}

// parseOperand binds an operand to the next positional flag awaiting
// one, or, when the command takes none, resolves it as a subcommand name.
func (c *argParser) parseOperand(token string) error {
	if len(c.positionals) > 0 {
		flag := c.positionals[0]
		n := c.observe(flag)
		if flag.maxCount > 0 && n == flag.maxCount {
			// all done with this positional flag
			c.positionals = c.positionals[1:]
		}
		return c.setFlag(flag, token)
	}

	// handle subcommand
	if len(c.cmd.subcommands) == 0 {
		return newArgumentErrorf(nil, c.cmd, nil, token, "unexpected positional argument: %s", token)
	}
	cmd, ok := c.subcommandsByName[token]
	if !ok {
		return newArgumentErrorf(nil, c.cmd, nil, token, "unrecognized subcommand: %s", token)
	}
	c.setCommand(cmd)
	return nil
}

// parseOption binds an option to its option-argument. The two forms are
// spelled differently enough to be worth separate readers.
func (c *argParser) parseOption(token string) error {
	if isShortOption(token) {
		return c.parseShortOptions(token)
	}
	return c.parseLongOption(token)
}

// parseLongOption binds one long option to its option-argument: one
// attached with "=", the next argument for a flag that takes a value, or
// an implicit "true" for a boolean.
func (c *argParser) parseLongOption(token string) error {
	name, value, attached := splitLongOption(token)
	flag := c.flagsByName[name]
	if flag == nil {
		return newArgumentErrorf(nil, c.cmd, nil, name, "unrecognized option: %s", name)
	}
	c.observe(flag)

	// An attached value is unambiguous by construction, so it binds
	// whatever it looks like and whatever the flag's type. A boolean takes
	// one here and nowhere else, which is how it is set false.
	if attached {
		return c.setFlag(flag, value)
	}
	if isBoolValue(flag.value) {
		return c.setFlag(flag, "true")
	}
	return c.parseDetachedValue(flag, name)
}

// parseShortOptions resolves one argument's worth of short options.
// Guideline 5: consumption continues while each name is a flag that takes
// no value, and the first that takes one takes the whole remainder of the
// argument as its attached value, so -abfx is -a -b -f x. This is why the
// argument cannot be split before the flag table is known.
func (c *argParser) parseShortOptions(arg string) error {
	for i, r := range arg {
		if i == 0 {
			continue // the delimiter
		}
		name := "-" + string(r)
		flag := c.flagsByName[name]
		if flag == nil {
			return newArgumentErrorf(nil, c.cmd, nil, name, "unrecognized option: %s", name)
		}
		c.observe(flag)
		rest := arg[i+utf8.RuneLen(r):]

		if isBoolValue(flag.value) {
			// Guideline 5 spends the whole remainder on further names,
			// which would leave a boolean no short spelling for false. A
			// short name can never be "=", so reading one as a delimiter
			// costs no ambiguity; see
			// docs/adr/posix-argument-conventions.md.
			if len(rest) > 0 && rest[0] == '=' {
				return c.setFlag(flag, rest[1:])
			}
			if err := c.setFlag(flag, "true"); err != nil {
				return err
			}
			continue
		}

		// This flag takes a value, so the rest of the argument is it and
		// no further name is read. A leading "=" is again the delimiter
		// rather than the value.
		if rest == "" {
			return c.parseDetachedValue(flag, name)
		}
		if rest[0] == '=' {
			rest = rest[1:]
		}
		return c.setFlag(flag, rest)
	}
	return nil
}

// parseDetachedValue binds the next argument to flag as its value.
// Guideline 14: an argument that parses as an option is one, so a detached
// value may never begin with "-". A value that must, such as -5, is given
// attached instead.
func (c *argParser) parseDetachedValue(flag *Flag, name string) error {
	next, ok := c.peek()
	if !ok || !isOperand(next) {
		return newArgumentErrorf(nil, c.cmd, flag, name,
			"option requires an argument: %s", name)
	}
	c.next() // consume the value
	return c.setFlag(flag, next)
}

// Set the value of flag, return any validation error.
func (c *argParser) setFlag(flag *Flag, token string) error {
	if err := flag.Set(token); err != nil {
		return newArgumentErrorf(err, c.cmd, flag, token, "%s", flag)
	}
	return nil
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

// splitLongOption divides a long option argument into its name and any
// option-argument attached to it, reporting whether one was attached at
// all. Guideline 6's attached form for a long option is "--name=value",
// which splits at the first "=".
//
// arg must be a long option; isLongOption decides that.
func splitLongOption(arg string) (name, value string, attached bool) {
	// From index 3, because a long name is at least one character wide and
	// "--=value" therefore names nothing.
	for i := 3; i < len(arg); i++ {
		if arg[i] == '=' {
			return arg[:i], arg[i+1:], true
		}
	}
	return arg, "", false
}
