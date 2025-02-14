package main

import (
	"fmt"
	"net/http"

	"log"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/gorilla/context"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rakutentech/shibuya/shibuya/api"
	"github.com/rakutentech/shibuya/shibuya/config"
	httpauth "github.com/rakutentech/shibuya/shibuya/http/auth"
	httproute "github.com/rakutentech/shibuya/shibuya/http/route"
	"github.com/rakutentech/shibuya/shibuya/model"
	"github.com/rakutentech/shibuya/shibuya/ui"
	_ "go.uber.org/automaxprocs"
)

var (
	excludedPaths = map[string]struct{}{
		"/metrics": {},
		"/health":  {},
	}
	excludedKeywords = []string{
		"stream",
	}
)

func main() {
	sc := config.LoadConfig()
	config.SetupLogging(sc)
	if err := model.CreateMySQLClient(sc.DBConf); err != nil {
		log.Fatal(err)
	}
	rootRouter := &httproute.Router{
		Name: "root",
		Path: "",
	}
	rootRouter.Mount(api.NewAPIServer(sc).Router())
	rootRouter.Mount(ui.NewUI(sc).Router())
	mux := rootRouter.Mux()

	mux.Handle("GET /metrics", promhttp.Handler())
	mux.Handle("GET /health", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Alive"))
	}))
	handler := http.Handler(mux)

	handler = httpauth.RequestLoggerWithoutPaths(handler)(handler)
	middlewares := []func(http.Handler) http.Handler{
		middleware.RequestID,
		middleware.RealIP,
	}
	for _, m := range middlewares {
		handler = m(handler)
	}
	// This should be the last one to be wrapper in order to pass the context to
	// future middlewares
	handler = httpauth.ExcludePathsFromLogger(handler, excludedPaths, excludedKeywords)(handler)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", 8080), context.ClearHandler(handler)))
}
