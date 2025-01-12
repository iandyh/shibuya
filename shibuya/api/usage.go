package api

import (
	"fmt"
	"net/http"

	log "github.com/sirupsen/logrus"

	"github.com/rakutentech/shibuya/shibuya/model"
)

type UsageAPI struct {
	PathHandler
}

func NewUsageAPI() *UsageAPI {
	return &UsageAPI{
		PathHandler: PathHandler{
			Path: "/api/usage",
		},
	}
}

func (ua *UsageAPI) collectRoutes() Routes {
	return Routes{
		{
			Name:        "Get usage summary",
			Method:      "GET",
			Path:        fmt.Sprintf("%s/summary", ua.Path),
			HandlerFunc: ua.usageSummaryHandler,
		},
		{
			Name:        "Get usage summary by sid",
			Method:      "GET",
			Path:        fmt.Sprintf("%s/summary_sid", ua.Path),
			HandlerFunc: ua.usageSummaryHandlerBySid,
		},
	}
}

func (ua *UsageAPI) usageSummaryHandler(w http.ResponseWriter, req *http.Request) {
	qs := req.URL.Query()
	st := qs.Get("started_time")
	et := qs.Get("end_time")
	summary, err := model.GetUsageSummary(st, et)
	if err != nil {
		log.Println(err)
		handleErrors(w, err)
		return
	}
	renderJSON(w, http.StatusOK, summary)
}

func (ua *UsageAPI) usageSummaryHandlerBySid(w http.ResponseWriter, req *http.Request) {
	qs := req.URL.Query()
	st := qs.Get("started_time")
	et := qs.Get("end_time")
	sid := qs.Get("sid")
	history, err := model.GetUsageSummaryBySid(sid, st, et)
	if err != nil {
		handleErrors(w, err)
		return
	}
	renderJSON(w, http.StatusOK, history)
}
