package main

import (
	"os"

	cdrserver "github.com/rakutentech/shibuya/shibuya/coordinator/server"
	log "github.com/sirupsen/logrus"

	_ "go.uber.org/automaxprocs"
)

func initFromEnv() cdrserver.CoordinatorConfig {
	namespace := os.Getenv("POD_NAMESPACE")
	projectID := os.Getenv("project_id")
	logLevel := os.Getenv("log_level")
	listenAddr := os.Getenv("listen_addr")
	return cdrserver.CoordinatorConfig{
		Namespace:  namespace,
		ProjectID:  projectID,
		LogLevel:   logLevel,
		ListenAddr: listenAddr,
		InCluster:  true,
		EnableTLS:  true,
	}
}

func main() {
	cc := initFromEnv()
	switch cc.LogLevel {
	case "debug":
		log.SetLevel(log.DebugLevel)
	default:
		log.SetLevel(log.InfoLevel)
	}

	if cc.ListenAddr == "" {
		cc.ListenAddr = ":8080"
	}
	coordinator := cdrserver.NewShibuyaCoordinator(cc)
	if err := coordinator.ListenHTTP(); err != nil {
		log.Fatal(err)
	}
}
