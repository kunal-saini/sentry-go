package atreugo

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/savsgio/atreugo/v11"
)

type contextKey int

const ContextKey = contextKey(1)
const valuesKey = "sentry"
const tracerKey = "sentry_tracer"

type Handler struct {
	repanic         bool
	waitForDelivery bool
	timeout         time.Duration
	tracer          bool
}

type Options struct {
	// Repanic configures whether Sentry should repanic after recovery
	Repanic bool
	// WaitForDelivery configures whether you want to block the request before moving forward with the response.
	WaitForDelivery bool
	// Timeout for the event delivery requests.
	Timeout time.Duration
	Tracer  bool
}

// New returns a struct that provides Handle method
func New(options Options) *Handler {
	timeout := options.Timeout
	if timeout == 0 {
		timeout = 2 * time.Second
	}
	return &Handler{
		repanic:         options.Repanic,
		timeout:         timeout,
		waitForDelivery: options.WaitForDelivery,
		tracer:          options.Tracer,
	}
}

func (h *Handler) Handle(ctx *atreugo.RequestCtx) error {
	hub := sentry.CurrentHub().Clone()
	r := convert(ctx)
	httpRequestCtx := r.Context()
	httpRequestCtx = sentry.SetHubOnContext(httpRequestCtx, hub)
	if h.tracer {
		span := sentry.StartSpan(httpRequestCtx, "atreugo.server",
			sentry.TransactionName(fmt.Sprintf("%s %s", r.Method, r.URL.Path)),
			sentry.ContinueFromRequest(r),
		)
		r = r.WithContext(span.Context())
		hub.Scope().SetRequest(r)
		ctx.SetUserValue(tracerKey, span)
	}

	hub.Scope().SetRequest(r)
	hub.Scope().SetRequestBody(ctx.Request.Body())
	ctx.SetUserValue(valuesKey, hub)
	return ctx.Next()
}

func (h *Handler) HandleTracer(ctx *atreugo.RequestCtx) error {
	if tracerSpan := GetTracerFromContext(ctx); tracerSpan != nil {
		defer tracerSpan.Finish()
	}
	return ctx.Next()
}

func (h *Handler) HandlePanic(ctx *atreugo.RequestCtx, err interface{}) {
	hub := GetHubFromContext(ctx)
	eventID := hub.RecoverWithContext(
		context.WithValue(context.Background(), sentry.RequestContextKey, ctx),
		err,
	)
	if eventID != nil && h.waitForDelivery {
		hub.Flush(h.timeout)
	}
	if h.repanic {
		panic(err)
	}
}

func GetHubFromContext(ctx *atreugo.RequestCtx) *sentry.Hub {
	hub := ctx.UserValue(valuesKey)
	if hub, ok := hub.(*sentry.Hub); ok {
		return hub
	}
	return nil
}

func GetTracerFromContext(ctx *atreugo.RequestCtx) *sentry.Span {
	tracer := ctx.UserValue(tracerKey)
	if tracer, ok := tracer.(*sentry.Span); ok {
		return tracer
	}
	return nil
}

func convert(ctx *atreugo.RequestCtx) *http.Request {
	defer func() {
		if err := recover(); err != nil {
			sentry.Logger.Printf("%v", err)
		}
	}()

	r := new(http.Request)

	r.Method = string(ctx.Method())
	uri := ctx.URI()
	// Ignore error.
	r.URL, _ = url.Parse(fmt.Sprintf("%s://%s%s", uri.Scheme(), uri.Host(), uri.Path()))

	// Headers
	r.Header = make(http.Header)
	r.Header.Add("Host", string(ctx.Host()))
	ctx.Request.Header.VisitAll(func(key, value []byte) {
		r.Header.Add(string(key), string(value))
	})
	r.Host = string(ctx.Host())

	// Cookies
	ctx.Request.Header.VisitAllCookie(func(key, value []byte) {
		r.AddCookie(&http.Cookie{Name: string(key), Value: string(value)})
	})

	// Env
	r.RemoteAddr = ctx.RemoteAddr().String()

	// QueryString
	r.URL.RawQuery = string(ctx.URI().QueryString())

	// Body
	r.Body = ioutil.NopCloser(bytes.NewReader(ctx.Request.Body()))

	return r
}
