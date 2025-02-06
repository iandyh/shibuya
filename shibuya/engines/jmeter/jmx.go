package jmeter

import (
	"errors"
	"strconv"

	"github.com/beevik/etree"
	"github.com/rakutentech/shibuya/shibuya/coordinator/storage"
	enginesModel "github.com/rakutentech/shibuya/shibuya/engines/model"
)

func getThreadGroups(planDoc *etree.Document) ([]*etree.Element, error) {
	jtp := planDoc.SelectElement("jmeterTestPlan")
	if jtp == nil {
		return nil, errors.New("Missing Jmeter Test plan in jmx")
	}
	ht := jtp.SelectElement("hashTree")
	if ht == nil {
		return nil, errors.New("Missing hash tree inside Jmeter test plan in jmx")
	}
	ht = ht.SelectElement("hashTree")
	if ht == nil {
		return nil, errors.New("Missing hash tree inside hash tree in jmx")
	}
	tgs := ht.SelectElements("ThreadGroup")
	stgs := ht.SelectElements("SetupThreadGroup")
	tgs = append(tgs, stgs...)
	return tgs, nil
}

func parseTestPlan(file []byte) (*etree.Document, error) {
	doc := etree.NewDocument()
	if err := doc.ReadFromBytes(file); err != nil {
		return nil, err
	}
	return doc, nil
}

func modifyJMX(file []byte, pec enginesModel.PlanEnginesConfig) ([]byte, error) {
	planDoc, err := parseTestPlan(file)
	if err != nil {
		return nil, err
	}
	durationInt, err := strconv.Atoi(pec.Duration)
	if err != nil {
		return nil, err
	}
	// it includes threadgroups and setupthreadgroups
	threadGroups, err := getThreadGroups(planDoc)
	if err != nil {
		return nil, err
	}
	for _, tg := range threadGroups {
		children := tg.ChildElements()
		for _, child := range children {
			attrName := child.SelectAttrValue("name", "")
			switch attrName {
			case "ThreadGroup.duration":
				child.SetText(strconv.Itoa(durationInt * 60))
			case "ThreadGroup.scheduler":
				child.SetText("true")
			case "ThreadGroup.num_threads":
				child.SetText(pec.Concurrency)
			case "ThreadGroup.ramp_time":
				child.SetText(pec.Rampup)
			}
		}
	}
	return planDoc.WriteToBytes()
}

func MakeTestPlan(pf *storage.PlanFiles, planName, filename string, fileBytes []byte, pec enginesModel.PlanEnginesConfig) error {
	modified, err := modifyJMX(fileBytes, pec)
	if err != nil {
		return err
	}
	if err := pf.StoreTestPlan(filename, modified); err != nil {
		return err
	}
	return err
}
