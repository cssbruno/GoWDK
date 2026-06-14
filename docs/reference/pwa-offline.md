# PWA And Offline

GOWDK does not emit a service worker, web app manifest, offline page, or
install prompt by default. PWA behavior is app-owned and opt-in because hidden
offline caching can break generated actions, APIs, fragments, SSR pages, auth,
CSRF, and cache policy.

## Current Contract

- Static SPA output can be cached by a user-owned service worker.
- Generated request-time routes keep their normal HTTP cache headers. Do not
  override `no-store` action, API, fragment, SSR, auth, or error responses.
- `Build.Scripts` can register a global script tag, but it does not copy the
  referenced file. Copy `register-sw.js`, `sw.js`, `manifest.webmanifest`, and
  icons with your deploy pipeline or static host.
- Root-scope service workers for one-binary deploys are not a GOWDK contract
  yet. If an app needs `/sw.js` from the root scope, serve that file at the
  deploy edge or a static host in front of the generated binary.
- `BuildConfig.Assets` and a first-class PWA addon remain planned.

## Minimal Registration

Add a user-owned registration script to the generated pages:

```go
var Config = gowdk.Config{
	Build: gowdk.BuildConfig{
		Scripts: []gowdk.Script{
			{Src: "/register-sw.js", Type: "module"},
		},
	},
}
```

Copy `register-sw.js` next to the deployed output:

```js
if ("serviceWorker" in navigator) {
  window.addEventListener("load", () => {
    navigator.serviceWorker.register("/sw.js", { scope: "/" });
  });
}
```

Link a web app manifest outside GOWDK today, for example through a static host
template or post-build HTML transform:

```html
<link rel="manifest" href="/manifest.webmanifest">
```

## Service Worker Rules

Keep the service worker conservative:

```js
const CACHE = "site-v1";
const PRECACHE = ["/", "/gowdk-assets.json"];

self.addEventListener("install", (event) => {
  event.waitUntil(caches.open(CACHE).then((cache) => cache.addAll(PRECACHE)));
});

self.addEventListener("fetch", (event) => {
  const url = new URL(event.request.url);
  if (event.request.method !== "GET") return;
  if (url.pathname.startsWith("/api/")) return;
  if (url.pathname.startsWith("/_gowdk/")) return;
  if (url.pathname.includes("/fragments/")) return;

  event.respondWith(
    caches.match(event.request).then((cached) => {
      return cached || fetch(event.request);
    }),
  );
});
```

Use explicit allowlists for pages and immutable generated assets. Avoid
catch-all offline fallbacks for request-time pages unless the app can tolerate
stale or unauthenticated content.

## What GOWDK Will Not Do Implicitly

- Register service workers.
- Cache generated action/API/fragment/SSR responses.
- Cache CSRF tokens, auth state, session-specific HTML, or error responses.
- Rewrite routes to an offline shell.
- Install Workbox, npm packages, or browser build tooling.

Document any app-owned offline behavior beside the deploy recipe so operators
know how to roll back a bad service worker version.
