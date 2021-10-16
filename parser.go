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
	cmd               *CommandInfo
	isTerminated      bool
	flagsByName       map[string]*FlagInfo
	subcommandsByName map[string]*CommandInfo
	flagsSeen         map[string]int
	positionals       []*FlagInfo
}

func newArgParser(cmd *CommandInfo, tokens []string) *argParser {
	tokens = normalize(tokens, cmd.WithTerminator)
	c := &argParser{
		tokens:            tokens,
		flagsByName:       make(map[string]*FlagInfo),
		flagsSeen:         make(map[string]int),
		subcommandsByName: make(map[string]*CommandInfo),
	}
	c.setCommand(cmd)
	return c
}

// setCommand descends the state machine into a new subcommand.
func (c *argParser) setCommand(cmd *CommandInfo) {
	// accumulate flags
	c.cmd = cmd
	c.positionals = make([]*FlagInfo, 0)
	for _, flag := range cmd.Flags {
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

	// reset subcommands
	c.subcommandsByName = make(map[string]*CommandInfo)
	for _, cmd := range cmd.Subcommands {
		c.subcommandsByName[cmd.Name] = cmd
	}
}

func (c *argParser) Parse() (cmd *CommandInfo, args []string, err error) {
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
	for _, flagInfo := range c.flagsByName {
		if flagInfo.EnvVar == "" {
			continue
		}
		n := c.flagsSeen[flagInfo.name()]
		if n > 0 {
			continue
		}
		s, ok := os.LookupEnv(flagInfo.EnvVar)
		if !ok {
			continue
		}
		c.observe(flagInfo)
		if err := flagInfo.Value.Set(s); err != nil {
			return err
		}
	}
	return nil
}

func (c *argParser) checkNArgs() error {
	if flagHelp {
		// don't check required flags if -h is specified
		return nil
	}
	for _, flag := range c.cmd.Flags {
		n := c.flagsSeen[flag.name()]
		if flag.MinCount > 0 && n < flag.MinCount {
			return errorf("missing argument: %s", flag)
		}
		if flag.MaxCount > 0 && n > flag.MaxCount {
			return errorf("argument declared too many times: %s", flag)
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

func (c *argParser) observe(flagInfo *FlagInfo) int {
	c.flagsSeen[flagInfo.name()] += 1
	return c.flagsSeen[flagInfo.name()]
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
	if isPositional(token) {
		return c.dispatchPositional(token)
	}
	return c.dispatchRegular(token)
}

func (c *argParser) dispatchPositional(token string) error {
	// handle positional flag
	if len(c.positionals) > 0 {
		flagInfo := c.positionals[0]
		n := c.observe(flagInfo)
		if flagInfo.MaxCount > 0 && n == flagInfo.MaxCount {
			// all done with this positional flag
			c.positionals = c.positionals[1:]
		}
		return flagInfo.Value.Set(token)
	}

	// handle subcommand
	if len(c.cmd.Subcommands) == 0 {
		return errorf("unexpected positional argument: %s", token)
	}
	cmd, ok := c.subcommandsByName[token]
	if !ok {
		return errorf("unrecognized command: %s", token)
	}
	c.setCommand(cmd)
	return nil
}

func (c *argParser) dispatchRegular(token string) error {
	// regular flag
	flagInfo := c.flagsByName[token]
	if flagInfo == nil {
		return errorf("unrecognized argument: %s", token)
	}
	c.observe(flagInfo)
	if isBoolValue(flagInfo.Value) {
		return flagInfo.Value.Set("true")
	}

	// read the next arg as a value
	value, ok := c.peek()
	if !ok || !isPositional(value) {
		return errorf("no value specified for flag: %s", token)
	}
	c.next() // consume the value
	return flagInfo.Value.Set(value)
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
