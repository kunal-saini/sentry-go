<p align="center">
  <a href="https://sentry.io" target="_blank" align="center">
    <img src="https://sentry-brand.storage.googleapis.com/sentry-logo-black.png" width="280">
  </a>
  <br />
</p>

# Official Sentry Atreugo Handler for Sentry-go SDK

**Godoc:** https://godoc.org/github.com/getsentry/sentry-go/atreugo

**Example:** https://github.com/getsentry/sentry-go/tree/master/example/atreugo

## Installation

```sh
go get github.com/getsentry/sentry-go/atreugo
```

```go
import (
	"fmt"
	"net/http"

	"github.com/getsentry/sentry-go"
    sentryatreugo "github.com/getsentry/sentry-go/atreugo"
)

// To initialize Sentry's handler, you need to initialize Sentry itself beforehand
if err := sentry.Init(sentry.ClientOptions{
	Dsn: "your-public-dsn",
}); err != nil {
	fmt.Printf("Sentry initialization failed: %v\n", err)
}

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

server.GET("/echo/{path:*}", func(ctx *atreugo.RequestCtx) error {
    return ctx.TextResponse("Echo message: " + ctx.UserValue("path").(string))
})

fmt.Println("Listening and serving HTTP on :3000")

if err := server.ListenAndServe(); err != nil {
    panic(err)
}
```

## Configuration

`sentryatreugo` accepts a struct of `Options` that allows you to configure how the handler will behave.

Currently, it respects 4 options:

```go
Repanic bool
WaitForDelivery bool
// Timeout for the event delivery requests.
Timeout time.Duration
// Tracer allows you to enable request tracing
Tracer bool
```

## Usage

`sentryatreugo` attaches an instance of `*sentry.Hub` (https://godoc.org/github.com/getsentry/sentry-go#Hub) to the request's context, which makes it available throughout the rest of the request's lifetime.
You can access it by using the `sentryatreugo.GetHubFromContext()` method on the context itself in any of your proceeding middleware and routes.
And it should be used instead of the global `sentry.CaptureMessage`, `sentry.CaptureException`, or any other calls, as it keeps the separation of data between the requests.

**Keep in mind that `*sentry.Hub` won't be available in middleware attached before to `sentryatreugo`!**

```go
func enhanceSentryEvent(handler atreugo.View) atreugo.View {
    return func(ctx *atreugo.RequestCtx) error {
        if hub := sentryatreugo.GetHubFromContext(ctx); hub != nil {
            hub.Scope().SetTag("someRandomTag", "maybeYouNeedIt")
        }
        return handler(ctx)
    }
}

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

server.GET("/", enhanceSentryEvent(func(ctx *atreugo.RequestCtx) error {
    panic("hello panic")
}))

server.GET("/echo/{path:*}", enhanceSentryEvent(func(ctx *atreugo.RequestCtx) error {
    return ctx.TextResponse("Echo message: " + ctx.UserValue("path").(string))
}))

fmt.Println("Listening and serving HTTP on :3000")

if err := server.ListenAndServe(); err != nil {
    panic(err)
}
```

### Accessing Context in `BeforeSend` callback

```go
sentry.Init(sentry.ClientOptions{
	Dsn: "your-public-dsn",
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
})
```
