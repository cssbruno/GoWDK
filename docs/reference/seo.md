# SEO Addon

`addons/seo` emits `sitemap.xml` and `robots.txt` during `gowdk build`.
It is disabled by default because sitemap URLs need site-level deploy policy.

Configure it in `gowdk.config.go`:

```sh
gowdk add seo --base-url https://example.com
```

```go
package app

import (
	"github.com/cssbruno/gowdk"
	"github.com/cssbruno/gowdk/addons/seo"
)

var Config = gowdk.Config{
	Addons: []gowdk.Addon{
		seo.Addon(seo.Options{
			BaseURL:  "https://example.com",
			Disallow: []string{"/drafts"},
			ExtraURLs: []seo.URL{
				{Loc: "/rss.xml"},
			},
		}),
	},
}
```

Build:

```sh
gowdk build --config gowdk.config.go --out dist/site
```

Output:

```text
dist/site/
  sitemap.xml
  robots.txt
```

`BaseURL` is required and must be an absolute `http` or `https` URL. Page routes
are joined onto it. If `BaseURL` includes a path such as
`https://example.com/docs`, generated URLs stay under that path.

`sitemap.xml` includes build-time-enumerable page routes:

- public static SPA pages;
- public dynamic SPA routes expanded from literal `paths {}` declarations;
- optional `ExtraURLs`, which may be absolute URLs or root-relative paths.

Request-time pages are not included because `gowdk build` cannot know their
runtime URL set. Guardless pages are also excluded because generated apps deny
those routes until access is stated with `guard public` or a protective guard.
The build report records one `seo_route_excluded` event for each excluded page,
including a `reason` and render `mode`.

`robots.txt` emits:

```text
User-agent: *
Disallow: /drafts
Sitemap: https://example.com/sitemap.xml
```

When `Disallow` is empty, the file emits an empty `Disallow:` directive.

Dynamic extra URLs can be supplied with `ExtraURLProvider` in normal Go config.
Project-aware commands execute the native config helper, so those URLs are
materialized before build output is generated.

## Structured Data

Pages can declare supported JSON-LD schema kinds with `jsonld` metadata:

```gwdk
package pages

route "/blog/launch"
guard public
title "Launch"
description "GOWDK launch notes"
canonical "https://example.com/blog/launch"
image "https://example.com/assets/launch.png"
jsonld Article

build {
  => {
    headline: "Launch",
    author: "Ada",
    datePublished: "2026-06-22",
  }
}

view {
  <main>
    <h1>{headline}</h1>
  </main>
}
```

Supported kinds today:

- `WebPage`
- `Article`

The compiler rejects unknown kinds with `invalid_structured_data` and duplicate
kinds on the same page with `duplicate_structured_data`.

Generated HTML writes one deterministic, escaped
`<script type="application/ld+json">` block per declaration. GOWDK serializes
the JSON with Go's structured JSON encoder and escapes script-breaking
characters, so page data such as `<GOWDK>` is emitted safely.

Payload fields are derived from page metadata first, then build data where the
schema needs page-specific values:

- `WebPage`: `name`, `description`, `url`, and `image`.
- `Article`: `headline`, `description`, `url`, `image`, `datePublished`, and
  `author`.

For `Article`, build data keys `headline`, `author` or `authorName`, and
`datePublished`, `published`, or `date` populate the schema when present.

Structured-data metadata is inspectable:

- `gowdk manifest` includes page metadata as `"jsonld": ["Article"]`.
- `gowdk build` records a `seo` / `structured_data` event in
  `gowdk-build-report.json` for each page with structured data.

## Dynamic Sitemap Hook

Build-time `sitemap.xml` intentionally includes only URLs known during the
build. Generated apps can also serve `/sitemap.xml` at request time by declaring
a dynamic sitemap provider:

```go
package app

import (
	"github.com/cssbruno/gowdk"
	"github.com/cssbruno/gowdk/addons/seo"
)

var Config = gowdk.Config{
	Addons: []gowdk.Addon{
		seo.Addon(seo.Options{
			BaseURL: "https://example.com",
			DynamicSitemap: seo.DynamicSitemap{
				ImportPath:   "github.com/acme/site/sitemap",
				Function:     "DynamicURLs",
				MaxURLs:      500,
				CacheSeconds: 300,
			},
		}),
	},
}
```

Provider package:

```go
package sitemap

import (
	"context"

	"github.com/cssbruno/gowdk/runtime/seo"
)

func DynamicURLs(ctx context.Context) ([]seo.URL, error) {
	return []seo.URL{
		{Loc: "/products/widget", LastMod: "2026-06-22"},
	}, nil
}
```

The generated app imports the provider and registers `/sitemap.xml` before the
fallback page route. The runtime handler merges:

- public build-time page URLs known to GOWDK;
- `ExtraURLs`;
- URLs returned by the dynamic provider.

URLs are normalized against `BaseURL`, deduplicated, sorted, and serialized as
standard sitemap XML. `MaxURLs` caps provider output, and `CacheSeconds`
controls the successful response cache header. Provider errors or cap overflow
return `503` with `Cache-Control: no-store`; the error detail stays server-side.

Guardless pages, non-public pages, and `noindex` pages remain excluded from the
GOWDK-owned static URL set. If an app wants request-time-only or database-owned
URLs in the sitemap, the provider must apply the app's own visibility policy
before returning them.
