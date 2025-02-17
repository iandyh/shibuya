package server

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	log "github.com/sirupsen/logrus"
)

var (
	STOPSIGNALS = []os.Signal{syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT}
)

func StartServer(server *http.Server, certFile, keyFile string, ctx context.Context) error {
	// Server run context
	serverCtx, serverStopCtx := context.WithCancel(context.Background())

	// Listen for syscall signals for process to interrupt/quit
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, STOPSIGNALS...)
	go func() {
		<-sig
		// Shutdown signal with grace period of 30 seconds
		timeout := 30 * time.Second
		log.Infof("Received shutdown signal. The graceful period will be %v", timeout)
		shutdownCtx, cancel := context.WithTimeout(serverCtx, timeout)
		defer cancel()
		go func() {
			<-shutdownCtx.Done()
			if shutdownCtx.Err() == context.DeadlineExceeded {
				log.Fatal("graceful shutdown timed out.. forcing exit.")
			}
		}()
		// Trigger graceful shutdown
		if ctx != nil {
			log.Info("Other context presented. Need to handle it")
			<-ctx.Done()
		}
		log.Info("Finally, Going to shut down http server...")
		err := server.Shutdown(shutdownCtx)
		if err != nil {
			log.Fatal(err)
		}
		serverStopCtx()
	}()
	// Run the server
	if certFile != "" && keyFile != "" {
		log.Infof("Started tls server on %s", server.Addr)
		if err := server.ListenAndServeTLS(certFile, keyFile); err != nil && err != http.ErrServerClosed {
			return err
		}
	} else {
		log.Infof("Started server on %s", server.Addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			return err
		}
	} // Wait for server context to be stopped
	<-serverCtx.Done()
	log.Info("Shut down is finished.")
	return nil
}
