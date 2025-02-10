package agentserver

import "os/exec"

type Command struct {
	Command string
	Args    []string
}

func (c Command) ToExec() *exec.Cmd {
	return exec.Command(c.Command, c.Args...)
}
