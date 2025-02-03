package route

import (
	"fmt"
	"net/http"
	"strings"

	log "github.com/sirupsen/logrus"
)

type Route struct {
	Name        string
	Method      string
	Path        string
	HandlerFunc http.HandlerFunc
}

type Routes []*Route

type Router struct {
	Name      string
	Path      string
	routes    Routes
	subrouter []*Router
}

func NewRouter(name, path string) *Router {
	r := &Router{
		Name: name,
		Path: path,
	}
	return r
}

func (r *Route) MakePttern() string {
	return fmt.Sprintf("%s %s", strings.ToUpper(r.Method), r.Path)
}

func (rou *Router) makeFullPath(path string) string {
	if strings.HasPrefix(path, "/") {
		path = path[1:]
	}
	return strings.TrimSuffix(fmt.Sprintf("%s/%s", rou.Path, path), "/")
}

// TODO: shall we make it thread safe?
func (rou *Router) AddRoutes(routes Routes) {
	for _, r := range routes {
		r.Path = rou.makeFullPath(r.Path)
		rou.routes = append(rou.routes, r)
	}
}

func (rou *Router) Mount(sub *Router) {
	for _, r := range sub.GetRoutes() {
		r.Path = rou.makeFullPath(r.Path)
	}
	rou.subrouter = append(rou.subrouter, sub)
}

func (rou *Router) GetRoutes() Routes {
	all := make(Routes, 0)
	for _, r := range rou.routes {
		all = append(all, r)
	}
	for _, r := range rou.subrouter {
		all = append(all, r.GetRoutes()...)
	}
	return all
}

func (rou *Router) Mux() *http.ServeMux {
	mux := http.NewServeMux()
	for _, r := range rou.GetRoutes() {
		log.Infof("Registering --- handler name: %s, path: %s, method: %s. pattern: %s ---",
			r.Name, r.Path, r.Method, r.MakePttern())
		mux.HandleFunc(r.MakePttern(), r.HandlerFunc)
	}
	log.Infof("Total %d handlers are registered.", len(rou.GetRoutes()))
	return mux
}
