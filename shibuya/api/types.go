package api

import (
	"github.com/rakutentech/shibuya/shibuya/model"
	smodel "github.com/rakutentech/shibuya/shibuya/scheduler/model"
)

type ShibuyaObject interface {
	*model.Project | *model.Collection | *model.Plan | *smodel.CollectionStatus
}
