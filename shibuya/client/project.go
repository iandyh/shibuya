package client

import (
	"net/http"
)

type ProjectClient struct {
	Meta
	*ClientOpts
}

func NewProjectClient(clientOpts *ClientOpts) ProjectClient {
	return ProjectClient{
		Meta: Meta{
			Kind: "projects",
		},
		ClientOpts: clientOpts,
	}
}

func (pc ProjectClient) Create(name, owner string) error {
	resourceUrl := pc.ResourceUrl(pc.Endpoint, "")
	params := map[string]string{
		"name":  name,
		"owner": owner,
		"sid":   "1",
	}
	req, err := makeFormRequest(resourceUrl, params)
	if err != nil {
		return err
	}
	resp, err := pc.Client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return handleResponse(resp)
}

func (pc ProjectClient) Get(ID string) error {
	resourceUrl := pc.ResourceUrl(pc.Endpoint, ID)
	req, err := http.NewRequest("GET", resourceUrl, nil)
	if err != nil {
		return err
	}
	resp, err := pc.Client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}
