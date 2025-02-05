package agentserver

import (
	"os"
	"path"
	"path/filepath"
)

const (
	FILEMODE = 0700
)

var (
	AGENT_ROOT  = os.Getenv("AGENT_ROOT")
	RESULT_ROOT = path.Join(AGENT_ROOT, "/test-result")
)

type TestFolder string

var (
	testFileFolder   = TestFolder("/test-data")
	testResultFolder = TestFolder(RESULT_ROOT)
)

func (tf TestFolder) reset() error {
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

func (tf TestFolder) saveFile(filename string, file []byte) error {
	filePath := filepath.Join(string(tf), filepath.Base(filename))
	if err := os.WriteFile(filePath, file, FILEMODE); err != nil {
		return err
	}
	return nil
}

func (tf TestFolder) resultFile(filename string) string {
	return path.Join(RESULT_ROOT, filename)
}
