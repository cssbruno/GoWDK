# HTTP response guards

## Development reverse proxy

Live-reload injection inspects at most 4 MiB of an eligible uncompressed HTML
response. A response with a declared `Content-Length` above that limit is
forwarded without reading its body. For chunked or unknown-length responses, the
proxy reads at most 4 MiB plus one byte; when the limit is exceeded, the buffered
prefix is replayed before the untouched backend stream.

Skipped responses retain their original status, headers, trailers, content
length, and close behavior. The development reload broker emits a
`proxy-injection-skipped` event containing only the reason, status, limit, and
declared byte count. Runtime-error events still fire for oversized 5xx HTML, but
the initial in-page overlay is not injected.

This limit applies only to the development reverse proxy. Generated production
applications are unchanged.

## CORS cache selectors

Successful preflight responses always vary on
`Access-Control-Request-Method` and `Access-Control-Request-Headers`. Policies
that reflect a specific allowed origin also vary on `Origin`; wildcard-origin
policies do not.

The runtime merges these selectors with every existing `Vary` header line and
comma-separated token, deduplicates names case-insensitively, and preserves
application-provided selectors. `Vary: *` is terminal and prevents additions.
Actual wildcard-origin responses continue to omit `Vary: Origin`.
