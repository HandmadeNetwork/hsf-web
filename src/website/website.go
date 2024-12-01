package website

import (
	"context"
	"errors"
	"hsf/src/buildcss"
	"hsf/src/config"
	"hsf/src/jobs"
	"hsf/src/logging"
	"hsf/src/templates"
	"net/http"
	"os"
	"os/signal"
	"time"
)

func Start() {
	logging.Info().Msg("Starting HSF webserver")

	jobTracker := jobs.NewTracker(context.Background())
	templates.LoadEmbedded()

	jobTracker.Add(
		templates.WatchTemplates(jobTracker),
		buildcss.RunServer(jobTracker),
	)

	server := http.Server{
		Addr:    config.Config.WebserverAddr,
		Handler: BuildRoutes(),
	}

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt)
	bgChan := make(chan struct{})
	go func() {
		<-signals
		logging.Info().Msg("Shutting down...")
		go func() {
			go func() {
				logging.Info().Msg("Shutting down background jobs...")
				unfinished := jobTracker.Finish(10 * time.Second)
				if len(unfinished) == 0 {
					logging.Info().Msg("Background jobs closed gracefully")
				} else {
					logging.Warn().Strs("Unfinished", unfinished).Msg("Background jobs did not finish by the deadline")
				}
				close(bgChan)
			}()
			logging.Info().Msg("Shutting down http server...")
			timeout, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			server.Shutdown(timeout)
		}()

		<-signals
		logging.Warn().Strs("Unfinished background jobs", jobTracker.ListUnfinished()).Msg("Forcibly killed the website")
		os.Exit(1)
	}()

	logging.Info().Str("Address", server.Addr).Msg("Serving HSF website")
	serverErr := server.ListenAndServe()
	if !errors.Is(serverErr, http.ErrServerClosed) {
		logging.Error().Err(serverErr).Msg("Server shut down unexpectedly")
	}
	<-bgChan
}
