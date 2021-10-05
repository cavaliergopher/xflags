package xflags

import (
	"os"
)

// TODO: position args must go last?
// TODO: fuzz

type argParser struct {
	args              []string
	cmd               *CommandInfo
	posFlag           *FlagInfo
	flagsByName       map[string]*FlagInfo
	subcommandsByName map[string]*CommandInfo
	flagsSeen         map[string]int
}

func newArgParser(cmd *CommandInfo, args []string) *argParser {
	c := &argParser{
		args:              normalize(args),
		flagsByName:       make(map[string]*FlagInfo),
		flagsSeen:         make(map[string]int),
		subcommandsByName: make(map[string]*CommandInfo),
	}
	c.setCommand(cmd)
	return c
}

func (c *argParser) setCommand(cmd *CommandInfo) {
	// accumulate flags
	c.cmd = cmd
	c.posFlag = nil
	for _, flag := range cmd.Flags {
		c.flagsByName[flag.Name] = flag
		if flag.ShortName != "" {
			c.flagsByName[flag.ShortName] = flag
		}
		if flag.Positional {
			c.posFlag = flag
		}
	}

	// reset subcommands
	c.subcommandsByName = make(map[string]*CommandInfo)
	for _, cmd := range cmd.Subcommands {
		c.subcommandsByName[cmd.Name] = cmd
	}
}

func (c *argParser) Parse() (*CommandInfo, error) {
	for {
		arg, ok := c.next()
		if !ok {
			break
		}
		if err := c.dispatch(arg); err != nil {
			return nil, err
		}
	}
	if err := c.parseEnvVars(); err != nil {
		return nil, err
	}
	if err := c.checkNArgs(); err != nil {
		return nil, err
	}
	return c.cmd, nil
}

func (c *argParser) parseEnvVars() error {
	for _, flagInfo := range c.flagsByName {
		if flagInfo.EnvVar == "" {
			continue
		}
		n := c.flagsSeen[flagInfo.Name]
		if n > 0 {
			continue
		}
		s, ok := os.LookupEnv(flagInfo.EnvVar)
		if !ok {
			continue
		}
		c.markSeen(flagInfo)
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
		n := c.flagsSeen[flag.Name]
		if flag.MinCount > 0 && n < flag.MinCount {
			return newArgError(1, "missing argument: %s", flag)
		}
		if flag.MaxCount > 0 && n > flag.MaxCount {
			return newArgError(1, "argument declared too many times: %s", flag)
		}
	}
	return nil
}

func (c *argParser) peek() (arg string, ok bool) {
	if len(c.args) == 0 {
		return
	}
	ok = true
	arg = c.args[0]
	return
}

func (c *argParser) next() (arg string, ok bool) {
	arg, ok = c.peek()
	if ok {
		c.args = c.args[1:]
	}
	return
}

func (c *argParser) markSeen(flagInfo *FlagInfo) {
	if _, ok := c.flagsSeen[flagInfo.Name]; !ok {
		flagInfo.Value.Reset()
	}
	c.flagsSeen[flagInfo.Name] += 1
}

func (c *argParser) dispatch(arg string) error {
	if isPositional(arg) {
		// handle position flag
		if c.posFlag != nil {
			c.markSeen(c.posFlag)
			return c.posFlag.Value.Set(arg)
		}
		if len(c.cmd.Subcommands) == 0 {
			return newArgError(1, "unexpected positional argument: %s", arg)
		}

		// handle subcommand
		cmd, ok := c.subcommandsByName[arg]
		if !ok {
			return newArgError(1, "unrecognized command: %s", arg)
		}
		c.setCommand(cmd)
		return nil
	}
	flagInfo, ok := c.flagsByName[flagName(arg)]
	if !ok {
		return newArgError(1, "unrecognized argument: %s", arg)
	}
	c.markSeen(flagInfo)

	// special case for booleans can be specified with a value or none at all
	if flagInfo.Boolean {
		value, ok := c.peek()
		if !ok {
			// no next arg
			return flagInfo.Value.Set("true")
		}
		// TODO: this is a bit broken
		if err := flagInfo.Value.Set(value); err != nil {
			// next arg doesn't appear to be true|false
			return flagInfo.Value.Set("true")
		}
		return nil
	}

	// read the next arg as a value
	value, ok := c.peek()
	if !ok || !isPositional(value) {
		return newArgError(1, "no value specified for flag: %s", arg)
	}
	c.next() // consume the value
	return flagInfo.Value.Set(value)
}

func flagName(arg string) string {
	if isSingleDash(arg) {
		return arg[1:]
	}
	if isDoubleDash(arg) {
		return arg[2:]
	}
	return arg
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
func normalize(args []string) []string {
	out := make([]string, 0, len(args))
	for _, arg := range args {
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
