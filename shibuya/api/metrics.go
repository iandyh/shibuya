package api

import (
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"time"

	"github.com/rakutentech/shibuya/shibuya/config"
	httproute "github.com/rakutentech/shibuya/shibuya/http/route"
	log "github.com/sirupsen/logrus"
)

type MetricsGateway struct {
	backends []config.MetricStorage
	tr       *http.Transport
}

func (mg *MetricsGateway) rewrite(r *httputil.ProxyRequest) {
	collectionID := r.In.Header.Get("collection_id")
	// Ignore empty collection header for reducing requests to the db
	if collectionID == "" {
		return
	}
	cid, err := strconv.Atoi(collectionID)
	if err != nil {
		return
	}
	// We disallow admin to write metrics.  so a nil authconfig is provided here
	// One thing we can improve(TODO) in the future is to return the error directly here to the users
	// for the errors(403, 404, etc)
	if _, err := checkCollectionOwnership(int64(cid), r.In, nil); err != nil {
		return
	}
	// TODO: right now we don't support fan-out as reverseproxy does not suppport it
	// If we need to support multiple backends, we need to write our own handlers.
	// However, there is a greater chance that when the metrics are passing the gateway,
	// we only need to write to one backend as we should rely on the replication
	// at the storage level. As a result, fanout will cause data duplicattion.
	backend := mg.backends[0]
	target, err := url.Parse(backend.RemoteWriteUrl)
	if err != nil {
		log.Error(err)
		return
	}
	// TODO: need to verify the ownership based on api key later
	r.SetURL(target)
	out := r.Out
	out.URL.Path = "/api/v1/write"
}

// The gateway forwards the metrics send from the scraper to the metric storage
// It has Authn/Authz to protect tenant data.
// It might need to separated as a standalone component. But atm, it should be ok
// to be running inside the apiserver.
func NewMetricsGateway(metricStorage []config.MetricStorage) *MetricsGateway {
	return &MetricsGateway{
		backends: metricStorage,
		tr: &http.Transport{
			MaxIdleConnsPerHost:   1000,
			ResponseHeaderTimeout: 3 * time.Second, // We should expect fast response from backend storage
			IdleConnTimeout:       1 * time.Hour,
		},
	}
}

func (mg *MetricsGateway) Router() *httproute.Router {
	router := httproute.NewRouter("metrics gateway", "/metrics")
	rp := &httputil.ReverseProxy{Rewrite: mg.rewrite, Transport: mg.tr}
	router.AddRoutes(httproute.Routes{
		{
			Name:        "gateway",
			Method:      "POST",
			HandlerFunc: rp.ServeHTTP,
		},
	})
	return router
}
