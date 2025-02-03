package api

import (
	"bytes"
	"fmt"
	"net/http"
	"time"

	httproute "github.com/rakutentech/shibuya/shibuya/http/route"
	"github.com/rakutentech/shibuya/shibuya/object_storage"
)

type FileAPI struct {
	objStorage object_storage.StorageInterface
}

func NewFileAPI(objStorage object_storage.StorageInterface) *FileAPI {
	fa := &FileAPI{
		objStorage: objStorage,
	}
	return fa
}

func (fa *FileAPI) Router() *httproute.Router {
	router := httproute.NewRouter("files", "/files")
	router.AddRoutes(httproute.Routes{
		{
			Name:        "Get a file",
			Method:      "GET",
			Path:        "{kind}/{id}/{name}",
			HandlerFunc: fa.fileDownloadHandler,
		},
	})
	return router
}

func (fa *FileAPI) fileDownloadHandler(w http.ResponseWriter, req *http.Request) {
	kind := req.PathValue("kind")
	id := req.PathValue("id")
	name := req.PathValue("name")
	filename := fmt.Sprintf("%s/%s/%s", kind, id, name)

	data, err := fa.objStorage.Download(filename)
	if err != nil {
		renderJSON(w, http.StatusNotFound, "not found")
		return
	}
	r := bytes.NewReader(data)
	w.Header().Add("Content-Disposition", "Attachment")
	http.ServeContent(w, req, filename, time.Now(), r)
}
