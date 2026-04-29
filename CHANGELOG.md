# Changelog

## [0.2.0] - 2026-04-29

### Added

- Added Go group DSL: `Group`, `Endpoint`, `Endpoints`.
- Added scoped `Provide`, `BeforeHandle`, `AfterHandle`, and `Observe`.
- Added typed headers through `Header[T]`, `RequireHeader`, and `OmitHeader`.
- Added scoped `ErrorPolicy[T]` and endpoint-level overrides.
- Added `GenerateIRJSON` for programmatic IR output.
- Added `onedef_ir` as the language-neutral contract.
- Added canonical IR docs, schema, fixtures, and validator tests.
- Added Dart `sdk_core` and `sdk_gen` packages.
- Added Dart `Result<T, E>` return model.
- Added typed Dart failure variants for HTTP errors, network failures, and contract violations.
- Added declared success status support, including `201` and `204`.
- Added structured success and error envelopes with `code`, `title`, `message`, and `data`.
- Added server timeout defaults with override options.
- Added chat example covering server, IR generation, and Dart SDK generation.

### Breaking Changes

- Replaced raw non-204 success bodies with `{code, title, message, data}` envelopes.
- Replaced global error handler flow with scoped `ErrorPolicy[T]`.
- Moved SDK generation out of the Go runtime.
- Moved Dart SDK generation to external IR consumers.
- Moved Dart shared runtime types to `onedef_dart/sdk_core`.
- Replaced generated Dart method return values with `Result<T, E>`.
- Required generated Dart clients to accept only the declared success status. Any other status is a failure.
- Required `204` endpoints to use `Response struct{}` and return no body.

### Removed

- Removed in-process Dart SDK generation from the Go runtime.
- Removed `GET /onedef/sdk/dart`.
- Removed the built-in Go Dart generator under `internal/sdk/dart`.
- Removed the old generated `OnedefApiException` flow.
- Removed flat single-client Dart SDK generation through `src/client.dart`.

## [0.1.0] - 2026-03-27

### Added

#### Core Framework

- Added struct-based API endpoint definitions with sealed HTTP method markers.
- Added `GET`, `POST`, `PUT`, `PATCH`, `DELETE`, `HEAD`, and `OPTIONS` markers.
- Added route definitions through `path:""` struct tags.
- Added `Handle(context.Context) error` handler interface.
- Added sealed method marker interfaces to block external marker implementations.

#### Request Parsing

- Added automatic path parameter extraction.
- Added type conversion for `string`, signed integers, unsigned integers, `bool`, and `uuid.UUID`.
- Added automatic query parameter parsing for `GET` and `DELETE`.
- Added JSON body parsing for `POST`, `PUT`, and `PATCH`.
- Added path parameter precedence over body values.

#### Dart SDK Generation

- Added Dart HTTP client generation from Go struct definitions.
- Added Go-to-Dart type mapping.
- Added Dart model classes with constructors, `fromJson()`, and `toJson()`.
- Added ZIP package delivery through `GET /onedef/sdk/dart`.
- Added customizable package naming.

#### Server

- Added `http.ServeMux` routing.
- Added `Register()` and `Run()` public API.
- Added registered route listing on startup.
