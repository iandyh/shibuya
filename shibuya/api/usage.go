package api

import (
	"net/http"

	log "github.com/sirupsen/logrus"

	httproute "github.com/rakutentech/shibuya/shibuya/http/route"
	"github.com/rakutentech/shibuya/shibuya/model"
)

type UsageAPI struct{}

func NewUsageAPI() *UsageAPI {
	ua := &UsageAPI{}
	return ua
}

func (ua *UsageAPI) Router() *httproute.Router {
	router := httproute.NewRouter("usage api", "usage")
	router.AddRoutes(httproute.Routes{
		{
			Name:        "Get usage summary",
			Method:      "GET",
			Path:        "summary",
			HandlerFunc: ua.usageSummaryHandler,
		},
		{
			Name:        "Get usage summary by sid",
			Method:      "GET",
			Path:        "summary_sid",
			HandlerFunc: ua.usageSummaryHandlerBySid,
		},
	})
	return router
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
