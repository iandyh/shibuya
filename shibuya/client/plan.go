package client

import (
	"fmt"
	"os"

	"github.com/rakutentech/shibuya/shibuya/model"
)

type PlanClient struct {
	Meta
	*ClientOpts
}

func NewPlanClient(clientOpts *ClientOpts) *PlanClient {
	return &PlanClient{
		Meta: Meta{
			Kind: "plans",
		},
		ClientOpts: clientOpts,
	}
}

func (pc *PlanClient) Create(projectID, name string, kind model.PlanKind) (*model.Plan, error) {
	resourceUrl := pc.ResourceUrl(pc.Endpoint, "")
	params := map[string]string{
		"project_id": projectID,
		"name":       name,
		"kind":       string(kind),
	}
	return sendCreateRequest(pc.Client, resourceUrl, params, &model.Plan{})
}

// TODO: currently, if the file already exists, we should return 400
// Instead, it's returning 500 now. We should fix it.
func (pc *PlanClient) UploadFile(planID int64, file *os.File) error {
	subResource := fmt.Sprintf("%d/files", planID)
	resourceUrl := pc.ResourceUrl(pc.Endpoint, subResource)
	req, err := makeFileUploadRequest(resourceUrl, "PUT", "planFile", file)
	if err != nil {
		return err
	}
	resp, err := pc.Client.Do(req)
	if err != nil {
		return err
	}
	return handleResponse(resp, nil)
}

func (pc *PlanClient) Delete(planID int64) error {
	resourceUrl := pc.ResourceUrl(pc.Endpoint, fmt.Sprintf("%d", planID))
	return sendDeleteRequest(pc.Client, resourceUrl)
}

func (pc *PlanClient) Get(planID int64) (*model.Plan, error) {
	resourceUrl := pc.ResourceUrl(pc.Endpoint, fmt.Sprintf("%d", planID))
	return sendGetRequest(pc.Client, resourceUrl, &model.Plan{})
}
