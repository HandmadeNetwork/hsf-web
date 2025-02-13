package website

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"hsf/src/logging"
	"io"
	"net/http"
	"net/netip"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/rs/zerolog"
)

/*
 * We use the standard library HTTP server, which performs fine but is pretty
 * anemic in terms of features. We have therefore built a few systems on top to
 * handle:
 *
 * - Routing (matching URLs to handler functions)
 * - Middleware (applying common logic to a group of routes)
 * - Buffering (modifying responses before they are actually sent)
 *
 * Requests and responses are handled by Handler functions, with the signature:
 *
 *   type Handler func(c *RequestContext) ResponseData
 *
 * The RequestContext contains basic data about the request, as well as
 * whatever other custom data you like in your system (e.g. the current user).
 * You are encouraged to modify RequestContext to fit the needs of your
 * own site.
 *
 * ResponseData is simply a struct containing the properties of an HTTP
 * response, e.g. status code, body, and headers. It is compatible with
 * http.ResponseWriter, so it can be used with other libraries if necessary.
 *
 * Routing is performed simply by iterating over a list of regular expressions
 * until one matches the current URL. You might expect this to be slow but it
 * is actually very fast, and has yet to be any sort of bottleneck for us. It
 * also gives much greater flexibility in what kinds of patterns can be matched
 * (as compared to popular choices like trie-based routing), and we can handle
 * path parameters using named capture groups.
 *
 * Middleware is simply wrapping Handlers in other Handlers, with functions of
 * this signature:
 *
 *   type Middleware func(h Handler) Handler
 *
 * Middleware can be used to apply common logic to a group of routes, e.g.
 * authentication, CSRF mitigation, timing attack mitigation, etc.
 */

type RequestContext struct {
	ctx context.Context

	Logger           *zerolog.Logger
	Req              *http.Request
	PathParams       map[string]string
	RequestStartTime time.Time

	// This is the http package's internal response object. Not just a
	// ResponseWriter. We sometimes need the original response object so that
	// some functions of the http package can set connection-management flags
	// on it.
	Res http.ResponseWriter

	// Below this point you may put whatever custom fields you like, to be set by
	// middleware and used throughout your handlers. We include one field as
	// an example.

	LongRunningRequests *LongRunningRequestTracker
}

var _ context.Context = &RequestContext{}

func (c *RequestContext) Deadline() (time.Time, bool) {
	return c.ctx.Deadline()
}

func (c *RequestContext) Done() <-chan struct{} {
	return c.ctx.Done()
}

func (c *RequestContext) Err() error {
	return c.ctx.Err()
}

func (c *RequestContext) Value(key any) any {
	return c.ctx.Value(key)
}

func (c *RequestContext) IsLongRunning() func() {
	c.LongRunningRequests.wg.Add(1)
	doneAlreadyCalled := false
	return func() {
		if doneAlreadyCalled {
			return
		}
		doneAlreadyCalled = true
		c.LongRunningRequests.wg.Done()
	}
}

type ResponseData struct {
	StatusCode int
	Body       *bytes.Buffer

	// Set to true to prevent the HSF system from handling the request and
	// response, e.g. if you are proxying the request to another system
	// like esbuild.
	Proxied bool

	header http.Header
}

var _ http.ResponseWriter = &ResponseData{}

func (rd *ResponseData) Header() http.Header {
	if rd.header == nil {
		rd.header = make(http.Header)
	}

	return rd.header
}

func (rd *ResponseData) Write(p []byte) (n int, err error) {
	if rd.Body == nil {
		rd.Body = new(bytes.Buffer)
	}

	return rd.Body.Write(p)
}

func (rd *ResponseData) WriteHeader(status int) {
	rd.StatusCode = status
}

func (rd *ResponseData) SetCookie(cookie *http.Cookie) {
	rd.Header().Add("Set-Cookie", cookie.String())
}

type Router struct {
	Routes []Route
}

var _ http.Handler = &Router{}

type Route struct {
	Method  string
	Regexes []*regexp.Regexp
	Handler Handler
}

func (r Route) String() string {
	var routeStrings []string
	for _, regex := range r.Regexes {
		routeStrings = append(routeStrings, regex.String())
	}
	return fmt.Sprintf("%s %v", r.Method, routeStrings)
}

type RouteBuilder struct {
	Router      *Router
	Prefixes    []*regexp.Regexp
	Middlewares []Middleware
}

type Handler func(c *RequestContext) ResponseData
type Middleware func(h Handler) Handler

func applyMiddlewares(h Handler, ms []Middleware) Handler {
	result := h
	for i := len(ms) - 1; i >= 0; i-- {
		result = ms[i](result)
	}
	return result
}

func (rb *RouteBuilder) Handle(methods []string, regex *regexp.Regexp, h Handler) {
	// Ensure that this regex matches the start of the string
	regexStr := regex.String()
	if !strings.HasPrefix(regexStr, "^") {
		panic("All routing regexes must begin with '^'")
	}

	if rb.Router == nil {
		rb.Router = new(Router)
	}

	h = applyMiddlewares(h, rb.Middlewares)
	for _, method := range methods {
		rb.Router.Routes = append(rb.Router.Routes, Route{
			Method:  method,
			Regexes: append(rb.Prefixes, regex),
			Handler: h,
		})
	}
}

func (rb *RouteBuilder) AnyMethod(regex *regexp.Regexp, h Handler) {
	rb.Handle([]string{""}, regex, h)
}

func (rb *RouteBuilder) GET(regex *regexp.Regexp, h Handler) {
	rb.Handle([]string{http.MethodGet}, regex, h)
}

func (rb *RouteBuilder) POST(regex *regexp.Regexp, h Handler) {
	rb.Handle([]string{http.MethodPost}, regex, h)
}

func (rb *RouteBuilder) WithMiddleware(ms ...Middleware) RouteBuilder {
	newRb := *rb
	newRb.Middlewares = append(rb.Middlewares, ms...)

	return newRb
}

func (rb *RouteBuilder) Group(regex *regexp.Regexp, ms ...Middleware) RouteBuilder {
	// Ensure that this regex matches the start of the string
	regexStr := regex.String()
	if len(regexStr) == 0 || regexStr[0] != '^' {
		panic("All routing regexes must begin with '^'")
	}

	newRb := *rb
	newRb.Prefixes = append(newRb.Prefixes, regex)
	newRb.Middlewares = append(rb.Middlewares, ms...)

	return newRb
}

func (r *Router) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	method := req.Method
	if method == http.MethodHead {
		method = http.MethodGet // HEADs map to GETs for the purposes of routing
	}

nextroute:
	for _, route := range r.Routes {
		if route.Method != "" && method != route.Method {
			continue
		}

		currentPath := strings.TrimSuffix(req.URL.Path, "/")
		if currentPath == "" {
			currentPath = "/"
		}

		var params map[string]string
		for _, regex := range route.Regexes {
			match := regex.FindStringSubmatch(currentPath)
			if len(match) == 0 {
				continue nextroute
			}

			if params == nil {
				params = map[string]string{}
			}
			subexpNames := regex.SubexpNames()
			for i, paramValue := range match {
				paramName := subexpNames[i]
				if paramName == "" {
					continue
				}
				if _, alreadyExists := params[paramName]; alreadyExists {
					logging.Warn().
						Str("Url", ReqFullUrl(req)).
						Str("Route", route.String()).
						Str("paramName", paramName).
						Msg("Duplicate names for path parameters; last one wins")
				}
				params[paramName] = paramValue
			}

			// Make sure that we never consume trailing slashes even if the route regex matches them
			toConsume := strings.TrimSuffix(match[0], "/")
			currentPath = currentPath[len(toConsume):]
			if currentPath == "" {
				currentPath = "/"
			}
		}

		c := &RequestContext{
			Logger:           logging.GlobalLogger(),
			Req:              req,
			Res:              rw,
			PathParams:       params,
			RequestStartTime: time.Now(),

			ctx: req.Context(),
		}

		doRequest(rw, c, route.Handler)

		return
	}

	panic(fmt.Sprintf("Path '%s' did not match any routes! Make sure to register a wildcard route to act as a 404.", req.URL))
}

func doRequest(rw http.ResponseWriter, c *RequestContext, h Handler) {
	defer func() {
		// This panic recovery is the last resort. If you want to render
		// an error page or something, make it a request wrapper.
		if recovered := recover(); recovered != nil {
			rw.WriteHeader(http.StatusInternalServerError)
			logging.LogPanicValue(c.Logger, recovered, "request panicked and was not handled")
			rw.Write([]byte("There was a problem handling your request."))
		}
	}()

	// Run the chosen handler
	res := h(c)

	if res.Proxied {
		// NOTE(asaf): In case we forward the request/response to another handler
		//             (like esbuild).
		return
	}

	if res.StatusCode == 0 {
		res.StatusCode = http.StatusOK
	}

	// Set Content-Type and Content-Length if necessary. This behavior would in
	// some cases be handled by http.ResponseWriter.Write, but we extract it so
	// that HEAD requests always return both headers.

	var preamble []byte // Any bytes we read to determine Content-Type
	if res.Body != nil {
		bodyLen := res.Body.Len()

		if res.Header().Get("Content-Type") == "" {
			preamble = res.Body.Next(512)
			rw.Header().Set("Content-Type", http.DetectContentType(preamble))
		}
		if res.Header().Get("Content-Length") == "" {
			rw.Header().Set("Content-Length", strconv.Itoa(bodyLen))
		}
	}

	// Ensure we send no body for HEAD requests
	if c.Req.Method == http.MethodHead {
		res.Body = nil
	}

	// Send remaining response headers
	for name, vals := range res.Header() {
		for _, val := range vals {
			rw.Header().Add(name, val)
		}
	}
	rw.WriteHeader(res.StatusCode)

	// Send response body
	if res.Body != nil {
		// Write preamble, if any
		_, err := rw.Write(preamble)
		if errors.Is(err, syscall.EPIPE) {
			// NOTE(asaf): Can be triggered when other side hangs up
			logging.Debug().Msg("Broken pipe")
		} else if err != nil {
			logging.Error().Err(err).Msg("Failed to write response preamble")
		}

		// Write remainder of body
		_, err = io.Copy(rw, res.Body)
		if errors.Is(err, syscall.EPIPE) {
			// NOTE(asaf): Can be triggered when other side hangs up
			logging.Debug().Msg("Broken pipe")
		} else if err != nil {
			logging.Error().Err(err).Msg("copied res.Body")
		}
	}
}

// Reverse-proxy-aware full url
func ReqFullUrl(req *http.Request) string {
	var scheme string

	if scheme == "" {
		proto, hasProto := req.Header["X-Forwarded-Proto"]
		if hasProto {
			scheme = fmt.Sprintf("%s://", proto[0])
		}
	}

	if scheme == "" {
		if req.TLS != nil {
			scheme = "https://"
		} else {
			scheme = "http://"
		}
	}

	return scheme + req.Host + req.URL.String()
}

// NOTE(asaf): Assumes port is present (it should be for RemoteAddr according to the docs)
var ipRegex = regexp.MustCompile(`^(\[(?P<addrv6>[^\]]+)\]:\d+)|((?P<addrv4>[^:]+):\d+)$`)

// Reverse-proxy-aware user's IP
func ReqGetIP(req *http.Request) *netip.Prefix {
	ipString := ""

	if ipString == "" {
		cf, hasCf := req.Header["CF-Connecting-IP"]
		if hasCf {
			ipString = cf[0]
		}
	}

	if ipString == "" {
		forwarded, hasForwarded := req.Header["X-Forwarded-For"]
		if hasForwarded {
			ipString = forwarded[0]
		}
	}

	if ipString == "" {
		ipString = req.RemoteAddr
		if ipString != "" {
			matches := ipRegex.FindStringSubmatch(ipString)
			if matches != nil {
				v4 := matches[ipRegex.SubexpIndex("addrv4")]
				v6 := matches[ipRegex.SubexpIndex("addrv6")]
				if v4 != "" {
					ipString = v4
				} else {
					ipString = v6
				}
			}
		}
	}

	if ipString != "" {
		res, err := netip.ParsePrefix(fmt.Sprintf("%s/32", ipString))
		if err == nil {
			return &res
		}
	}

	return nil
}

type LongRunningRequestTracker struct {
	ctx    context.Context
	cancel func()

	wg sync.WaitGroup
}

func NewLongRunningRequestTracker() *LongRunningRequestTracker {
	ctx, cancel := context.WithCancel(context.Background())
	return &LongRunningRequestTracker{
		ctx:    ctx,
		cancel: cancel,
	}
}

func (t *LongRunningRequestTracker) Cancel() {
	t.cancel()
}

func (t *LongRunningRequestTracker) Canceled() <-chan struct{} {
	return t.ctx.Done()
}

func (t *LongRunningRequestTracker) Wait(timeout time.Duration) {
	done := make(chan struct{})
	go func() {
		t.wg.Wait()
		done <- struct{}{}
	}()
	timer := time.NewTimer(timeout)
	select {
	case <-timer.C:
		logging.Warn().Msg("long-running requests failed to shut down in time")
	case <-done:
	}
}
