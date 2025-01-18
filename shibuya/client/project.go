package client

import (
	"strconv"

	"github.com/rakutentech/shibuya/shibuya/model"
)

type ProjectClient struct {
	Meta
	*ClientOpts
}

func NewProjectClient(clientOpts *ClientOpts) *ProjectClient {
	return &ProjectClient{
		Meta: Meta{
			Kind: "projects",
		},
		ClientOpts: clientOpts,
	}
}

func (pc *ProjectClient) Create(name, owner string) (*model.Project, error) {
	resourceUrl := pc.ResourceUrl(pc.Endpoint, "")
	params := map[string]string{
		"name":  name,
		"owner": owner,
		"sid":   "1",
	}
	return sendCreateRequest(pc.Client, resourceUrl, params, &model.Project{})
}

func (pc *ProjectClient) Get(ID int64) (*model.Project, error) {
	resourceUrl := pc.ResourceUrl(pc.Endpoint, strconv.Itoa(int(ID)))
	return sendGetRequest(pc.Client, resourceUrl, &model.Project{})
}

func (pc *ProjectClient) Delete(ID int64) error {
	resourceUrl := pc.ResourceUrl(pc.Endpoint, strconv.Itoa(int(ID)))
	return sendDeleteRequest(pc.Client, resourceUrl)
}
