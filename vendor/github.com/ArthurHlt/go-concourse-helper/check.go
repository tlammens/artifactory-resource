package go_concourse_helper

import "errors"

type CheckCommand struct {
	*Command
}

func NewCheckCommand() *CheckCommand {
	return NewCheckCommandWithMessager(NewMessager())
}
func NewCheckCommandWithMessager(messager *Messager) *CheckCommand {
	cmd := NewCommand(messager)
	return &CheckCommand{cmd}
}
func (c CheckCommand) Send(versions []Version) {
	c.messager.SendJsonResponse(versions)
}
func (c CheckCommand) Params(v interface{}) error {
	return errors.New("No params in check command")
}
