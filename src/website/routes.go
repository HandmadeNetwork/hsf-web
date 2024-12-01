package website

import (
	"bytes"
	"fmt"
	"hsf/src/buildcss"
	"hsf/src/logging"
	"io"
	"net/http"
	"net/http/httputil"
	"regexp"
	"strings"
	"time"
)

func BuildRoutes() http.Handler {
	router := &Router{}
	routes := RouteBuilder{
		Router: router,
		Middlewares: []Middleware{
			MiddlewareRequestLogger,
		},
	}

	// NOTE(asaf): Static files and EsBuild proxying.
	routes.GET(regexp.MustCompile(`^/public/.+$`), func(c *RequestContext) ResponseData {
		var res ResponseData
		if buildcss.ActiveServerPort != 0 {
			if strings.HasSuffix(c.Req.URL.Path, ".css") {
				proxy := httputil.ReverseProxy{
					Director: func(r *http.Request) {
						r.URL.Scheme = "http"
						r.URL.Host = fmt.Sprintf("localhost:%d", buildcss.ActiveServerPort)
						r.Host = "localhost"
					},
					ModifyResponse: func(esRes *http.Response) error {
						res.StatusCode = esRes.StatusCode
						if esRes.StatusCode > 400 {
							errStr, err := io.ReadAll(esRes.Body)
							if err != nil {
								return err
							}
							esRes.Body.Close()
							logging.Error().Str("EsBuild error", string(errStr)).Msg("EsBuild is complaining")
							esRes.Body = io.NopCloser(bytes.NewReader(errStr))
						}
						return nil
					},
				}
				logging.Debug().Msg("Redirecting css request to esbuild")
				proxy.ServeHTTP(c.Res, c.Req)
				res.Hijacked = true
				return res
			}
		}
		http.StripPrefix("/public/", http.FileServer(http.Dir("public"))).ServeHTTP(&res, c.Req)
		return res
	})

	routes.GET(regexp.MustCompile(`^/$`), LandingHTML)

	routes.GET(regexp.MustCompile(`^.+$`), func(c *RequestContext) ResponseData {
		return render404HTML(c)
	})

	return router
}

func MiddlewareRequestLogger(h Handler) Handler {
	return func(c *RequestContext) ResponseData {
		res := h(c)

		duration := time.Now().Sub(c.RequestStartTime)

		c.Logger.Info().Msgf("Served [%9s] HTTP:%d %s", duration.String(), res.StatusCode, c.Req.URL.Path)

		return res
	}
}

func LandingHTML(c *RequestContext) ResponseData {
	return renderHTML(c, "landing", GetBaseData())
}
