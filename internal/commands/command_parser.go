package commands

import (
	"errors"
	"strings"
)

var (
	ErrUnknownCommand       = errors.New("unknown command")
	ErrCommandNotExecutable = errors.New("command is not executable")
)

type CommandParser struct {
	registry *CommandRegistry

	current *CommandNode
	token   string
	args    []string

	path            []*CommandNode
	restTrailSpace  bool
}

func NewCommandParser(reg *CommandRegistry) *CommandParser {
	p := &CommandParser{
		registry: reg,
	}
	p.Reset()
	return p
}

func (p *CommandParser) Reset() {
	p.current = p.registry.Root
	p.token = ""
	p.args = p.args[:0]
	p.path = p.path[:0]
	p.restTrailSpace = false
}

func (p *CommandParser) AddRune(r rune) {
	p.token += string(r)
}

func (p *CommandParser) Backspace() {
	if len(p.token) == 0 {
		return
	}

	r := []rune(p.token)
	p.token = string(r[:len(r)-1])
}

func (p *CommandParser) Current() *CommandNode {
	return p.current
}

func (p *CommandParser) CurrentToken() string {
	return p.token
}

func (p *CommandParser) Path() []*CommandNode {
	return p.path
}

func (p *CommandParser) Args() []string {
	return p.args
}

func (p *CommandParser) Suggestions() []*CommandNode {
	if p.current != nil && p.current.RestArgs {
		return nil
	}
	list, _ := p.current.Complete(p.token)
	return list
}

// RestTrailingSpace reports whether Tab completion is after a trailing space in rest-args mode.
func (p *CommandParser) RestTrailingSpace() bool { return p != nil && p.restTrailSpace }

// SuggestionNames returns tab-completion names for the current token.
// Rest-args leaves use CompleteArgs when set; otherwise tree children.
func (p *CommandParser) SuggestionNames() []string {
	if p.current == nil {
		return nil
	}
	if p.current.RestArgs {
		if p.current.CompleteArgs == nil {
			return nil
		}
		return p.current.CompleteArgs(p.token, p.restTrailSpace)
	}
	list, _ := p.current.Complete(p.token)
	names := make([]string, len(list))
	for i, n := range list {
		names[i] = n.Name
	}
	return names
}

// CurrentIsRestArgs reports whether the parser is on a rest-args leaf.
func (p *CommandParser) CurrentIsRestArgs() bool {
	return p.current != nil && p.current.RestArgs
}

func (p *CommandParser) Accept() error {
	list, ok := p.current.Complete(p.token)
	if !ok || len(list) == 0 {
		return ErrUnknownCommand
	}
	if len(list) == 1 {
		p.current = list[0]
		p.path = append(p.path, p.current)
		p.token = ""
		return nil
	}
	// Prefix matched several leaves (e.g. next + nexti). Prefer an exact name.
	for _, n := range list {
		if n.Name == p.token {
			p.current = n
			p.path = append(p.path, p.current)
			p.token = ""
			return nil
		}
	}
	return ErrUnknownCommand
}

func (p *CommandParser) HasChildren() bool {
	if p.current != nil && p.current.RestArgs {
		return false
	}
	list, _ := p.current.Children.Complete("")
	return len(list) > 0
}

func (p *CommandParser) CanExecute() bool {
	return p.current.Action != nil
}

func (p *CommandParser) Execute(args ...any) error {
	if p.current.Action == nil {
		return ErrCommandNotExecutable
	}

	if len(args) == 0 && len(p.args) > 0 {
		anyArgs := make([]any, len(p.args))
		for i, a := range p.args {
			anyArgs[i] = a
		}
		p.current.Action(anyArgs...)
		return nil
	}

	p.current.Action(args...)
	return nil
}

func (p *CommandParser) Parse(line string) error {
	p.Reset()

	line = strings.TrimSpace(line)
	// Vim-style :!cmd — bang may be glued to the command (:!ls) or spaced (:! ls).
	if strings.HasPrefix(line, "!") {
		p.token = "!"
		if err := p.Accept(); err != nil {
			return err
		}
		if p.current.RestArgs {
			rest := strings.TrimSpace(line[1:])
			if rest != "" {
				p.args = strings.Fields(rest)
			}
			return nil
		}
	}

	tokens := strings.Fields(line)
	for i, token := range tokens {
		bang := false
		if len(token) > 1 && strings.HasSuffix(token, "!") {
			// Vim-style command bang (:q! / :quit!) — not :!shell.
			bang = true
			token = token[:len(token)-1]
		}
		p.token = token

		if err := p.Accept(); err != nil {
			return err
		}
		if bang {
			p.args = append(p.args, "!")
		}
		if p.current.RestArgs {
			p.args = append(p.args, tokens[i+1:]...)
			return nil
		}
	}

	return nil
}

// Sync replays input up to cursor so the parser reflects the token being edited.
func (p *CommandParser) Sync(line string, cursor int) {
	p.Reset()
	if cursor < 0 {
		cursor = 0
	}

	runes := []rune(line)
	if cursor > len(runes) {
		cursor = len(runes)
	}

	if len(runes) > 0 && runes[0] == '!' {
		p.token = "!"
		if err := p.Accept(); err == nil && p.current != nil && p.current.RestArgs {
			if cursor > 1 {
				p.token = string(runes[1:cursor])
			} else {
				p.token = ""
			}
			p.restTrailSpace = cursor < len(runes) && (runes[cursor] == ' ' || runes[cursor] == '\t')
			return
		}
		p.Reset()
	}

	tokenStart := 0
	for i := 0; i < cursor; i++ {
		if runes[i] == ' ' || runes[i] == '\t' {
			if p.current != nil && p.current.RestArgs {
				p.token = string(runes[tokenStart:cursor])
				p.restTrailSpace = cursor < len(runes) && (runes[cursor] == ' ' || runes[cursor] == '\t')
				return
			}
			p.token = stripCmdBang(string(runes[tokenStart:i]))
			_ = p.Accept()
			if p.current != nil && p.current.RestArgs {
				p.token = string(runes[i+1 : cursor])
				p.restTrailSpace = cursor < len(runes) && (runes[cursor] == ' ' || runes[cursor] == '\t')
				return
			}
			tokenStart = i + 1
		}
	}

	p.token = stripCmdBang(string(runes[tokenStart:cursor]))
}

// stripCmdBang removes a trailing vim-style bang from a command token (:q!).
func stripCmdBang(token string) string {
	if len(token) > 1 && strings.HasSuffix(token, "!") {
		return token[:len(token)-1]
	}
	return token
}
