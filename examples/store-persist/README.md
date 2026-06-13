# Persisted page store

A page `store` that opts into browser persistence with `persist "local"`, so a
shared cart survives reloads and SPA navigation.

```gwdk
store cart ui.CartState = ui.NewCartState() persist "local"
```

- `ui/cart.go` — the user-owned Go state. GOWDK serializes only its declared
  fields; it adds no opinions to the struct.
- `shop.page.gwdk` — declares the persisted store and renders the two islands.
- `add-button.cmp.gwdk` / `cart-badge.cmp.gwdk` — share the store via
  `client { use cart }`.

## Build it

```sh
gowdk build --out ./dist \
  examples/store-persist/shop.page.gwdk \
  examples/store-persist/add-button.cmp.gwdk \
  examples/store-persist/cart-badge.cmp.gwdk
```

The generated `shop/index.html` carries the persist config on the store seed:

```html
<script type="application/json" data-gowdk-store="cart"
  data-gowdk-persist="local"
  data-gowdk-persist-key="gowdk:store:cart"
  data-gowdk-persist-version="…">{"count":0}</script>
```

and `assets/gowdk/islands/stores.js` hydrates the store from `localStorage` on
load and writes it back on every change.

## What to try

1. Open `/shop`, click **Add to cart** a few times — the badge counts up.
2. Reload the page. The badge keeps your count (restored from `localStorage`).
3. Change `CartState`'s shape in `ui/cart.go` and rebuild. The old persisted
   value is discarded automatically (the embedded schema hash changes), so the
   store falls back to its init value instead of restoring stale data.

## Scopes

- `persist "local"` — `localStorage`; survives a browser restart, shared across
  all tabs and routes on the origin.
- `persist "session"` — `sessionStorage`; survives reload and SPA navigation,
  cleared when the tab closes.

Persisted store state is browser-visible: never persist secrets, session tokens,
or trusted authorization state. The compiler warns
(`page_store_persist_secret_field`) when a persisted field name resembles a
secret.
