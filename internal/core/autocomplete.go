package core

import "strings"

type AutoCompleter interface {
	Complete(prefix string) []Command
}

type SimpleCompleter struct {
	commands []Command
}

func NewSimpleCompleter(cmds []Command) *SimpleCompleter {
	return &SimpleCompleter{
		commands: cmds,
	}
}

func (c *SimpleCompleter) AddCommand(cmd Command) {
	c.commands = append(c.commands, cmd)
}

func (c *SimpleCompleter) Complete(prefix string) []Command {

	var matches []Command

	for _, cmd := range c.commands {

		if strings.HasPrefix(cmd.Name, prefix) {
			matches = append(matches, cmd)
		}
	}

	return matches
}
