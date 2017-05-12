package go_concourse_helper

import "encoding/json"

type Command struct {
	messager *Messager
	request  Request
}

func NewCommand(messager *Messager) *Command {
	command := &Command{
		messager: messager,
	}
	command.load()
	return command
}
func (c Command) Messager() *Messager {
	return c.messager
}
func (c Command) Version() Version {
	return c.request.Version
}
func (c Command) Source(v interface{}) error {
	b, _ := json.Marshal(c.request.Source)
	return json.Unmarshal(b, v)
}
func (c Command) Params(v interface{}) error {
	b, _ := json.Marshal(c.request.Params)
	return json.Unmarshal(b, v)
}
func (c *Command) load() {
	err := c.messager.RetrieveJsonRequest(&c.request)
	if err != nil {
		c.messager.Fatal("Error when parsing object given by concourse: " + err.Error())
	}
}
func (c Command) Send(metadata []Metadata) {
	c.messager.SendJsonResponse(Response{
		Metadata: metadata,
		Version:  c.request.Version,
	})
}
