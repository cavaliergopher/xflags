package xflags

import (
	"os"
)

// TODO: fuzz tests?

// argument to terminate parsing of all remaining arguments
const terminator = "--"

type argParser struct {
	tokens            []string
	args              []string
	cmd               *Command
	isTerminated      bool
	flagsByName       map[string]*Flag
	subcommandsByName map[string]*Command
	flagsSeen         map[string]int
	positionals       []*Flag
}

func newArgParser(cmd *Command, tokens []string) *argParser {
	tokens = normalize(tokens, cmd.WithTerminator)
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
	c.positionals = make([]*Flag, 0)
	for _, group := range cmd.FlagGroups {
		for _, flag := range group.Flags {
			if flag.Name != "" {
				c.flagsByName["--"+flag.Name] = flag
			}
			if flag.ShortName != "" {
				c.flagsByName["-"+flag.ShortName] = flag
			}
			if flag.Positional {
				c.positionals = append(c.positionals, flag)
			}
		}
	}

	// reset subcommands
	c.subcommandsByName = make(map[string]*Command)
	for _, cmd := range cmd.Subcommands {
		c.subcommandsByName[cmd.Name] = cmd
	}
}

func (c *argParser) Parse() (cmd *Command, args []string, err error) {
	for {
		arg, ok := c.next()
		if !ok {
			break
		}
		if err = c.dispatch(arg); err != nil {
			return
		}
	}
	if err = c.parseEnvVars(); err != nil {
		return
	}
	if err = c.checkNArgs(); err != nil {
		return
	}
	return c.cmd, c.args, nil
}

func (c *argParser) parseEnvVars() error {
	for _, flag := range c.flagsByName {
		if flag.EnvVar == "" {
			continue
		}
		n := c.flagsSeen[flag.name()]
		if n > 0 {
			continue
		}
		s, ok := os.LookupEnv(flag.EnvVar)
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
	for _, group := range c.cmd.FlagGroups {
		for _, flag := range group.Flags {
			n := c.flagsSeen[flag.name()]
			if flag.MinCount > 0 && n < flag.MinCount {
				return newArgErr(c.cmd, flag, "", "missing argument: %s", flag)
			}
			if flag.MaxCount > 0 && n > flag.MaxCount {
				return newArgErr(c.cmd, flag, "", "argument declared too many times: %s", flag)
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
	c.flagsSeen[flag.name()] += 1
	return c.flagsSeen[flag.name()]
}

func (c *argParser) dispatch(token string) error {
	if c.isTerminated {
		if c.args == nil {
			c.args = make([]string, 0, 1)
		}
		c.args = append(c.args, token)
		return nil
	}
	if token == terminator && c.cmd.WithTerminator {
		c.isTerminated = true
		return nil
	}
	if token == "-h" || token == "--help" {
		return &HelpError{Cmd: c.cmd}
	}
	if isPositional(token) {
		return c.dispatchPositional(token)
	}
	return c.dispatchRegular(token)
}

func (c *argParser) dispatchPositional(token string) error {
	// handle positional flag
	if len(c.positionals) > 0 {
		flag := c.positionals[0]
		n := c.observe(flag)
		if flag.MaxCount > 0 && n == flag.MaxCount {
			// all done with this positional flag
			c.positionals = c.positionals[1:]
		}
		return c.setFlag(flag, token)
	}

	// handle subcommand
	if len(c.cmd.Subcommands) == 0 {
		return newArgErr(c.cmd, nil, token, "unexpected positional argument: %s", token)
	}
	cmd, ok := c.subcommandsByName[token]
	if !ok {
		return newArgErr(c.cmd, nil, token, "unrecognized command: %s", token)
	}
	c.setCommand(cmd)
	return nil
}

func (c *argParser) dispatchRegular(token string) error {
	// regular flag
	flag := c.flagsByName[token]
	if flag == nil {
		return newArgErr(c.cmd, nil, token, "unrecognized argument: %s", token)
	}
	c.observe(flag)
	if isBoolValue(flag.Value) {
		return c.setFlag(flag, "true")
	}

	// read the next arg as a value
	value, ok := c.peek()
	if !ok || !isPositional(value) {
		return newArgErr(c.cmd, flag, token, "no value specified for flag: %s", token)
	}
	c.next() // consume the value
	return c.setFlag(flag, value)
}

func (c *argParser) setFlag(flag *Flag, value string) error {
	if err := flag.Set(value); err != nil {
		return wrapArgErr(err, c.cmd, flag, value)
	}
	return nil
}

func isSingleDash(arg string) bool {
	if len(arg) < 2 {
		return false
	}
	return arg[0] == '-' && arg[1] != '-'
}

func isDoubleDash(arg string) bool {
	if len(arg) < 3 {
		return false
	}
	return arg[0] == '-' && arg[1] == '-'
}

func isPositional(arg string) bool {
	return !isSingleDash(arg) && !isDoubleDash(arg)
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
		if isSingleDash(arg) {
			out = append(out, arg[:2])
			arg = arg[2:]
			if len(arg) > 0 {
				if arg[0] == '=' {
					arg = arg[1:]
				}
			} else {
				continue
			}
		} else if isDoubleDash(arg) {
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
