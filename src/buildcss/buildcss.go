package buildcss

import (
	"hsf/src/config"
	"hsf/src/jobs"
	"hsf/src/logging"
	"hsf/src/utils"

	"github.com/evanw/esbuild/pkg/api"
)

/*
 * Recent versions of CSS have finally added the ability to nest selectors. We
 * want to take advantage of this for our own sanity, but not all browsers yet
 * support this new CSS syntax. We therefore run our CSS through a tool called
 * esbuild to transpile the CSS into a more widely-compatible form.
 *
 * Conveniently, esbuild is written in Go, which means we can just import it as
 * a package and use its API directly, rather than using it as a
 * command-line tool.
 */

var ActiveServerPort uint16

func BuildContext() (api.BuildContext, *api.ContextError) {
	return api.Context(api.BuildOptions{
		EntryPoints: []string{
			"src/rawdata/css/style.css",
		},
		Outbase:  "src/rawdata/css",
		Outdir:   "public",
		External: []string{"/public/*"},
		Bundle:   true,
		Write:    true,
		Engines: []api.Engine{
			{Name: api.EngineChrome, Version: "109"},
			{Name: api.EngineFirefox, Version: "109"},
			{Name: api.EngineSafari, Version: "12"},
		},
	})
}

func RunServer() *jobs.Job {
	job := jobs.New("BuildCSS")
	if config.Config.Env != config.Dev {
		return job.Finish()
	}

	logger := logging.ExtractLogger(job.Ctx).With().Str("module", "EsBuild").Logger()
	esCtx := utils.Must1(BuildContext())

	logger.Info().Msg("Starting EsBuild server and watcher")
	utils.Must(esCtx.Watch(api.WatchOptions{}))
	serverResult := utils.Must1(esCtx.Serve(api.ServeOptions{
		Port:     config.Config.EsBuild.Port,
		Servedir: "./",
		OnRequest: func(args api.ServeOnRequestArgs) {
			if args.Status != 200 {
				logger.Warn().Interface("args", args).Msg("Response from esbuild server")
			}
		},
	}))
	ActiveServerPort = serverResult.Port
	logger.Info().Msgf("EsBuild server running at %d", ActiveServerPort)

	go func() {
		<-job.Canceled()
		logger.Info().Msg("Shutting down esbuild server and watcher")
		esCtx.Dispose()
		job.Finish()
	}()

	return job
}
