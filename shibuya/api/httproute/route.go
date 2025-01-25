package httproute

import (
	"fmt"
	"net/http"
)

type Route struct {
	Name        string
	Method      string
	Path        string
	HandlerFunc http.HandlerFunc
}

type PathHandler struct {
	Path string
}

type Routes []*Route

func (r *Route) MakePttern() string {
	return fmt.Sprintf("%s %s", r.Method, r.Path)
}
