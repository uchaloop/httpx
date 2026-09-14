# Changelog

All notable changes to this module are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this module adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.1.0] - 2026-09-14

First release: router-neutral building blocks for JSON HTTP APIs on the standard
library alone.

- `response`: a handler's answer as a `Response` value, with `JSON`, `Data`,
  `OK`, `Created`, `NoContent` and `Page` constructors, `Validate`,
  `SetHeaders` with canonical header names, and `Write`, the net/http backend
  that encodes the body before it commits anything.
- `problem`: RFC 9457 Problem Details with `code` and `errors` extensions; an
  ordered `Mapper` with `WhenIs`, `WhenAs` and `When` rules that answers every
  unknown error with a bare 500 `internal_error`; `MapKnown` for router
  fallbacks; `Header` for headers a problem calls for, such as `Accept` on 415.
- `problem.InputProblemRule` for the errors of `request`, `page` and `sortby`:
  400 `invalid_json`, 413, 415 with `Accept`, and 422 `invalid_request` with
  go-playground/validator tag names and messages; `ErrorRule`, `MakeError`,
  `MakeInvalidRequest`, `InvalidRequest` and `TypeParam` for handlers and
  router adapters.
- `request`: strict `DecodeJSON` and `DecodeAndValidateJSON` - a JSON media
  type, no unknown fields unless `AllowUnknownFields`, exactly one document - with
  a typed `DecodeError`; body read errors, such as a size limit, are returned as
  is.
- `page`: offset pagination validated against an endpoint's `Config`, with
  optional `MaxOffset` and errors reported in page numbers.
- `sortby`: `field` and `field:direction` expressions parsed into a typed
  `Order` through an application field parser, with `Order.Make` for storage
  types.
- `apitest`: handler tests through a real HTTP client and an in-memory server,
  with fluent requests, per-request timeouts bound by the test deadline, and
  exact `JSONEqual` comparison with differences reported by path.

[0.1.0]: https://github.com/uchaloop/httpx/releases/tag/v0.1.0
