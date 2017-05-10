package go_concourse_helper

type OutCommand struct {
	*Command
}

func NewOutCommand() *OutCommand {
	return NewOutCommandWithMessager(NewMessager())
}
func NewOutCommandWithMessager(messager *Messager) *OutCommand {
	cmd := NewCommand(messager)
	return &OutCommand{cmd}
}

func (c *OutCommand) SourceFolder() string {
	return c.messager.Directory
}
