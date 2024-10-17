package client

import (
	"fmt"
	"net/http"
	"time"
)

type Meta struct {
	Kind string
	ID   string
}

type ClientOpts struct {
	Client   *http.Client
	Endpoint string
}

func NewClientOpts(endpoint string, httpClient *http.Client) *ClientOpts {
	if httpClient == nil {
		httpClient = &http.Client{
			Timeout: 5 * time.Second,
		}
	}
	return &ClientOpts{
		Client:   httpClient,
		Endpoint: endpoint,
	}
}

func (m Meta) ResourceUrl(endpoint, subResource string) string {
	if subResource == "" {
		return fmt.Sprintf("%s/api/%s", endpoint, m.Kind)
	}
	return fmt.Sprintf("%s/api/%s/%s", endpoint, m.Kind, subResource)

}
