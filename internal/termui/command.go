package termui

type CommandID int

const CmdUnknown CommandID = 0

type Command struct {
	ID   CommandID
	Name string
}

type Commands struct {
	list   []Command
	byName map[string]Command
	byID   map[CommandID]Command
}

func NewCommands() *Commands {
	return &Commands{
		byName: make(map[string]Command),
		byID:   make(map[CommandID]Command),
	}
}

func (c *Commands) AddCommand(id CommandID, name string) {
	cmd := Command{ID: id, Name: name}
	c.list = append(c.list, cmd)
	c.byName[name] = cmd
	c.byID[id] = cmd
}

func (c *Commands) FindByName(name string) (Command, bool) {
	cmd, ok := c.byName[name]
	return cmd, ok
}

func (c *Commands) FindByID(id CommandID) (Command, bool) {
	cmd, ok := c.byID[id]
	return cmd, ok
}

func (c *Commands) List() []Command {
	return c.list
}
