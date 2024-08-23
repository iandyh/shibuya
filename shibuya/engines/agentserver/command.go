package agentserver

import "os/exec"

type Command struct {
	Command string
	Args    []string
}

func (c Command) ToExec(extraArgs []string) *exec.Cmd {
	for _, ea := range extraArgs {
		c.Args = append(c.Args, ea)
	}
	return exec.Command(c.Command, c.Args...)
}
