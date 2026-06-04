# API

The current parser records `api {}` or `api <name> {}` block declarations. API method, route, request decoding, response encoding, and generated handler behavior are planned.

Future API behavior must define:

- How API names map to routes and methods.
- Request body and query decoding.
- Response types and JSON/HTML boundaries.
- Authentication and authorization hooks.
- Error response shape.
- Interaction with static/action pages without full-page SSR.
