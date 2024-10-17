package client

import (
	"context"
	"fmt"
	"net/http"
	"os"

	es "github.com/iandyh/eventsource"
)

type CollectionClient struct {
	Meta
	ClientOpts
}

func NewCollectionClient(opts ClientOpts) CollectionClient {
	return CollectionClient{
		Meta: Meta{
			Kind: "collections",
		},
		ClientOpts: opts,
	}
}

func (cc CollectionClient) Create(projectID, name string) error {
	resourceUrl := cc.ResourceUrl(cc.Endpoint, "")
	params := map[string]string{
		"project_id": projectID,
		"name":       name,
	}
	req, err := makeFormRequest(resourceUrl, params)
	if err != nil {
		return err
	}
	resp, err := cc.Client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return handleResponse(resp)
}

func (cc CollectionClient) Configure(collectionID string, confFile *os.File) error {
	subResource := fmt.Sprintf("%s/config", collectionID)
	resourceUrl := cc.ResourceUrl(cc.Endpoint, subResource)
	req, err := makeFileUploadRequest(resourceUrl, "PUT", "collectionYAML", confFile)
	if err != nil {
		return err
	}
	resp, err := cc.Client.Do(req)
	if err != nil {
		return err
	}
	return handleResponse(resp)
}

func (cc CollectionClient) sendCollectionVerbReq(collectionID, verb string) error {
	subResource := fmt.Sprintf("%s/%s", collectionID, verb)
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
	return handleResponse(resp)
}

func (cc CollectionClient) Launch(collectionID string) error {
	return cc.sendCollectionVerbReq(collectionID, "deploy")
}

// TODO we should prevent triggering at api side as well
func (cc CollectionClient) Trigger(collectionID string) error {
	return cc.sendCollectionVerbReq(collectionID, "trigger")
}

func (cc CollectionClient) Stop(collectionID string) error {
	return cc.sendCollectionVerbReq(collectionID, "stop")
}

func (cc CollectionClient) Purge(collectionID string) error {
	return cc.sendCollectionVerbReq(collectionID, "purge")
}

func (cc CollectionClient) Status(collectionID string) error {
	return nil
}

func (cc CollectionClient) Subscribe(collectionID string) (*es.Stream, context.CancelFunc, error) {
	subResource := fmt.Sprintf("%s/stream", collectionID)
	resourceUrl := cc.ResourceUrl(cc.Endpoint, subResource)
	req, err := http.NewRequest("GET", resourceUrl, nil)
	if err != nil {
		return nil, nil, err
	}
	ctx, cancel := context.WithCancel(req.Context())
	req = req.WithContext(ctx)
	httpClient := &http.Client{}
	stream, err := es.SubscribeWith("", httpClient, req)
	if err != nil {
		cancel()
		return nil, nil, err
	}
	return stream, cancel, nil
}
