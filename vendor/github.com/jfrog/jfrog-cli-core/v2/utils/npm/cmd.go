package npmutils

import (
	"io"
	"os/exec"
	"regexp"
)

var npmCommands = regexp.MustCompile(`npm-install|npmi|npm-ci|npmci|npm-publish|npmp`)

func IsNpmCommand(cmd string) bool {
	return npmCommands.MatchString(cmd)
}

func (config *NpmConfig) GetCmd() *exec.Cmd {
	var cmd []string
	cmd = append(cmd, config.Npm)
	cmd = append(cmd, config.Command...)
	cmd = append(cmd, config.CommandFlags...)
	return exec.Command(cmd[0], cmd[1:]...)
}

func (config *NpmConfig) GetEnv() map[string]string {
	return map[string]string{}
}

func (config *NpmConfig) GetStdWriter() io.WriteCloser {
	return config.StrWriter
}

func (config *NpmConfig) GetErrWriter() io.WriteCloser {
	return config.ErrWriter
}

type NpmConfig struct {
	Npm          string
	Command      []string
	CommandFlags []string
	StrWriter    io.WriteCloser
	ErrWriter    io.WriteCloser
}
