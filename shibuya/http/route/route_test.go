package route_test

import (
	"testing"

	httproute "github.com/rakutentech/shibuya/shibuya/http/route"
	"github.com/stretchr/testify/assert"
)

func TestRoute(t *testing.T) {
	testcases := []struct {
		name         string
		expectedPath string
		routes       httproute.Routes
	}{
		{
			name:         "with slash",
			expectedPath: "/collections/get",
			routes: httproute.Routes{
				{
					Name:   "get",
					Path:   "/get",
					Method: "GET",
				},
			},
		},
		{
			name:         "without slash",
			expectedPath: "/collections/post",
			routes: httproute.Routes{
				{
					Name:   "post",
					Path:   "post",
					Method: "POST",
				},
			},
		},
		{
			name:         "with pattern",
			expectedPath: "/collections/{collection_id}",
			routes: httproute.Routes{
				{
					Name:   "get collection by id",
					Path:   "{collection_id}",
					Method: "GET",
				},
			},
		},
	}
	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			router := &httproute.Router{
				Name: "collection",
				Path: "/collections",
			}
			router.AddRoutes(tc.routes)
			for _, r := range router.GetRoutes() {
				assert.Equal(t, tc.expectedPath, r.Path)
			}
		})
	}
}

func TestSubrouter(t *testing.T) {
	root := &httproute.Router{
		Name: "shibuya",
		Path: "",
	}
	apiRouter := &httproute.Router{
		Name: "shibuya api",
		Path: "/api",
	}
	collectionRouter := &httproute.Router{
		Name: "collection",
		Path: "/collections",
	}
	collectionRouter.AddRoutes(httproute.Routes{
		{
			Name:   "get collection",
			Path:   "{collection_id}",
			Method: "GET",
		},
		{
			Name:   "get collection",
			Method: "GET",
		},
	})
	apiRouter.Mount(collectionRouter)
	assert.Equal(t, 2, len(apiRouter.GetRoutes()))
	root.Mount(apiRouter)
	for _, r := range root.GetRoutes() {
		t.Log(r.Name)
		t.Log(r.Path)
	}
	assert.Equal(t, 2, len(root.GetRoutes()))
}
