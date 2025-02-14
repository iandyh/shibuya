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
	Token    string
}

type TransportWithToken struct {
	Transport http.RoundTripper
	Token     string
}

func (t *TransportWithToken) RoundTrip(req *http.Request) (*http.Response, error) {
	req.Header.Set("Authorization", "Bearer "+t.Token)
	// Use the underlying transport (default if nil)
	if t.Transport == nil {
		t.Transport = http.DefaultTransport
	}
	return t.Transport.RoundTrip(req)
}

func NewClientOpts(endpoint, token string, httpClient *http.Client) *ClientOpts {
	if httpClient == nil {
		httpClient = &http.Client{
			Transport: &TransportWithToken{
				Token: token,
			},
			Timeout: 5 * time.Second,
		}
	}
	return &ClientOpts{
		Client:   httpClient,
		Endpoint: endpoint,
		Token:    token,
	}
}

func (m Meta) ResourceUrl(endpoint, subResource string) string {
	if subResource == "" {
		return fmt.Sprintf("%s/api/%s", endpoint, m.Kind)
	}
	return fmt.Sprintf("%s/api/%s/%s", endpoint, m.Kind, subResource)

}
