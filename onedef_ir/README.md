# onedef_ir

`onedef_ir` is the language-agnostic contract between onedef server implementations
and Client SDK generators.

The directory intentionally contains no HTTP runtime, SDK rendering, or
target-language client code. It may contain reference validators for the IR
contract itself. Each server implementation should emit this IR as JSON. Each SDK
generator should parse and validate the same JSON before rendering
target-language code.

## What Belongs Here

- Canonical IR version documents.
- JSON Schema for structural validation.
- Valid and invalid fixtures shared by all producers and consumers.
- Conformance test manifests that describe semantic checks beyond JSON Schema.
- Reference validators for semantic checks that JSON Schema cannot express.

## What Does Not Belong Here

- HTTP server runtime logic.
- Language-specific reflection or AST parsers.
- SDK rendering code.
- Transport/client runtime libraries.
- Dart-specific parser models, unless they live in a separate binding package.

## Layout

```text
onedef_ir/
  spec/
    v1.md
  schema/
    v1.schema.json
  fixtures/
    valid/
    invalid/
  conformance/
    README.md
    cases.json
  validator/
    document.go
    types.go
```

## Package Boundaries

- `onedef_ir`: canonical IR contract.
- `onedef_ir/validator`: Go reference model and validator for IR JSON.
- `onedef_go/internal/irbuild`: Go producer helpers that turn Go endpoint definitions into IR.
- `onedef_dart/sdk_gen`: Dart SDK generator and its Dart-side IR reader.
- Future `onedef_*_sdk_gen`: target-specific consumers of the same IR.

The important invariant is simple: server implementations do not target Dart,
TypeScript, Kotlin, or any other SDK directly. They target onedef IR.
