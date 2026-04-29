# onedef

**One def**inition, REST API server and client SDKs — always in sync.

I got tired of manually syncing APIs with Frontend, so I built a framework that generates the SDKs automatically.

> ⚠️ **v0.2.0** — Not production-ready. Expect breaking changes.

## The Solution

In onedef, the struct *is* the API contract.

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

- **`GET /users/{id}`** — registered, path param parsed, response serialized
- **Client SDK** — type-safe, generated from the same definition, no drift possible

Change the struct. Everything updates. Synchronization cannot break — structurally.

## The Problem

OpenAPI solved the documentation problem. But it didn't solve the product problem.

You can describe an API with OpenAPI. What you can't guarantee is that the generated SDK feels like something a developer would actually want to use.

Here's what usually happens: the backend team generates a massive spec and puts Swagger UI or Redoc in front of it. The API is technically documented. But client developers are still left piecing together how the API actually works — reading through endpoints, schemas, nullable fields, error shapes, and weird naming from the generator.

When that's not enough, they go read the backend code directly.

Then they start fighting the schema. Tuning the generator. Patching the generated client. Writing glue code around all of it.

Somehow, this became the de facto standard.

I built onedef because I got tired of that. I've worked as a client developer, a backend developer, a CTO, and a tech lead — and every time, the handoff was the same problem.

## Client SDK

Currently targeting Dart.

TypeScript, Swift, Kotlin, and more on the roadmap.

The SDK generation is powered by a custom IR (Intermediate Representation) designed
from scratch — not derived from OpenAPI, not a wrapper around existing tooling.
It was built with one goal: **make client developers happy.**

Because the IR is language-agnostic, any language is theoretically supported
as long as it has a parser. With a Dart reference implementation already in place,
the LLM era makes adding new targets more tractable than ever.