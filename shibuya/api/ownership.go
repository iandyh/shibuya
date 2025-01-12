package api

import (
	"net/http"

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

func hasCollectionOwnership(r *http.Request, authConfig *config.AuthConfig) (*model.Collection, error) {
	collectionID := r.PathValue("collection_id")
	collection, err := getCollection(collectionID)
	if err != nil {
		return nil, err
	}
	account := r.Context().Value(accountKey).(*model.Account)
	project, err := model.GetProject(collection.ProjectID)
	if err != nil {
		return nil, err
	}
	if r := hasProjectOwnership(project, account, authConfig); !r {
		return nil, makeCollectionOwnershipError()
	}
	return collection, nil
}
