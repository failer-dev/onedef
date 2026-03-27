# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

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
