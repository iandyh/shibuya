package agentserver_test

import (
	"testing"

	"github.com/rakutentech/shibuya/shibuya/engines/agentserver"
)

func startDummyAgent(startCommand agentserver.Command, stopCommand *agentserver.Command) error {
	options := agentserver.AgentServerOptions{
		StartCommand: startCommand,
		StopCommand:  stopCommand,
		ResultFile:   "",
		EngineMeta: agentserver.EngineMeta{
			CoordinatorIP: "",
			CollectionID:  "",
			PlanID:        "",
			EngineID:      0,
			APIKey:        "",
		},
	}
	as := agentserver.MakeAgentServer(options)
	return as.Run()
}

type testcase struct {
	name     string
	method   string
	path     string
	expected int
	after    func()
}

// Current tests are pretty dummy because agent tests are difficult to setup as it relies on coordinator.
// S we are only testing the API now.
func TestSever(t *testing.T) {
	startCommand := agentserver.Command{
		Command: "sleep",
		Args:    []string{"100"},
	}

	go func() {
		startDummyAgent(startCommand, nil)
	}()
}
