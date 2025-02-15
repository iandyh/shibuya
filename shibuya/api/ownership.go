package api

import (
	"github.com/rakutentech/shibuya/shibuya/config"
	"github.com/rakutentech/shibuya/shibuya/model"
)

func hasProjectOwnership(project *model.Project, account *model.Account, authConfig *config.AuthConfig) bool {
	if _, ok := account.MLMap[project.Owner]; !ok {
		if !account.IsAdmin(authConfig) {
			return false
		}
	}
	return true
}
