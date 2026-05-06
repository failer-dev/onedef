# onedef

**One definition. HTTP runtime, programmatic IR, and language SDK generators.**

[![Go 1.25+](https://img.shields.io/badge/go-1.25+-blue)](https://go.dev/dl/)

```sh
go get github.com/failer-dev/onedef/onedef_go
```

```go
import "github.com/failer-dev/onedef/onedef_go"
```

## The Solution

In onedef, the struct *is* the API contract and spec.

```go
type GetUserAPI struct {
    onedef.GET `path:"/users/{id}"`
    Request    struct{ ID string }
    Response   User
    Provide    struct {
        Users UserRepo
    }
}

func (h *GetUserAPI) Handle(ctx context.Context) error {
    user, err := h.Provide.Users.FindUser(ctx, h.Request.ID)
    if err != nil {
        return err
    }
    h.Response = user
    return nil
}

repo := &PostgresUserRepo{}
api := onedef.Group(
    "/",
    onedef.Provide[UserRepo](repo),
    onedef.Endpoint(&GetUserAPI{}),
)
app := onedef.New(api)
```

This single struct gives you:

- `GET /users/{id}` — registered, path param parsed, response serialized
- Predictable HTTP contract — optional `status:"201"`, success envelopes, and structured JSON errors
- Typed Provide scope — scoped `onedef.Provide(...)` nodes plus request-time values from `BeforeHandle`
- Programmatic IR JSON — parse the same endpoint definitions into a language-neutral spec from Go
- SDK generation from IR — Dart today, other languages later

Change the struct. Everything updates. Synchronization cannot break — structurally.

## Definition + IR Generation

Keep your endpoint definitions in an importable package. Runtime dependencies live in the same definition tree, but they are ignored by IR generation:

```go
package api

type RuntimeDeps struct {
    Users  UserRepo
    Logger *slog.Logger
}

func Definition(deps RuntimeDeps) *onedef.Spec {
    return onedef.Group(
        "/",
        onedef.Provide[UserRepo](deps.Users),
        onedef.Provide(deps.Logger),
        onedef.Endpoints(&GetUser{}, &CreateUser{}),
    )
}
```

Then your `main` stays thin:

```go
app := onedef.New(api.Definition(api.RuntimeDeps{
    Users: repo,
    Logger: logger,
}))
```

For IR generation, call the same `Definition(deps)` with dummy or in-memory dependencies:

```go
specJSON, err := api.Definition(dummyDeps).GenerateIRJSON(onedef.GenerateIROptions{})
if err != nil {
    panic(err)
}
if err := os.WriteFile("onedef.spec.json", specJSON, 0o644); err != nil {
    panic(err)
}
```

Dart SDK support consumes that IR from sibling packages: `../onedef_dart/sdk_gen` and `../onedef_dart/sdk_core`.
Generated Dart SDKs depend on the shared `onedef_dart_sdk_core` Dart package in `../onedef_dart/sdk_core`.

## Server hardening defaults

`app.Run(...)` now applies secure `net/http.Server` defaults automatically:

- `ReadHeaderTimeout: 5s`
- `ReadTimeout: 15s`
- `WriteTimeout: 30s`
- `IdleTimeout: 60s`
- `MaxHeaderBytes: 1 << 20`
- `MaxBodyBytes: 10 MiB`

You can still override them per server:

```go
if err := app.Run(
    ":8080",
    onedef.WithReadTimeout(20*time.Second),
    onedef.WithIdleTimeout(90*time.Second),
    onedef.WithMaxBodyBytes(20<<20),
); err != nil {
    panic(err)
}
```

## Why onedef?
TL;DR: The paradigm shift in the LLM era isn't using LLM as a tool — it's making your project a tool for LLM.

Read more in [WHY_ONEDEF.md](./docs/WHY_ONEDEF.md).
