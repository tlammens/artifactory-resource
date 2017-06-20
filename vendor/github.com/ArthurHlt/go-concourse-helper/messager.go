package go_concourse_helper

import (
	"encoding/json"
	"fmt"
	"github.com/mitchellh/colorstring"
	"io"
	"os"
)

type Messager struct {
	LogWriter      io.Writer
	ResponseWriter io.Writer
	RequestReader  io.Reader
	ExitOnFatal    bool
	Directory      string
}

func NewMessager() *Messager {
	directory := ""
	if len(os.Args) >= 2 {
		directory = os.Args[1]
	}
	return &Messager{
		LogWriter:      os.Stderr,
		ResponseWriter: os.Stdout,
		RequestReader:  os.Stdin,
		ExitOnFatal:    true,
		Directory:      directory,
	}
}

func (rl *Messager) Log(message string, args ...interface{}) {
	message = colorstring.Color(message)
	if len(args) > 0 {
		fmt.Fprintf(rl.LogWriter, message, args...)
	} else {
		fmt.Fprint(rl.LogWriter, message)
	}
}
func (rl *Messager) Logln(message string, args ...interface{}) {
	message = message + "\n"
	rl.Log(message, args...)
}
func (rl *Messager) RetrieveJsonRequest(v interface{}) error {
	return json.NewDecoder(rl.RequestReader).Decode(v)
}
func (rl *Messager) SendJsonResponse(v interface{}) {
	json.NewEncoder(rl.ResponseWriter).Encode(v)
}
func (rl *Messager) GetLogWriter() io.Writer {
	return rl.LogWriter
}
func (rl *Messager) GetResponseWriter() io.Writer {
	return rl.ResponseWriter
}
func (rl *Messager) FatalIf(doing string, err error) {
	if err != nil {
		rl.Fatal(doing + ": " + err.Error())
	}
}

func (rl *Messager) Fatal(message string) {
	fmt.Fprintln(rl.LogWriter, colorstring.Color("[red]"+ message +"[reset]"))
	fmt.Fprintln(rl.ResponseWriter, colorstring.Color(message))
	if rl.ExitOnFatal {
		os.Exit(1)
	}
}
