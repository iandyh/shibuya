package storage_test

import (
	"encoding/csv"
	"os"
	"strings"
	"testing"

	"github.com/rakutentech/shibuya/shibuya/coordinator/storage"
	enginesModel "github.com/rakutentech/shibuya/shibuya/engines/model"
	"github.com/rakutentech/shibuya/shibuya/model"
	"github.com/stretchr/testify/assert"
)

const (
	collectionID = "1"
	planID       = "1"
	testFilename = "shibuya-test-file.jmx"
	dataFilename = "shibuya-test-data.csv"
)

func TestFilepath(t *testing.T) {
	pf := storage.NewPlanFiles("/tmp", collectionID, planID)
	path := pf.TestFilePath(testFilename)
	assert.True(t, strings.HasPrefix(path, "/"))
}

func TestStoreTestPlan(t *testing.T) {
	pf := storage.NewPlanFiles("/tmp", collectionID, planID)
	err := pf.StoreTestPlan(testFilename, []byte("hello"))
	assert.Nil(t, err)
}

func prepareDataConfig(engineNum, totalSplits int) []*enginesModel.EngineDataConfig {
	splitRequired := false
	if engineNum == totalSplits {
		splitRequired = true
	}
	dataConfig := make([]*enginesModel.EngineDataConfig, engineNum)
	for i := 0; i < engineNum; i++ {
		currentSplit := 0
		if splitRequired {
			currentSplit = i
		}
		dataConfig[i] = &enginesModel.EngineDataConfig{
			EngineData: map[string]*model.ShibuyaFile{
				dataFilename: {
					Filename:     dataFilename,
					TotalSplits:  totalSplits,
					CurrentSplit: currentSplit,
				},
			},
		}
	}
	return dataConfig
}

func TestStoreDataFile(t *testing.T) {
	engineNum := 2
	fileBytes := []byte("a\nb\n")
	testcases := []struct {
		name         string
		totalSplits  int
		expectedRows int
	}{
		{
			name:         "split",
			totalSplits:  2,
			expectedRows: 1,
		},
		{
			name:         "no split",
			totalSplits:  1,
			expectedRows: 2,
		},
	}
	for _, c := range testcases {
		t.Run(c.name, func(t *testing.T) {
			dataConfig := prepareDataConfig(engineNum, c.totalSplits)
			pf := storage.NewPlanFiles("/tmp", collectionID, planID)
			err := pf.StoreDataFile(dataFilename, fileBytes, dataConfig)
			assert.Nil(t, err)

			for i := 0; i < engineNum; i++ {
				edp := pf.EngineDataPath(dataFilename, i)
				file, err := os.Open(edp)
				assert.Nil(t, err)
				defer file.Close()

				reader := csv.NewReader(file)
				records, err := reader.ReadAll()
				assert.Nil(t, err)
				assert.Equal(t, c.expectedRows, len(records))
			}
		})
	}

}
