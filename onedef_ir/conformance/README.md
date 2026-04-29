# onedef IR Conformance

Conformance exists so every server implementation and every SDK generator can
agree on the same IR behavior.

JSON Schema validation is necessary but not enough. Some rules require semantic
checks, such as duplicate type names, unknown named type references, and matching
path variables to `request.pathParams`.

## Producers

A producer is conformant when it:

- emits JSON that passes `schema/v1.schema.json`;
- emits JSON that passes all semantic rules in `spec/v1.md`;
- can produce fixtures equivalent to the valid examples for its language;
- never relies on SDK-language-specific fields.

## Consumers

A consumer is conformant when it:

- parses all files listed as valid in `cases.json`;
- rejects all files listed as invalid in `cases.json`;
- normalizes omitted or `null` arrays as empty arrays;
- treats missing endpoint `error` as `DefaultError`;
- ignores unknown fields while rejecting unknown type kinds.

## Semantic Error Codes

Implementations do not need to expose these exact strings to users, but tests
should map failures to equivalent categories.

- `duplicate_type`
- `unknown_type_ref`
- `invalid_type_ref`
- `path_param_mismatch`
- `invalid_success_response`
- `unsupported_version`
