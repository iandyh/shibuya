package agentserver

import (
	"fmt"
	"log"
	"net/http"
	"os/exec"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func startDummyAgent(startCommand, stopCommand *exec.Cmd) (*AgentServer, error) {
	options := AgentServerOptions{
		StartCommand:   startCommand,
		StopCommand:    stopCommand,
		ResultFileFunc: findResultFile,
	}
	return StartAgentServer(options)
}

func makeStopCommand(command *exec.Cmd) *exec.Cmd {
	if err := command.Process.Kill(); err != nil {
		log.Println(err)
	}
	return exec.Command("echo", "/dev/null")
}

func makeUrl(path string) string {
	return fmt.Sprintf("http://localhost:8080/%s", path)
}

type testcase struct {
	name     string
	method   string
	path     string
	expected int
	after    func()
}

func TestSever(t *testing.T) {
	startCommand := exec.Command("sleep", "100")
	_, err := startDummyAgent(startCommand, nil)
	assert.Nil(t, err)

	cases := []testcase{
		{
			name:     "expected not in progress",
			path:     "progress",
			expected: http.StatusNotFound,
		},
		{
			name:     "valid start",
			method:   "POST",
			path:     "start",
			expected: http.StatusOK,
			after:    func() { time.Sleep(1 * time.Second) },
		},
		{
			name:     "in progress",
			method:   "GET",
			path:     "progress",
			expected: http.StatusOK,
		},
		{
			name:     "invalid stop",
			method:   "GET",
			path:     "stop",
			expected: http.StatusMethodNotAllowed,
		},
		{
			name:     "valid stop",
			method:   "POST",
			path:     "stop",
			expected: http.StatusOK,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			req, err := http.NewRequest(c.method, makeUrl(c.path), nil)
			assert.Nil(t, err)
			resp, err := http.DefaultClient.Do(req)
			assert.Nil(t, err)
			defer resp.Body.Close()
			assert.Equal(t, c.expected, resp.StatusCode)
			if c.after != nil {
				c.after()
			}
		})
	}
}
