package website

import (
	"bytes"
	"fmt"
	"hsf/src/buildcss"
	"hsf/src/logging"
	"hsf/src/utils"
	"io"
	"net/http"
	"net/http/httputil"
	"regexp"
	"strings"
	"time"
)

func WebsiteRoutes(tracker *LongRunningRequestTracker) http.Handler {
	router := &Router{}
	routes := RouteBuilder{
		Router: router,
		Middlewares: []Middleware{
			MiddlewareSetLRRTracker(tracker),
			MiddlewareRequestLogger,
		},
	}

	routes.GET(regexp.MustCompile(`^/$`), LandingHTML)
	routes.GET(regexp.MustCompile(`^/public/.+$`), StaticFiles)
	routes.GET(regexp.MustCompile(`^/long$`), func(c *RequestContext) ResponseData {
		time.Sleep(time.Second * 15)
		return ResponseData{StatusCode: http.StatusNoContent}
	})
	routes.POST(regexp.MustCompile(`^/hijacked$`), func(c *RequestContext) ResponseData {
		hj, ok := c.Res.(http.Hijacker)
		if !ok {
			return ResponseData{StatusCode: http.StatusInternalServerError}
		}
		conn, bufrw := utils.Must2(hj.Hijack())
		done := c.IsLongRunning()

		go func() {
			defer done()
			defer conn.Close()

			go func() {
				<-c.LongRunningRequests.Canceled()
				bufrw.WriteString("HTTP/1.1 200 OK\r\n" +
					"Content-Type: text/plain; charset=UTF-8\r\n" +
					"\r\n" +
					"you have been gracefully terminated 🔫\r\n")
				bufrw.Flush()
				time.Sleep(3 * time.Second)
				bufrw.WriteString("bye :)\r\n")
				bufrw.Flush()
				conn.Close()
				done()
			}()

			s, err := bufrw.ReadString('\n')
			if err != nil {
				bufrw.WriteString("HTTP/1.1 400 Bad Request\r\n" +
					"Content-Type: text/plain; charset=UTF-8\r\n" +
					"\r\n" +
					"you made an error >:(\r\n" +
					err.Error() + "\r\n" +
					"\r\n")
				bufrw.Flush()
				return
			}
			bufrw.WriteString("HTTP/1.1 200 OK\r\n" +
				"Content-Type: text/html; charset=UTF-8\r\n" +
				"\r\n" +
				"<html>\r\n" +
				"I want to get off MR BONES WILD RIDE\r\n" +
				s + "\r\n" +
				"</html>\r\n" +
				"\r\n")
			bufrw.Flush()
		}()

		return ResponseData{
			Proxied: true,
		}
	})
	routes.AnyMethod(regexp.MustCompile(`^.+$`), func(c *RequestContext) ResponseData {
		return render404HTML(c)
	})

	return router
}

func MiddlewareSetLRRTracker(tracker *LongRunningRequestTracker) Middleware {
	return func(h Handler) Handler {
		return func(c *RequestContext) ResponseData {
			c.LongRunningRequests = tracker
			return h(c)
		}
	}
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

// NOTE(asaf): Static files and EsBuild proxying.
func StaticFiles(c *RequestContext) ResponseData {
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
			res.Proxied = true
			return res
		}
	}
	http.StripPrefix("/public/", http.FileServer(http.Dir("public"))).ServeHTTP(&res, c.Req)
	return res
}
