# onedef

**One definition. HTTP server, Dart SDK, TypeScript SDK, and docs — all at once.**

[![Go 1.25+](https://img.shields.io/badge/go-1.25+-blue)](https://go.dev/dl/)

## The Solution

In onedef, the struct *is* the API contract.

```go
type GetUser struct {
    onedef.GET `path:"/users/{id}"`
    Request    struct{ ID string }
    Response   User
}

func (h *GetUser) Handle(ctx context.Context) error {
    h.Response = db.FindUser(h.Request.ID)
    return nil
}
```

This single struct gives you:

- `GET /users/{id}` — registered, path param parsed, response serialized
- Dart SDK — `curl localhost:8080/onedef/sdk/dart`
- TypeScript SDK — `curl localhost:8080/onedef/sdk/ts`
- API docs — `localhost:8080/onedef/docs`

Change the struct. Everything updates. Synchronization cannot break — structurally.

## The Problem

Adding one endpoint means touching multiple files:

```
router.go      — register the route
handler.go     — parse params, serialize response
types.go       — define Request/Response
swagger.yaml   — update docs (usually forgotten)
user.dart      — update Dart SDK (almost always forgotten)
```

The contract is scattered. Drift is inevitable. 

In the LLM era, this makes it worse — they touch one file and miss the rest.


## Why onedef

### 1. Structural synchronization

- **Single source of truth:** the struct — no separate spec file, no SDK to hand-maintain, no docs to remember to update
- **Auto-propagation:** when the struct changes, everything downstream changes automatically
- **Drift-proof:** sync cannot break — it's a structural impossibility

### 2. LLM-first design

- **One struct, one task:** when an LLM adds an endpoint, it writes one struct — no router registration, no param parsing, no doc annotation
- **No room for error:** the struct shape is the convention — there's nothing else to get wrong
- **Fast human review:** method, path, input, output — all visible in five seconds, in one place

### 3. Deterministic generation

- **Type-stable:** `string` is always `String` in Dart; `[]User` is always `List<User>`
- **No creativity required:** SDK generation needs no judgment — onedef generates it deterministically, without LLMs, without humans
- **Always correct:** deterministic input, deterministic output