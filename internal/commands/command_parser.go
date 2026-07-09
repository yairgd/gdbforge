package commands

import "strings"
import "errors"

var (
	ErrUnknownCommand       = errors.New("unknown command")
	ErrCommandNotExecutable = errors.New("command is not executable")
)

type CommandParser struct {
	registry *CommandRegistry

	current *CommandNode
	token   string

	path []*CommandNode
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
	p.path = p.path[:0]
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

func (p *CommandParser) Suggestions() []*CommandNode {
	list, _ := p.current.Complete(p.token)
	return list
}

func (p *CommandParser) Accept() error {
	list, ok := p.current.Complete(p.token)
	if !ok || len(list) != 1 {
		return ErrUnknownCommand
	}

	p.current = list[0]
	p.path = append(p.path, p.current)
	p.token = ""

	return nil
}

func (p *CommandParser) HasChildren() bool {
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

	p.current.Action(args...)
	return nil
}

func (p *CommandParser) Parse(line string) error {
	p.Reset()

	for _, token := range strings.Fields(line) {
		p.token = token

		if err := p.Accept(); err != nil {
			return err
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

	tokenStart := 0
	for i := 0; i < cursor; i++ {
		if runes[i] == ' ' || runes[i] == '\t' {
			p.token = string(runes[tokenStart:i])
			_ = p.Accept()
			tokenStart = i + 1
		}
	}

	p.token = string(runes[tokenStart:cursor])
}
