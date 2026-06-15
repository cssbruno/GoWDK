# Realtime

`addons/realtime` registers `FeatureRealtime` for browser-facing presentation
event fanout. Current support covers delivery of
`contracts.PresentationEvent` envelopes to browser clients, compiler metadata
for `g:subscribe` query regions, generated SSE fanout for bound subscriptions,
and bounded generated client patches for query-owned regions.

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

## `.gwdk` Subscriptions

Use `g:subscribe` on the same element as `g:query` to declare that a query-owned
region should listen for a presentation event:

```gwdk
view {
  <section
    g:query="patients.GetPatientPage"
    g:subscribe="patients.PatientNotice">
    <h1>Patients</h1>
  </section>
}
```

Current behavior:

- Requires `realtime.Addon()` in `gowdk.config.go`.
- Adds a subscription record to `internal/gwdkir.Program.RealtimeSubscriptions`.
- Validates the event reference against scanned `runtime/contracts`
  presentation-event registrations.
- Rejects unknown events, invalid event registrations, domain/integration event
  categories, and registrations that are not available to the `web` role.
- Renders `data-gowdk-query`, `data-gowdk-subscribe`, and validated
  `data-gowdk-subscribe-type` markers for compiler-owned runtime hookup.
- Adds `realtime_subscription` events to `gowdk-build-report.json` when present.
- Generated apps with bound subscriptions mount a dependency-free SSE stream at
  `/_gowdk/realtime/events`.
- Generated command adapters dispatch command-emitted presentation events to a
  subscription-filtered fanout, so only explicitly subscribed event types are
  streamed.
- Generated stream handlers run inherited subscribed-page guards before opening
  an SSE response. They choose page guards from `?path=...` or the request
  referer path when available; if no page path can be identified, guarded
  subscriptions fail closed by requiring the union of subscribed guard IDs.
- Generated `gowdk.js` connects pages with subscribed regions to the SSE stream
  and applies explicit `replaceHTML` patches to matching query-owned elements.
- Generated apps expose `RealtimeEventsPath` and
  `RegisterRealtimeFanout(realtime.PresentationFanout)` for app-owned server
  setup.

Current limits:

- `g:subscribe` does not implicitly derive domain-event invalidation. Domain
  and integration events stay backend-owned.
- Subscriptions must be query-bounded; `g:subscribe` without `g:query` is
  rejected.
- Only explicit `replaceHTML` patches are supported in the generated client
  runtime; richer patch shapes and query refetch policy are deferred.
- Custom retry/backoff/replay, active server-side session-change stream
  revocation, richer patch shapes, and query refetch policy remain follow-up
  work.

## Live Example

`examples/contracts/patients.page.gwdk` demonstrates the current live-update
contract:

- `.gwdk` owns `g:query="patients.GetPatientPage"` and
  `g:subscribe="patients.PatientNotice"` on the same region.
- User Go owns the command, query, presentation-event registration, and the
  server-generated `replaceHTML` patch payload.
- Generated Go owns the command/query web adapters, subscription-filtered SSE
  stream, inherited guard checks, and command event sink composition.
- Generated `gowdk.js` owns the EventSource connection and applies the patch to
  the subscribed query region.

Build and run it:

```sh
go run ./cmd/gowdk build --config examples/contracts/gowdk.config.go --out /tmp/gowdk-contracts-build --app /tmp/gowdk-contracts-app --bin /tmp/gowdk-contracts-site examples/contracts/patients.page.gwdk
/tmp/gowdk-contracts-site
```

Open `http://127.0.0.1:8080/contracts/patients`. With JavaScript enabled, the
generated runtime opens `/_gowdk/realtime/events`; submitting the form runs the
Go command, emits `patients.PatientNotice`, and replaces the subscribed status
region with the patch HTML from user Go. Without JavaScript, the page still
renders the static query region and the form posts to the generated command
endpoint.

Useful smoke checks:

```sh
test -f /tmp/gowdk-contracts-build/assets/gowdk/gowdk.js
grep -F '"kind": "realtime_subscription"' /tmp/gowdk-contracts-build/gowdk-build-report.json
grep -F '"event": "patients.PatientNotice"' /tmp/gowdk-contracts-build/gowdk-build-report.json
grep -F 'data-gowdk-subscribe-type=' /tmp/gowdk-contracts-build/contracts/patients/index.html
go test ./examples/contracts/patients
```

## Client Patch Payloads

Generated clients consume `gowdk-presentation` SSE messages whose envelope
`Value` contains either one `patch` object or a `patches` array:

```json
{
  "patch": {
    "op": "replaceHTML",
    "html": "<p>Updated</p>",
    "swap": "innerHTML"
  }
}
```

Supported patch fields:

- `op`: must be `replaceHTML`.
- `html`: replacement HTML string.
- `swap`: optional, `innerHTML` by default; `outerHTML` is also accepted.

The browser runtime applies patches only to regions whose validated
`data-gowdk-subscribe-type` matches the presentation event type. Unsupported
patch operations, missing HTML, malformed payloads, and unsupported swaps emit
`gowdk:realtime-error` and leave the DOM unchanged.

## Stream Failure And Backpressure

Generated streams are ordinary same-origin SSE responses. Before the response
is opened, generated code runs the guard IDs inherited by subscribed page
regions. Guard failures return the existing no-store guard failure response
instead of an SSE stream, so browsers do not receive protected events after
access is denied.

The dependency-free SSE adapter sends `retry: 1000`, so browser EventSource
clients use a one-second reconnect delay for ordinary transport failures. GOWDK
does not add custom browser retry state in this slice.

Each SSE client has a bounded queue (`16` messages by default, configurable
with `realtime.WithSSEBufferSize`). When a client queue is full, new events for
that client are dropped instead of blocking command execution or other clients.
Use a broker/outbox/replay path for applications that require guaranteed
delivery after disconnects or slow clients.

Active server-side session changes are enforced on the next stream open or
reconnect. Immediate revocation of already-open streams remains app-owned or
future GOWDK runtime work.

## Transport Choice

Generated apps use SSE first when the browser only needs server-to-client
presentation events. SSE is dependency-free in the root module, uses normal
HTTP, and works well for notifications, progress updates, and invalidation
signals.

For app-owned routes or custom setup outside generated subscriptions:

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
go test ./internal/appgen -run 'TestGenerateGuardsRealtimeStreamForSubscribedPages|TestGeneratedBinaryRealtimeStreamGuardDenialClosesStream'
(cd runtime/contracts/websocketfanout && go test ./...)
```
