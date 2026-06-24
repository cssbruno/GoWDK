# Realtime

`addons/realtime` registers `FeatureRealtime` for browser-facing presentation
event fanout. Current support covers delivery of
`contracts.PresentationEvent` envelopes to browser clients, compiler metadata
for `g:subscribe` query regions, generated SSE fanout for bound subscriptions,
bounded generated client patches for query-owned regions, explicit
domain-event to query invalidation refresh, configurable SSE reconnect/replay
policy, and audience-based active stream revocation.

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
- Presentation event envelopes can carry normalized `Audience` labels.
  Dependency-free SSE fanout can assign server-owned labels to each client and
  only deliver scoped events to clients that match every event label.
- Generated stream handlers run inherited subscribed-page guards before opening
  an SSE response. They choose page guards from `?path=...` or the request
  referer path when available; if no page path can be identified, guarded
  subscriptions fail closed by requiring the union of subscribed guard IDs.
- Generated `gowdk.js` connects pages with subscribed or invalidated query
  regions to the SSE stream with `?path=<current path>` so guarded stream
  reconnects use the current route rather than relying on the referer fallback.
  It applies explicit versioned `replaceHTML` patches to matching query-owned
  elements.
- Generated apps expose `RealtimeEventsPath` and
  `RegisterRealtimeFanout(realtime.PresentationFanout)` so app startup code can
  replace the default fanout with an audience-scoped hub or another transport.

Current limits:

- Subscriptions must be query-bounded; `g:subscribe` without `g:query` is
  rejected.
- Only explicit `replaceHTML` patches are supported in the generated client
  runtime; richer patch shapes are deferred.
- Fragment/API-specific query execution remains follow-up work. Query
  invalidation refresh first asks the generated route/query refresh endpoint
  for standalone region patches when eligible renderers exist, then falls back
  to refetching the current document for any remaining query regions.

## Query Invalidations

Use `contracts.RegisterInvalidation[event, query]` in Go when a backend-owned
domain event should refresh query-owned regions:

```go
func Register(registry *contracts.Registry) {
  contracts.RegisterQuery[GetPatientPage, PatientPageData](registry, LoadPatientPage, contracts.RoleWeb)
  contracts.RegisterCommand[CreatePatient, CreatePatientResult](registry, HandleCreatePatient, contracts.RoleWeb)
  contracts.RegisterDomainEvent[PatientCreated](registry, SendWelcomeEmail, contracts.RoleWorker)
  contracts.RegisterInvalidation[PatientCreated, GetPatientPage](registry)
}
```

Current behavior:

- Requires `realtime.Addon()` when the invalidated query is bound by `.gwdk`.
- Scans invalidation edges beside normal contract registrations.
- Rejects edges that name an unknown query, an unknown domain event, or a
  domain event no scanned command emits.
- Joins edges with bound `g:query` references into
  `Program.QueryInvalidations`.
- Adds `query_invalidation` events to `gowdk-build-report.json`.
- Prints invalidation edges in `gowdk graph`.
- Renders validated `data-gowdk-query-type` markers.
- Generated command adapters send a `gowdk.query.invalidate` presentation event
  after successful command event dispatch when captured domain events invalidate
  bound queries.
- Generated apps with eligible standalone public SSR/hybrid query regions mount
  `/_gowdk/realtime/query-refresh`. Generated `gowdk.js` calls it with the
  current route path and invalidated query types, applies any returned
  `{query, html}` patches, and refetches the current document only for query
  regions that were not patched. Regions with `g:subscribe` are left to
  explicit presentation patches so a refresh does not overwrite a patch.

The generated invalidation event value is:

```json
{
  "version": 1,
  "queries": ["github.com/acme/clinic/patients.GetPatientPage"],
  "events": ["domain:github.com/acme/clinic/patients.PatientCreated"],
  "eventIDs": ["01H..."]
}
```

## Live Example

`examples/contracts/patients.page.gwdk` demonstrates the current live-update
contract:

- `.gwdk` owns `g:query="patients.GetPatientPage"` and
  `g:subscribe="patients.PatientNotice"` on the live region, plus a
  non-subscribed query region refreshed through invalidation.
- User Go owns the command, query, presentation-event registration, and the
  server-generated `replaceHTML` patch payload. It also registers
  `RegisterInvalidation[PatientCreated, GetPatientPage]`.
- Generated Go owns the command/query web adapters, subscription-filtered SSE
  stream, invalidation presentation events, inherited guard checks, and command
  event sink composition.
- Generated `gowdk.js` owns the EventSource connection and applies the patch to
  the subscribed query region or refetches invalidated query regions.

Build and run it:

```sh
go run ./cmd/gowdk build --config examples/contracts/gowdk.config.go --out /tmp/gowdk-contracts-build --app /tmp/gowdk-contracts-app --bin /tmp/gowdk-contracts-site examples/contracts/patients.page.gwdk
/tmp/gowdk-contracts-site
```

Open `http://127.0.0.1:8080/contracts/patients`. With JavaScript enabled, the
generated runtime opens `/_gowdk/realtime/events`; submitting the form runs the
Go command, emits `patients.PatientNotice`, and replaces the subscribed status
region with the patch HTML from user Go. The same command emits
`patients.PatientCreated`, which triggers a generated query invalidation event
for the non-subscribed query region. The browser tries
`/_gowdk/realtime/query-refresh` for targeted region patches before falling back
to a current-document refetch. Without JavaScript, the page still renders the
static query regions and the form posts to the generated command endpoint.

Useful smoke checks:

```sh
test -f /tmp/gowdk-contracts-build/assets/gowdk/gowdk.js
grep -F '"kind": "realtime_subscription"' /tmp/gowdk-contracts-build/gowdk-build-report.json
grep -F '"kind": "query_invalidation"' /tmp/gowdk-contracts-build/gowdk-build-report.json
grep -F '"event": "patients.PatientNotice"' /tmp/gowdk-contracts-build/gowdk-build-report.json
grep -F 'data-gowdk-subscribe-type=' /tmp/gowdk-contracts-build/contracts/patients/index.html
grep -F 'data-gowdk-query-type=' /tmp/gowdk-contracts-build/contracts/patients/index.html
go test ./examples/contracts/patients
```

## Client Patch Payloads

Generated clients consume `gowdk-presentation` SSE messages whose envelope
`Value` contains either one `patch` object or a `patches` array:

```json
{
  "version": 1,
  "patch": {
    "op": "replaceHTML",
    "html": "<p>Updated</p>",
    "swap": "innerHTML"
  }
}
```

Supported patch fields:

- `version`: optional; when present, must be `1`. It can appear on the value
  envelope or each patch object.
- `op`: must be `replaceHTML`.
- `html`: replacement HTML string.
- `swap`: optional, `innerHTML` by default; `outerHTML` is also accepted.

The browser runtime applies patches only to regions whose validated
`data-gowdk-subscribe-type` matches the presentation event type. Unsupported
patch operations, missing HTML, malformed payloads, and unsupported swaps emit
`gowdk:realtime-error` and leave the DOM unchanged.

## Audience Scoping

Audience scoping is server-owned. Browser query parameters, event names, or
client-provided labels are not trusted authorization input.

To emit a user- or tenant-scoped presentation event from a command handler:

```go
return result, contracts.EmitPresentationForAudience(
	ctx,
	PatientNotice{ID: patientID},
	"tenant:"+tenantID,
	"user:"+userID,
)
```

To let generated streams deliver those events, replace the generated default
fanout during app startup with an SSE hub that derives labels from authenticated
request state:

```go
hub := realtime.NewSSE(
	realtime.WithSSEAudienceFromRequest(func(request *http.Request) []string {
		tenantID, userID := tenantAndUserFromSession(request)
		if tenantID == "" || userID == "" {
			return nil
		}
		sessionID := sessionIDFromRequest(request)
		return []string{"tenant:" + tenantID, "user:" + userID, "session:" + sessionID}
	}),
)

gowdkapp.RegisterRealtimeFanout(hub)
```

`tenantAndUserFromSession` and `sessionIDFromRequest` are app-owned code. Empty
event audience means broadcast to every guard-authorized and
subscription-matched client. Non-empty event audience uses AND matching: every
event label must be present in the client label set. Clients with no audience
labels receive broadcast events only.

To force already-open streams to reconnect after a session change, revoke the
server-owned audience label:

```go
hub.RevokeAudience("session:" + sessionID)
```

The next browser reconnect re-runs generated guards and audience assignment
before any new presentation events are delivered.

## Stream Failure And Backpressure

Generated streams are ordinary same-origin SSE responses. Before the response
is opened, generated code runs the guard IDs inherited by subscribed page
regions. Guard failures return the existing no-store guard failure response
instead of an SSE stream, so browsers do not receive protected events after
access is denied.

The dependency-free SSE adapter sends `retry: 1000` by default, so browser
EventSource clients use a one-second reconnect delay for ordinary transport
failures. Set `realtime.WithSSERetryMillis(ms)` when an app needs a different
server-advertised reconnect delay.

Set `realtime.WithSSEReplayLimit(n)` to keep the last `n` presentation events
in memory. The hub writes SSE `id:` lines from `contracts.EventEnvelope.ID`, and
browsers reconnect with `Last-Event-ID`; the hub then replays only later events
whose audience labels still match the reconnecting request. This is an
in-memory convenience for short disconnects, not durable delivery across
process restarts.

`hub.Stats()` returns process-local counters for current clients, slow-client
drops, audience revocations, replayed events, and replay misses. Export these
through app-owned metrics infrastructure when production operations need them.

Each SSE client has a bounded queue (`16` messages by default, configurable
with `realtime.WithSSEBufferSize`). When a client queue is full, new events for
that client are dropped instead of blocking command execution or other clients.
Use a broker/outbox/replay path for applications that require guaranteed
delivery after disconnects or slow clients.

Active server-side session changes are enforced on the next stream open or
reconnect. Use `RevokeAudience` with a session label to disconnect already-open
streams immediately.

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

hub := realtime.NewSSE(
	realtime.WithSSEBufferSize(32),
	realtime.WithSSERetryMillis(2000),
	realtime.WithSSEReplayLimit(128),
	realtime.WithSSEAudienceFromRequest(func(request *http.Request) []string {
		tenantID, userID := tenantAndUserFromSession(request)
		if tenantID == "" || userID == "" {
			return nil
		}
		return []string{"tenant:" + tenantID, "user:" + userID}
	}),
)
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
- Generated realtime streams are guard-checked and subscription/type filtered.
  Add `WithSSEAudienceFromRequest` plus `EmitPresentationForAudience` for
  user-, session-, or tenant-specific presentation payloads. Without an
  audience-scoped fanout, scoped events are not delivered to generated SSE
  clients.
- `WithSSEReplayLimit` is process-local. Multi-instance deployments still need
  a broker/outbox path for durable cross-process replay.
- SSE responses set `X-Accel-Buffering: no`; reverse proxies may still need
  explicit buffering and timeout settings for long-lived streams.
- WebSocket deployments should set origin checks and proxy upgrade headers.
- Presentation events are untrusted browser output. Do not treat client
  messages or presentation-event names as authorization.

## Verification

```sh
go run ./cmd/gowdk add --list
go run ./cmd/gowdk add --list --registry
go test ./runtime/contracts/sse
go test ./addons/realtime
go test ./internal/appgen -run 'TestGenerateGuardsRealtimeStreamForSubscribedPages|TestGeneratedBinaryRealtimeStreamGuardDenialClosesStream'
(cd runtime/contracts/websocketfanout && go test ./...)
```
