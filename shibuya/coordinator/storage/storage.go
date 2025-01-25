package storage

import (
	"os"
	"path/filepath"
	"strconv"

	enginesModel "github.com/rakutentech/shibuya/shibuya/engines/model"
	"github.com/rakutentech/shibuya/shibuya/utils"
)

const (
	DirRoot  = "/coordinator/files"
	filemode = 0700 // filemode is set to private as it should only accessed by coordinator
)

type PlanFiles struct {
	CollectionID string
	PlanID       string
	dirname      string
	rootDir      string
}

func NewPlanFiles(rootDir, collectionID, planID string) *PlanFiles {
	pf := &PlanFiles{CollectionID: collectionID, PlanID: planID}
	pf.rootDir = DirRoot
	if rootDir != "" {
		pf.rootDir = rootDir
	}
	pf.dirname = pf.makeDirName()
	return pf
}

func (pf *PlanFiles) StoreTestPlan(filename string, fileBytes []byte) error {
	if err := pf.makePlanDir(); err != nil {
		return err
	}
	f := filepath.Join(pf.dirname, filename)
	return os.WriteFile(f, fileBytes, filemode)
}

func (pf *PlanFiles) StoreDataFile(filename string, fileBytes []byte, dataConfig []*enginesModel.EngineDataConfig) error {
	if err := pf.makePlanDir(); err != nil {
		return err
	}
	datadir := filepath.Join(pf.dirname, filename)
	if _, err := os.Stat(datadir); os.IsNotExist(err) {
		err := os.MkdirAll(datadir, filemode)
		if err != nil {
			return err
		}
	}
	for engineID, edc := range dataConfig {
		subfolder := filepath.Join(datadir, strconv.Itoa(engineID))
		if _, err := os.Stat(subfolder); os.IsNotExist(err) {
			err := os.MkdirAll(subfolder, filemode)
			if err != nil {
				return err
			}
		}
		for _, sf := range edc.EngineData {
			if sf.Filename != filename {
				continue
			}
			f := filepath.Join(subfolder, sf.Filename)
			splittedCSV, err := utils.SplitCSV(fileBytes, sf.TotalSplits, sf.CurrentSplit)
			if err != nil {
				return err
			}
			if err := os.WriteFile(f, splittedCSV, filemode); err != nil {
				return err
			}
		}
	}
	return nil
}

func (pf *PlanFiles) makePlanDir() error {
	dirname := pf.makeDirName()
	if _, err := os.Stat(dirname); os.IsNotExist(err) {
		err := os.MkdirAll(dirname, filemode)
		if err != nil {
			return err
		}
	}
	return nil
}

func (pf *PlanFiles) makeDirName() string {
	return filepath.Join(pf.rootDir, "collection", pf.CollectionID, "plan", pf.PlanID)
}

func (pf *PlanFiles) TestFilePath(filename string) string {
	return filepath.Join(pf.makeDirName(), filename)
}

func (pf *PlanFiles) EngineDataPath(filename string, engineNo int) string {
	return filepath.Join(pf.makeDirName(), filename, strconv.Itoa(engineNo), filename)
}
