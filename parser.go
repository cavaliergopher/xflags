package xflags

import "os"

// TODO: fuzz tests?

// argument to terminate parsing of all remaining arguments
const terminator = "--"

type argParser struct {
	tokens            []string
	args              []string
	cmd               *Command
	path              []string
	isTerminated      bool
	helpRequested     bool
	flagsByName       map[string]*Flag
	subcommandsByName map[string]*Command
	flagsSeen         map[string]int
	positionals       []*Flag
}

func newArgParser(cmd *Command, tokens []string) *argParser {
	tokens = normalize(tokens, cmd.withTerminator)
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
		Args:          c.args,
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

func (c *argParser) checkNArgs() error {
	for _, group := range c.cmd.flagGroups {
		for _, flag := range group.flags {
			n := c.flagsSeen[flag.keyName()]
			if flag.minCount > 0 && n < flag.minCount {
				return newArgumentErrorf(nil, c.cmd, flag, "", "missing argument")
			}
			if flag.maxCount > 0 && n > flag.maxCount {
				return newArgumentErrorf(nil, c.cmd, flag, "", "argument specified too many times")
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
func (c *argParser) parseOne(token string) error {
	if c.isTerminated {
		if c.args == nil {
			c.args = make([]string, 0, 1)
		}
		c.args = append(c.args, token)
		return nil
	}
	if token == terminator && c.cmd.withTerminator {
		c.isTerminated = true
		return nil
	}
	if token == "-h" || token == "--help" {
		c.helpRequested = true
		return nil
	}
	if isOperand(token) {
		return c.parseOperand(token)
	}
	return c.parseOption(token)
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

// parseOption binds an option to its option-argument: the next token
// for a flag that takes a value, or an implicit "true" for a boolean.
func (c *argParser) parseOption(token string) error {
	flag := c.flagsByName[token]
	if flag == nil {
		return newArgumentErrorf(nil, c.cmd, nil, token, "unrecognized argument: %s", token)
	}
	c.observe(flag)
	if isBoolValue(flag.value) {
		return c.setFlag(flag, "true")
	}

	// read the next arg as a value
	value, ok := c.peek()
	if !ok || !isOperand(value) {
		return newArgumentErrorf(nil, c.cmd, flag, token, "option requires an argument")
	}
	c.next() // consume the value
	return c.setFlag(flag, value)
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

// normalize splits any arguments that declare both a key and a value (E.g.
// --key=value, or -kV) into two distinct arguments.
func normalize(args []string, withTerminator bool) []string {
	out := make([]string, 0, len(args))
	for i, arg := range args {
		if withTerminator && arg == terminator {
			out = append(out, args[i:]...)
			return out
		}
		if isShortOption(arg) {
			out = append(out, arg[:2])
			arg = arg[2:]
			if len(arg) > 0 {
				if arg[0] == '=' {
					arg = arg[1:]
				}
			} else {
				continue
			}
		} else if isLongOption(arg) {
			for i := 3; i < len(arg); i++ {
				if arg[i] == '=' {
					out = append(out, arg[:i])
					arg = arg[i+1:]
					break
				}
			}
		}
		out = append(out, arg)
	}
	return out
}
