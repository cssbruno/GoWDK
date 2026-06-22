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
The executable config bridge materializes those URLs before build output is
generated.
