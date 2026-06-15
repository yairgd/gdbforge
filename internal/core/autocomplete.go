package core

import "strings"

type AutoCompleter interface {
	Complete(prefix string) []string
}

type SimpleCompleter struct {
	commands []string
}

func NewSimpleCompleter(cmds []string) *SimpleCompleter {
	return &SimpleCompleter{commands: cmds}
}

func (c *SimpleCompleter) AddCommand(name string) {
	c.commands = append(c.commands, name)
}

func (c *SimpleCompleter) Complete(prefix string) []string {
	var matches []string
	for _, cmd := range c.commands {
		if strings.HasPrefix(cmd, prefix) {
			matches = append(matches, cmd)
		}
	}
	return matches
}
