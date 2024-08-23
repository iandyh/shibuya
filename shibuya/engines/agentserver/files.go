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
	AGENT_ROOT       = os.Getenv("AGENT_ROOT")
	RESULT_ROOT      = path.Join(AGENT_ROOT, "/test-result")
	TEST_DATA_FOLDER = "/test-data"
)

func removePreviousData(folder string) error {
	files, err := os.ReadDir(folder)
	if err != nil {
		return err
	}
	for _, file := range files {
		f := path.Join(folder, file.Name())
		if err := os.Remove(f); err != nil {
			return err
		}
	}
	return nil
}

func saveToDisk(folder, filename string, file []byte) error {
	filePath := filepath.Join(folder, filepath.Base(filename))
	if err := os.WriteFile(filePath, file, FILEMODE); err != nil {
		return err
	}
	return nil
}

func makeFullResultPath(filename string) string {
	return path.Join(RESULT_ROOT, filename)
}
