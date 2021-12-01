package main

import (
	"fmt"
	"github.com/getsentry/sentry-go"
	sentryatreugo "github.com/getsentry/sentry-go/atreugo"
	"github.com/savsgio/atreugo/v11"
	"net/http"
	"time"
)

func enhanceSentryEvent(handler atreugo.View) atreugo.View {
	return func(ctx *atreugo.RequestCtx) error {
		if hub := sentryatreugo.GetHubFromContext(ctx); hub != nil {
			hub.Scope().SetTag("someRandomTag", "maybeYouNeedIt")
		}
		return handler(ctx)
	}
}

func main() {
	err := sentry.Init(sentry.ClientOptions{
		Dsn: "",
		BeforeSend: func(event *sentry.Event, hint *sentry.EventHint) *sentry.Event {
			if hint.Context != nil {
				if ctx, ok := hint.Context.Value(sentry.RequestContextKey).(*atreugo.RequestCtx); ok {
					// You have access to the original Context if it panicked
					fmt.Println(string(ctx.Request.Host()))
				}
			}
			fmt.Println(event)
			return event
		},
		Debug:            true,
		AttachStacktrace: true,
		TracesSampleRate: 1.0,
	})
	if err != nil {
		fmt.Println(err)
	}

	defer sentry.Flush(2 * time.Second)

	// Later in the code
	sentryHandler := sentryatreugo.New(sentryatreugo.Options{
		Repanic:         false,
		WaitForDelivery: true,
		Tracer:          true,
	})

	server := atreugo.New(atreugo.Config{Addr: "0.0.0.0:3000", PanicView: func(ctx *atreugo.RequestCtx, err interface{}) {
		defer sentryHandler.HandlePanic(ctx, err)
		ctx.SetStatusCode(http.StatusInternalServerError)
	}})
	server.UseBefore(sentryHandler.Handle).UseAfter(sentryHandler.HandleTracer)

	server.GET("/", func(ctx *atreugo.RequestCtx) error {
		panic("hello panic")
	})

	server.GET("/echo/{path:*}", enhanceSentryEvent(func(ctx *atreugo.RequestCtx) error {
		return ctx.TextResponse("Echo message: " + ctx.UserValue("path").(string))
	}))

	v1 := server.NewGroupPath("/v1")
	v1.GET("/", func(ctx *atreugo.RequestCtx) error {
		return ctx.TextResponse("Hello V1 Group")
	})

	if err := server.ListenAndServe(); err != nil {
		panic(err)
	}
}
