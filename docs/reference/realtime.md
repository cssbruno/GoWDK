# Realtime

`addons/realtime` registers `FeatureRealtime` for browser-facing presentation
event fanout. It does not choose a transport, create routes, or patch the DOM.
Realtime UI reactivity is planned for M14; current support is delivery of
`contracts.PresentationEvent` envelopes to app-owned browser code.

Use it with contract web adapters when commands emit presentation events:

```go
import (
	"github.com/cssbruno/gowdk"
	"github.com/cssbruno/gowdk/addons/contracts"
	"github.com/cssbruno/gowdk/addons/realtime"
)

var Config = gowdk.Config{
	Addons: []gowdk.Addon{
		contracts.Addon(),
		realtime.Addon(),
	},
}
```

`gowdk add realtime` inserts the same config entry.

## Transport Choice

Use SSE first when the browser only needs server-to-client presentation events.
SSE is dependency-free in the root module, uses normal HTTP, and works well for
notifications, progress updates, and invalidation signals:

```go
import (
	"net/http"

	"github.com/cssbruno/gowdk/addons/realtime"
	"github.com/cssbruno/gowdk/runtime/contracts"
)

hub := realtime.NewSSE(realtime.WithSSEBufferSize(32))
http.Handle("/gowdk/events", hub)

gowdkapp.RegisterContractEventSink(
	contracts.PresentationFanoutCommandEventSink(hub),
)
```

The browser receives `event: gowdk-presentation` messages whose `data` field is
a JSON `contracts.EventEnvelope`. Domain and integration events are filtered
out before delivery.

Use WebSocket when the app already needs a persistent bidirectional socket, a
WebSocket-specific deployment path, or protocol-level client messages. The
current GOWDK WebSocket adapter only fans presentation events out to connected
clients; inbound WebSocket commands remain app-owned Go.

WebSocket support is a nested module so `github.com/coder/websocket` does not
enter the root module graph:

```sh
go get github.com/cssbruno/gowdk/runtime/contracts/websocketfanout
```

```go
import (
	"net/http"

	"github.com/coder/websocket"
	"github.com/cssbruno/gowdk/runtime/contracts"
	"github.com/cssbruno/gowdk/runtime/contracts/websocketfanout"
)

hub := websocketfanout.New(websocketfanout.WithAcceptOptions(websocket.AcceptOptions{
	OriginPatterns: []string{"https://example.com"},
}))
http.Handle("/gowdk/events/ws", hub)

gowdkapp.RegisterContractEventSink(
	contracts.PresentationFanoutCommandEventSink(hub),
)
```

Each presentation event is written as one text JSON `contracts.EventEnvelope`.

## Deployment Notes

- In-process SSE and WebSocket hubs only know about clients connected to the
  same process. Multi-instance deployments should pair fanout with a broker,
  outbox, or external pub/sub path when all clients must see the same event.
- SSE responses set `X-Accel-Buffering: no`; reverse proxies may still need
  explicit buffering and timeout settings for long-lived streams.
- WebSocket deployments should set origin checks and proxy upgrade headers.
- Presentation events are untrusted browser output. Do not treat client
  messages or presentation-event names as authorization.

## Verification

```sh
go run ./cmd/gowdk add --list
go test ./runtime/contracts/sse
(cd runtime/contracts/websocketfanout && go test ./...)
```
