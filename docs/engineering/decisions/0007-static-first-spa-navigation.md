# ADR 0007: Static-First SPA Navigation

## Status

Accepted

## Context

GOWDK defaults full pages to SPA/build-time output, but that must not turn the
framework into a browser-owned application shell. Generated JavaScript can make
navigation and forms feel smoother, but the Go compiler, generated Go runtime,
and user Go packages must remain the source of truth for route existence,
backend behavior, security, and request-time policy.

## Decision

SPA means static-first pages with optional client navigation enhancement.

Every generated SPA route must be a real URL with a concrete HTML artifact or
runtime-served file candidate that works on direct open, browser refresh, and
non-JavaScript clients. Generated JavaScript may intercept internal links only
as an enhancement after those URLs already exist.

Generated JavaScript may enhance:

- internal-link navigation between generated SPA page artifacts;
- fetching built page shells or server fragments;
- swapping the visible page region;
- preserving scroll and focus where possible;
- prefetching static route assets;
- showing loading and error UI;
- progressively enhancing action forms and partial fragment swaps.

Generated JavaScript must not own:

- route existence;
- authentication or authorization decisions;
- business rules;
- database access or persistence;
- trusted server validation;
- action behavior;
- global application state;
- page loading policy;
- cache or revalidation policy.

Action forms must remain progressively enhanced. A supported generated form
should degrade to a normal HTTP POST where possible; generated JavaScript may
add partial request headers, swap fragments, and restore focus, but it must not
be required for the server to know what the action means.

`client {}` remains for local component/UI behavior such as toggles, tabs,
counters, focus, small filters, and visual state. It is not a general
application logic runtime and must not become the place where routes,
authorization, persistence, or trusted validation live.

## Consequences

- GOWDK can add SPA navigation without becoming a client-owned framework.
- Generated page files and the generated Go runtime continue to define route
  availability.
- User Go handlers continue to own auth, validation, storage, business rules,
  and action/API behavior.
- Future navigation work must prove direct URL and refresh behavior before
  adding interception, prefetching, or page-region swapping.
