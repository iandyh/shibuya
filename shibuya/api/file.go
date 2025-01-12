package api

import (
	"bytes"
	"fmt"
	"net/http"
	"time"

	"github.com/rakutentech/shibuya/shibuya/object_storage"
)

type FileAPI struct {
	PathHandler
	objStorage object_storage.StorageInterface
}

var (
	staticFileServer = http.FileServer(http.Dir("/static"))
)

func NewFileAPI(objStorage object_storage.StorageInterface) *FileAPI {
	return &FileAPI{
		PathHandler: PathHandler{
			Path: "/api/files",
		},
		objStorage: objStorage,
	}
}

func (fa *FileAPI) collectRoutes() Routes {
	return Routes{
		{
			Name:        "Get a file",
			Method:      "GET",
			Path:        fmt.Sprintf("%s/{kind}/{id}/{name}", fa.Path),
			HandlerFunc: fa.fileDownloadHandler,
		},
		{
			Name:        "Get static files",
			Method:      "GET",
			Path:        "/static/{filepath...}",
			HandlerFunc: fa.serveStaticFile,
		},
	}
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

func (fa *FileAPI) serveStaticFile(w http.ResponseWriter, req *http.Request) {
	// Set the cache expiration time to 7 days
	w.Header().Set("Cache-Control", "public, max-age=604800")
	filepath := req.PathValue("filepath")
	req.URL.Path = filepath
	staticFileServer.ServeHTTP(w, req)
}
