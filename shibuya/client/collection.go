package client

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strconv"

	es "github.com/iandyh/eventsource"
	"github.com/rakutentech/shibuya/shibuya/model"
	smodel "github.com/rakutentech/shibuya/shibuya/scheduler/model"
)

type CollectionClient struct {
	Meta
	ClientOpts
}

func NewCollectionClient(opts ClientOpts) *CollectionClient {
	return &CollectionClient{
		Meta: Meta{
			Kind: "collections",
		},
		ClientOpts: opts,
	}
}

func (cc *CollectionClient) Create(projectID, name string) (*model.Collection, error) {
	resourceUrl := cc.ResourceUrl(cc.Endpoint, "")
	params := map[string]string{
		"project_id": projectID,
		"name":       name,
	}
	return sendCreateRequest(cc.Client, resourceUrl, params, &model.Collection{})
}

func (cc *CollectionClient) Configure(collectionID int64, confFile *os.File) error {
	subResource := fmt.Sprintf("%d/config", collectionID)
	resourceUrl := cc.ResourceUrl(cc.Endpoint, subResource)
	req, err := makeFileUploadRequest(resourceUrl, "PUT", "collectionYAML", confFile)
	if err != nil {
		return err
	}
	resp, err := cc.Client.Do(req)
	if err != nil {
		return err
	}
	return handleResponse(resp, nil)
}

func (cc *CollectionClient) UploadFile(collectionID int64, dataFile *os.File) error {
	subResource := fmt.Sprintf("%d/files", collectionID)
	resourceUrl := cc.ResourceUrl(cc.Endpoint, subResource)
	req, err := makeFileUploadRequest(resourceUrl, "PUT", "collectionFile", dataFile)
	if err != nil {
		return err
	}
	resp, err := cc.Client.Do(req)
	if err != nil {
		return err
	}
	return handleResponse(resp, nil)
}

func (cc *CollectionClient) Subscribe(collectionID int64) (*es.Stream, context.CancelFunc, error) {
	subResource := fmt.Sprintf("%d/stream", collectionID)
	resourceUrl := cc.ResourceUrl(cc.Endpoint, subResource)
	req, err := http.NewRequest("GET", resourceUrl, nil)
	if err != nil {
		return nil, nil, err
	}
	ctx, cancel := context.WithCancel(req.Context())
	req = req.WithContext(ctx)
	httpClient := &http.Client{}
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", cc.ClientOpts.Token))
	stream, err := es.SubscribeWith("", httpClient, req)
	if err != nil {
		cancel()
		return nil, nil, err
	}
	return stream, cancel, nil
}

func (cc *CollectionClient) sendCollectionVerbReq(collectionID int64, verb string) error {
	subResource := fmt.Sprintf("%d/%s", collectionID, verb)
	resourceUrl := cc.ResourceUrl(cc.Endpoint, subResource)
	req, err := http.NewRequest("POST", resourceUrl, nil)
	if err != nil {
		return err
	}
	resp, err := cc.Client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return handleResponse(resp, nil)
}

func (cc *CollectionClient) Delete(collectionID int64) error {
	resourceUrl := cc.ResourceUrl(cc.Endpoint, strconv.Itoa(int(collectionID)))
	return sendDeleteRequest(cc.Client, resourceUrl)
}

func (cc *CollectionClient) Get(collectionID int64) (*model.Collection, error) {
	resourceUrl := cc.ResourceUrl(cc.Endpoint, strconv.Itoa(int(collectionID)))
	return sendGetRequest(cc.Client, resourceUrl, &model.Collection{})
}

func (cc *CollectionClient) Launch(collectionID int64) error {
	return cc.sendCollectionVerbReq(collectionID, "deploy")
}

// TODO we should prevent triggering at api side as well
func (cc *CollectionClient) Trigger(collectionID int64) error {
	return cc.sendCollectionVerbReq(collectionID, "trigger")
}

func (cc *CollectionClient) Stop(collectionID int64) error {
	return cc.sendCollectionVerbReq(collectionID, "stop")
}

func (cc *CollectionClient) Purge(collectionID int64) error {
	return cc.sendCollectionVerbReq(collectionID, "purge")
}

func (cc *CollectionClient) Status(collectionID int64) (*smodel.CollectionStatus, error) {
	subResource := fmt.Sprintf("%d/status", collectionID)
	resourceUrl := cc.ResourceUrl(cc.Endpoint, subResource)
	return sendGetRequest(cc.Client, resourceUrl, &smodel.CollectionStatus{})

}
