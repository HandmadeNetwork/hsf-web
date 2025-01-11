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
	"sync"
	"time"
)

func Start() {
	logging.Info().Msg("Starting HSF webserver")

	templates.LoadEmbedded()

	var wg sync.WaitGroup

	// Start background jobs
	wg.Add(1)
	backgroundJobs := jobs.Jobs{
		templates.WatchTemplates(),
		buildcss.RunServer(),
	}

	// Create tracker for long-running requests
	wg.Add(1)
	lrrTracker := NewLongRunningRequestTracker()

	// Create HTTP server
	wg.Add(1)
	server := http.Server{
		Addr:    config.Config.WebserverAddr,
		Handler: WebsiteRoutes(lrrTracker),
	}
	go func() {
		logging.Info().Str("Address", server.Addr).Msg("Serving HSF website")
		serverErr := server.ListenAndServe()
		if !errors.Is(serverErr, http.ErrServerClosed) {
			logging.Error().Err(serverErr).Msg("Server shut down unexpectedly")
		}
		// The wg.Done() happens in the shutdown logic below.
	}()

	// Wait for SIGINT in the background and gracefully shut down
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt)
	go func() {
		<-signals // First SIGINT (start shutdown)
		logging.Info().Msg("Shutting down...")

		const timeout = 10 * time.Second

		// Cancel long-running requests (allowing websockets et. al. to close)
		lrrTracker.Cancel()
		go func() {
			lrrTracker.Wait(timeout)
			wg.Done()
		}()

		// Shut down background jobs
		go func() {
			logging.Info().Msg("Shutting down background jobs...")
			unfinished := backgroundJobs.CancelAndWait(10 * time.Second)
			if len(unfinished) == 0 {
				logging.Info().Msg("Background jobs closed gracefully")
			} else {
				logging.Warn().Strs("Unfinished", unfinished).Msg("Background jobs did not finish by the deadline")
			}
			wg.Done()
		}()

		// Gracefully shut down the HTTP server
		go func() {
			timeoutCtx, cancel := context.WithTimeout(context.Background(), timeout)
			defer cancel()
			err := server.Shutdown(timeoutCtx)
			if err != nil {
				logging.Warn().Err(err).Msg("Server did not shut down gracefully")
			}
			wg.Done()
		}()

		<-signals // Second SIGINT (force quit)
		logging.Warn().Strs("Unfinished background jobs", backgroundJobs.ListUnfinished()).Msg("Forcibly killed the website")
		os.Exit(1)
	}()

	// Wait for all of the above to finish, then exit
	wg.Wait()
}
