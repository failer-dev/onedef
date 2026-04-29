# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- Programmatic `GenerateIRJSON` and `GenerateSDK` APIs for consuming endpoint definitions from Go
- A public `ir` package for parsing endpoint definitions into JSON-serializable IR
- A Dart melos workspace under `generators/dart` with `onedef_gen` and `onedef_core`
- A shared `onedef_core` Dart package for `Transport`, `Success`, `ApiException`, and future shared runtime helpers
- Scoped `ErrorPolicy[T]` nodes for typed runtime error responses and endpoint-specific overrides
- Dart SDK `Result<T, E>` return values with typed HTTP, network, and contract-violation variants

### Removed

- The built-in Go Dart generator under `internal/sdk/dart`
- The runtime `GET /onedef/sdk/dart` SDK download endpoint
- `ErrorHandler`/`SetErrorHandler` in favor of group-scoped `ErrorPolicy[T]`

## [0.2.0] - 2026-04-20

### Added

#### HTTP Contract

- Method marker tags can now declare a custom success status — e.g `status:"201"` and `status:"204"`
- A public `HTTPError` type with convenience constructors: `BadRequest`, `Unauthorized`, `Forbidden`, `NotFound`, `Conflict`, `Unprocessable`, and `Internal`
- Errors now return a structured JSON envelope with `code`, `title`, `message`, and `data` fields
- Non-`204` success responses are wrapped in the same `{code, title, message, data}` shape, with `data` carrying the endpoint payload
- Endpoints that declare `Response struct{}` are strictly enforced as `204 No Content`

#### Dart SDK

- `Success<T>` and `Failure` types are now generated alongside each package
- `Failure` factories cover three cases: HTTP errors, network failures, and invalid responses
- Client methods check for the exact declared success status rather than any non-error code
- `204` endpoints are generated as `Future<Success<void>>`
- APIs are now grouped — a root `Api` class with subgroups like `UserApi` and `AuthApi`
- Package layout is now `src/api.dart`, `src/core/base_api.dart`, `src/core/models.dart`, `src/core/result.dart`, and `src/<group>/api.dart`

#### Examples and Tests

- An end-to-end Dart example walking through create, conflict, fetch, and delete user flows
- Go tests covering success status inspection, HTTP envelope shape, `204` handling, and grouped Dart SDK generation

### Changed

- Non-`204` success responses now return a `{code, title, message, data}` envelope instead of the raw `Response` body
- Request parse failures produce structured `400` errors with stable codes: `invalid_body`, `invalid_path_parameter`, and `invalid_query_parameter`
- Handler panics and unexpected errors are masked as `500 internal_error` — raw server errors no longer leak through
- Dart SDK methods now return `Future<Success<T>>` and throw `Failure` rather than raw model types and `OnedefApiException`
- The generated package layout moves from a flat single-file client to the grouped `src/api.dart` + `src/core/*` + `src/<group>/api.dart` structure
- Generated clients now accept only the declared success status, not any non-error response

### Removed

- `OnedefApiException` from generated Dart packages
- The flat `src/client.dart` entrypoint
### Removed

- Generated `OnedefApiException` type from Dart SDK packages
- Flat single-client Dart SDK entrypoint generation via `src/client.dart`

## [0.1.0] - 2026-03-27

### Added

#### Core Framework

- Struct-based API endpoint definition system with sealed HTTP method markers (GET, POST, PUT, PATCH, DELETE, HEAD, OPTIONS)
- Route definition via `path:""` struct tags
- `Handle(context.Context) error` handler interface
- Type-safe sealed interface preventing external method marker implementations

#### Request Parsing

- Automatic path parameter extraction with type conversion (string, int8-64, uint8-64, bool, uuid.UUID)
- Automatic query parameter parsing for GET/DELETE requests
- JSON body parsing for POST/PUT/PATCH requests
- Path parameters override body values

#### Dart SDK Generation

- Automatic Dart HTTP client generation from Go struct definitions
- Go to Dart type mapping (integers, floats, strings, booleans, pointers to nullable, slices to List, maps to Map, structs to classes, uuid to String)
- Dart model class generation with constructors, `fromJson()`, and `toJson()`
- ZIP package delivery via `GET /onedef/sdk/dart` endpoint
- Customizable package naming

#### Server

- `http.ServeMux`-based routing
- `Register()` and `Run()` public API
- Registered route listing on startup
