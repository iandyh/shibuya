package server

import (
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/rakutentech/shibuya/shibuya/coordinator/api"
	"github.com/rakutentech/shibuya/shibuya/coordinator/planprogress"
	"github.com/rakutentech/shibuya/shibuya/coordinator/upstream"
	pubsub "github.com/reqfleet/pubsub/server"
	log "github.com/sirupsen/logrus"
)

const (
	certFile = "/tls/tls.crt"
	keyFile  = "/tls/tls.key"
)

type ShibuyaCoordinator struct {
	inventory *upstream.Inventory
	Handler   http.Handler
	cc        CoordinatorConfig
}

var tr = &http.Transport{
	// Currently we have 4 engines per host. Each engine will require at least 2 connections.
	// 1 for metric subscription and 1 for trigger/healthcheck requests.
	// So minimum per host is 8. Currently, the capacity should be big enough
	// because it's designed with 10 engines per host and 10 conns per engine.
	MaxIdleConnsPerHost: 100,

	// Usually one collection will not run longer than 1 hour. If it's longer than 1 Hour,
	// We should do some GC to prevent too many connections accumulated.
	IdleConnTimeout: 1 * time.Hour,

	// We wait max 5 minutes for engines to respond. A complex plan might take some time to start.
	// But it should no longer than 5 minutes.
	ResponseHeaderTimeout: 5 * time.Minute,
}

type CoordinatorConfig struct {
	Namespace  string
	ProjectID  string
	LogLevel   string
	ListenAddr string
	EnableTLS  bool
	InCluster  bool
	APIKey     string
}

func (sc *ShibuyaCoordinator) authRequired(next http.Handler) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {
		bearer := r.Header.Get("Authorization")
		if bearer == "" {
			http.Error(w, "bearer header is empty", http.StatusForbidden)
			return
		}
		t := strings.Split(bearer, " ")
		if len(t) != 2 {
			http.Error(w, "bearer header is invalid", http.StatusBadRequest)
			return
		}
		key := t[1]
		if key != sc.cc.APIKey {
			http.Error(w, fmt.Sprintf("incorrect token %s", key), http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	}
	return http.HandlerFunc(fn)
}

func NewShibuyaCoordinator(cc CoordinatorConfig) *ShibuyaCoordinator {
	log.Infof("Engine namespace %s", cc.Namespace)
	log.Infof("Project ID: %s", cc.ProjectID)

	inventory, err := upstream.NewInventory(cc.Namespace, cc.InCluster)
	if err != nil {
		log.Fatal(err)
	}
	serverOpts := pubsub.ServerOpts{
		Mode:     pubsub.TCP,
		Password: cc.APIKey,
	}
	pub := pubsub.NewPubSubServer(serverOpts)
	go pub.Listen()

	s := &ShibuyaCoordinator{inventory: inventory}
	go s.inventory.MakeInventory(cc.ProjectID)
	rp := httputil.ReverseProxy{
		Rewrite:   s.rewriteURL,
		Transport: tr,
	}
	mux := http.NewServeMux()
	pp := planprogress.NewPlanProgress()
	apiserver := api.NewAPIServer(pub, pp, inventory)
	routes := apiserver.MakeHTTPRoutes()
	for _, route := range routes {
		mux.HandleFunc(route.MakePttern(), route.HandlerFunc)
	}
	mux.Handle("/{engine}/stream", &rp)
	handler := http.Handler(mux)

	middlewares := []func(http.Handler) http.Handler{
		s.authRequired,
	}
	for _, m := range middlewares {
		handler = m(handler)
	}
	s.Handler = handler
	s.cc = cc
	return s
}

// This func does two things:
// 1. It rewrites ingress ip to engine ip.
// 2. It rewrites path by removing engine id info.
// Usage of this func is guided by code here: https://github.com/golang/go/blob/go1.20.2/src/net/http/httputil/reverseproxy.go#L42
func (sic *ShibuyaCoordinator) rewriteURL(r *httputil.ProxyRequest) {
	// When we encoutered an error, the rewrite won't happen. Controller side should see 502
	// Which is the expected behaviour from reverse proxy POV.
	in := r.In
	items := strings.Split(in.RequestURI, "/")
	if len(items) < 3 {
		log.Error(fmt.Errorf("Invalid request path %s", in.RequestURI))
		return
	}
	log.Debugf("The path items are %v", items)
	engine := items[1]
	podIP := sic.inventory.FindPodIP(engine)
	if podIP == "" {
		log.Warnf("Cannot find pod ip for %s", engine)
		return
	}
	target, err := url.Parse(fmt.Sprintf("http://%s", podIP))
	if err != nil {
		log.Error(err)
		return
	}
	out := r.Out
	r.SetURL(target)
	// We need to rewrite the path from /engine-project-collection-plan-engineid/start to /start
	// Otherwise it will be 404 at engine handler side
	t := fmt.Sprintf("/%s", path.Join(items[2:]...))
	orig := out.URL.Path
	out.URL.Path = t
	out.URL.RawPath = t
	log.Debugf("rewriting original path %s to %s", orig, out.URL.Path)
}

func (s *ShibuyaCoordinator) ListenHTTP() error {
	cc := s.cc
	if cc.EnableTLS {
		return http.ListenAndServeTLS(cc.ListenAddr, certFile, keyFile, s.Handler)
	}
	return http.ListenAndServe(cc.ListenAddr, s.Handler)
}
