# onedef

**One definition. HTTP server, Dart SDK and TypeScript SDK — all at once.**

[![Go 1.25+](https://img.shields.io/badge/go-1.25+-blue)](https://go.dev/dl/)

## The Solution

In onedef, the struct *is* the API contract and spec.

```go
type GetUserAPI struct {
    onedef.GET `path:"/users/{id}"`
    Request    struct{ ID string }
    Response   User
}

func (h *GetUserAPI) Handle(ctx context.Context) error {
    h.Response = db.FindUser(h.Request.ID)
    return nil
}
```

This single struct gives you:

- `GET /users/{id}` — registered, path param parsed, response serialized
- Dart SDK — `curl localhost:8080/onedef/sdk/dart`
- TypeScript SDK — `curl localhost:8080/onedef/sdk/ts` (TODO)

Change the struct. Everything updates. Synchronization cannot break — structurally.

## Why onedef?
TL;DR: The paradigm shift in the LLM era isn't using LLM as a tool — it's making your project a tool for LLM.

Read more in [WHY_ONEDEF.md](./docs/WHY_ONEDEF.md).