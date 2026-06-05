# API

The current parser records `api {}` or `api <name> {}` block declarations.
The first implemented API route metadata subset accepts one method/route line:

```gowdk
api health {
  GET "/api/health"
}
```

Supported methods are `GET`, `POST`, `PUT`, `PATCH`, and `DELETE`.
The route must be a quoted absolute route path. API request decoding, response
encoding, and generated handler behavior are planned.

Future API behavior must define:

- Request body and query decoding.
- Response types and JSON/HTML boundaries.
- Authentication and authorization hooks.
- Error response shape.
- Interaction with SPA/action pages without full-page SSR.
