# Addons Reference

Addons currently register feature IDs with the compiler. The CSS processor
contract can run during SPA builds; other addon packages do not yet execute
full generated behavior.

Current feature IDs:

- `spa`
- `actions`
- `partial`
- `ssr`
- `api`
- `embed`
- `css`
- `ratelimit`

Current packages:

- `addons/spa`
- `addons/actions`
- `addons/partial`
- `addons/ssr`
- `addons/api`
- `addons/embed`
- `addons/css`
- `addons/tailwind`
- `addons/ratelimit`

The current compiler validator checks whether SSR is enabled when a page uses
`@render ssr` or `@render hybrid`. SPA builds also invoke addons that
implement `gowdk.CSSProcessor`.

`addons/tailwind` is an experimental Tailwind v4 CSS processor wrapper around a
user-provided standalone CLI executable. It does not use npm, download Tailwind
automatically, add Tailwind to GOWDK core, or generate Tailwind v3 content
configuration.

`addons/ratelimit` provides request-time HTTP middleware with fixed-window
decisions, rate-limit response headers, a process-local in-memory store, and a
Redis-backed store adapter. It does not add a Redis client dependency or wire
limits into generated handlers automatically.

```go
store := ratelimit.NewInMemoryStore(ratelimit.InMemoryOptions{})
limiter, err := ratelimit.New(ratelimit.Options{
	Limit:  60,
	Window: time.Minute,
	Store:  store,
})
if err != nil {
	return err
}

handler := limiter.Middleware(next)
```

Distributed deployments can use `ratelimit.NewRedisStore` with a small
`RedisClient` adapter:

```go
redisStore, err := ratelimit.NewRedisStore(ratelimit.RedisOptions{
	Client: redisClientAdapter,
})
if err != nil {
	return err
}

limiter, err := ratelimit.New(ratelimit.Options{
	Limit:  300,
	Window: time.Minute,
	Store:  redisStore,
})
```
