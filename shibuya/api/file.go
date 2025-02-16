package api

import (
	"bytes"
	"net/http"
	"time"

	"github.com/rakutentech/shibuya/shibuya/object_storage"
)

func serveFile(objStorage object_storage.StorageInterface, w http.ResponseWriter, req *http.Request, filename string) {
	data, err := objStorage.Download(filename)
	if err != nil {
		renderJSON(w, http.StatusNotFound, "not found")
		return
	}
	r := bytes.NewReader(data)
	w.Header().Add("Content-Disposition", "Attachment")
	http.ServeContent(w, req, filename, time.Now(), r)
}
