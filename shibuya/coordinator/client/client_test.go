package client_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	cdrclient "github.com/rakutentech/shibuya/shibuya/coordinator/client"
	cdrserver "github.com/rakutentech/shibuya/shibuya/coordinator/server"
	"github.com/stretchr/testify/assert"
)

func TestClient(t *testing.T) {
	namespace := "shibuya-executors"
	projectID := "1"
	cc := cdrserver.CoordinatorConfig{
		Namespace: namespace,
		ProjectID: projectID,
	}
	s := cdrserver.NewShibuyaCoordinator(cc)
	server := httptest.NewServer(s.Mux)
	endpoint := server.URL
	client := cdrclient.NewClient(&http.Client{Timeout: 5 * time.Second})
	err := client.ProgressCheck(endpoint, 1, 1)
	assert.Error(t, err)
}
