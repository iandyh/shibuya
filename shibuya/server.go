package main

import (
	"fmt"
	"net/http"

	"log"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/gorilla/context"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rakutentech/shibuya/shibuya/api"
	"github.com/rakutentech/shibuya/shibuya/auth"
	"github.com/rakutentech/shibuya/shibuya/config"
	"github.com/rakutentech/shibuya/shibuya/model"
	"github.com/rakutentech/shibuya/shibuya/ui"
	_ "go.uber.org/automaxprocs"
)

func main() {
	sc := config.LoadConfig()
	config.SetupLogging(sc)
	endpoint := model.MakeMySQLEndpoint(sc.DBConf)
	if err := auth.CreateSesstionStore(endpoint, sc.DBConf.Keypairs); err != nil {
		log.Fatal(err)
	}

	if err := model.CreateMySQLClient(sc.DBConf); err != nil {
		log.Fatal(err)
	}
	routes := api.NewAPIServer(sc).InitRoutes()
	uiRoutes := ui.NewUI(sc).InitRoutes()
	routes = append(routes, uiRoutes...)
	mux := http.NewServeMux()
	for _, route := range routes {
		mux.HandleFunc(route.MakePttern(), route.HandlerFunc)
	}

	mux.Handle("GET /metrics", promhttp.Handler())

	handler := http.Handler(mux)
	handler = api.RequestLoggerWithoutPaths(handler)(handler)
	middlewares := []func(http.Handler) http.Handler{
		middleware.RequestID,
		middleware.RealIP,
	}
	for _, m := range middlewares {
		handler = m(handler)
	}
	// This should be the last one to be wrapper in order to pass the context to
	// future middlewares
	handler = api.ExcludePathsFromLogger(handler)(handler)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", 8080), context.ClearHandler(handler)))
}
