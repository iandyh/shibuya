package agentserver

import (
	"os"
	"path"
	"path/filepath"
)

const (
	FILEMODE = 0700
)

type (
	AgentDir struct {
		dir string
	}
	AgentDirectory       string
	TestFilesDirectory   string
	ResultFilesDirectory string
	ConfFilesDirectory   string
)

func NewAgentDirHandler(dir string) AgentDir {
	if dir == "" {
		dir = os.Getenv("AGENT_ROOT")
	}
	return AgentDir{dir: dir}
}

func (af AgentDir) Dir() AgentDirectory {
	return AgentDirectory(af.dir)
}

func (af AgentDir) TestFilesDir() TestFilesDirectory {
	return TestFilesDirectory(path.Join("", "/test-data"))
}

func (af AgentDir) ConfFilesDir() ConfFilesDirectory {
	return ConfFilesDirectory(path.Join(af.dir, "test-conf"))
}

func (af AgentDir) ResultFilesDir() ResultFilesDirectory {
	return ResultFilesDirectory(path.Join(af.dir, "test-result"))
}

func (tf TestFilesDirectory) reset() error {
	files, err := os.ReadDir(string(tf))
	if err != nil {
		return err
	}
	for _, file := range files {
		f := path.Join(string(tf), file.Name())
		if err := os.Remove(f); err != nil {
			return err
		}
	}
	return nil
}

func (tf TestFilesDirectory) saveFile(filename string, file []byte) error {
	filePath := filepath.Join(string(tf), filepath.Base(filename))
	if err := os.WriteFile(filePath, file, FILEMODE); err != nil {
		return err
	}
	return nil
}

func join(parent string, subdirs ...string) string {
	full := make([]string, len(subdirs)+1)
	full[0] = parent
	for i, item := range subdirs {
		full[i+1] = item
	}
	return path.Join(full...)
}

func (af AgentDirectory) Filepath(sub ...string) string {
	return join(string(af), sub...)
}

func (cf ConfFilesDirectory) Filepath(sub ...string) string {
	return join(string(cf), sub...)
}

func (tf TestFilesDirectory) Filepath(sub ...string) string {
	return join(string(tf), sub...)
}

func (rf ResultFilesDirectory) resultFile(sub ...string) string {
	return join(string(rf), sub...)
}
