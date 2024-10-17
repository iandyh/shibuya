package client

import (
	"fmt"
	"os"
)

type PlanClient struct {
	Meta
	*ClientOpts
}

func NewPlanClient(clientOpts *ClientOpts) PlanClient {
	return PlanClient{
		Meta: Meta{
			Kind: "plans",
		},
		ClientOpts: clientOpts,
	}
}

func (pc PlanClient) Create(projectID, name string) error {
	resourceUrl := pc.ResourceUrl(pc.Endpoint, "")
	params := map[string]string{
		"project_id": projectID,
		"name":       name,
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

// TODO: currently, if the file already exists, we should return 400
// Instead, it's returning 500 now. We should fix it.
func (pc PlanClient) UploadFile(planID string, file *os.File) error {
	subResource := fmt.Sprintf("%s/files", planID)
	resourceUrl := pc.ResourceUrl(pc.Endpoint, subResource)
	req, err := makeFileUploadRequest(resourceUrl, "PUT", "planFile", file)
	if err != nil {
		return err
	}
	resp, err := pc.Client.Do(req)
	if err != nil {
		return err
	}
	return handleResponse(resp)
}
