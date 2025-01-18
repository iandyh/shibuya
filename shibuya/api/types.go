package api

import "github.com/rakutentech/shibuya/shibuya/model"

type ShibuyaObject interface {
	*model.Project | *model.Collection | *model.Plan
}
