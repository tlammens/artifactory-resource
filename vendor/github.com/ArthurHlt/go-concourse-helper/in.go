package go_concourse_helper

type InCommand struct {
	*Command
}

func NewInCommand() *InCommand {
	return NewInCommandWithMessager(NewMessager())
}
func NewInCommandWithMessager(messager *Messager) *InCommand {
	cmd := NewCommand(messager)
	return &InCommand{cmd}
}
func (c InCommand) DestinationFolder() string {
	return c.messager.Directory
}
