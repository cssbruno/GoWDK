# Images

GOWDK does not optimize, resize, transcode, or rewrite images today. Image
optimization is app-owned, CDN-owned, or addon-owned. The compiler core keeps
image handling explicit so it does not need native image libraries, npm tools,
or remote optimization services.

## Current Contract

- Literal `<img>`, `<picture>`, `<source>`, `src`, and `srcset` markup is
  supported in `view {}`.
- URL-valued attributes are checked for unsafe schemes. `srcset` candidates are
  checked individually.
- `missing_img_alt` warns when a literal `<img>` has no explicit `alt`.
- Page `image` metadata and `Build.Head.Image` can emit Open Graph/Twitter
  image metadata. They do not create image files.
- Component `asset` metadata can package image files into generated output and
  record them in `gowdk-assets.json`, but GOWDK does not rewrite literal
  `<img src>` URLs to those content-hashed paths.

## Recommended Pipeline

Optimize images before `gowdk build`, or at the CDN/static host:

1. Keep source images in app-owned folders such as `assets/source/`.
2. Generate deploy variants such as AVIF, WebP, and fallback JPEG/PNG with a
   tool chosen by the app.
3. Copy optimized files to stable deploy paths or serve them through a CDN.
4. Reference those paths explicitly from `.gwdk` markup.
5. Use `width`, `height`, `loading`, `decoding`, `sizes`, and `srcset` so the
   browser can choose the right asset.

Example:

```gwdk
view {
  <picture>
    <source
      type="image/avif"
      srcset="/images/hero-640.avif 640w, /images/hero-1280.avif 1280w"
      sizes="(max-width: 700px) 100vw, 700px" />
    <source
      type="image/webp"
      srcset="/images/hero-640.webp 640w, /images/hero-1280.webp 1280w"
      sizes="(max-width: 700px) 100vw, 700px" />
    <img
      src="/images/hero-1280.jpg"
      alt="Product dashboard"
      width="1280"
      height="720"
      loading="lazy"
      decoding="async" />
  </picture>
}
```

## Component Assets

Use component `asset` metadata when the image is owned by a component and should
be visible in the generated asset manifest:

```gwdk
component ProductBadge
asset "./badge.png"
```

GOWDK emits the file under
`assets/gowdk/components/<package>/<component>/` with a content hash and records
the logical-to-emitted mapping in `gowdk-assets.json`. The component markup
still owns its image URL. Use a stable deploy path or consume the manifest in
app-owned code when the hashed path must be referenced.

## Social Images

Use page metadata or `Build.Head.Image` for social cards:

```gwdk
page launch
route "/launch"
guard public
image "https://cdn.example.com/social/launch.png"
```

`image` emits metadata only. The referenced file must already exist at the
given URL.

## Non-Goals

GOWDK does not currently:

- Generate responsive image variants.
- Download or install image tools.
- Rewrite image references through a CDN.
- Inline image data.
- Infer `alt` text.
- Turn `asset` metadata into automatic `<img>` URLs.

Future optional integrations may emit optimized variants or metadata, but they
must stay opt-in and outside the compiler/runtime core dependency surface.
