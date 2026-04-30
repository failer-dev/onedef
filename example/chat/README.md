# Chat Example

This example shows a small customer-to-merchant chat backend where one onedef
definition powers the HTTP server, IR generation, and Dart SDK generation.

It demonstrates:

- group-scoped dependencies
- request-scoped dependencies from `BeforeHandle`
- `AfterHandle` audit logging
- `Observe` request outcome logging
- typed `Header[T]` symbols
- the same `Authorization` wire header parsed differently per group

## Run the Backend

```sh
cd example/chat/backend
go run ./cmd/server
```

Use a different address if needed:

```sh
go run ./cmd/server :8081
```

## Try the API

Start a conversation as a customer:

```sh
curl -s -X POST http://localhost:8280/api/customer/conversations \
  -H 'Authorization: Customer customer_123' \
  -H 'Content-Type: application/json' \
  -d '{"storeId":"store_main","message":"Is this lamp still available?"}'
```

Read the conversation as the same customer:

```sh
curl -s http://localhost:8280/api/customer/conversations/conv_1 \
  -H 'Authorization: Customer customer_123'
```

Send another message as the customer:

```sh
curl -s -X POST http://localhost:8280/api/customer/conversations/conv_1/messages \
  -H 'Authorization: Customer customer_123' \
  -H 'Content-Type: application/json' \
  -d '{"message":"I can pick it up this afternoon."}'
```

Read it as the merchant:

```sh
curl -s http://localhost:8280/api/merchant/conversations/conv_1 \
  -H 'Authorization: Merchant merchant_456 store_main'
```

Reply as the merchant:

```sh
curl -s -X POST http://localhost:8280/api/merchant/conversations/conv_1/messages \
  -H 'Authorization: Merchant merchant_456 store_main' \
  -H 'Content-Type: application/json' \
  -d '{"message":"Yes, it is available today."}'
```

The two groups use the same `Authorization` wire header, but parse different
formats. This request fails in the customer group because the merchant token
format does not match the customer parser:

```sh
curl -s http://localhost:8280/api/customer/conversations/conv_1 \
  -H 'Authorization: Merchant merchant_456 store_main'
```

## Generate IR and Dart SDK

From `example/chat`:

```sh
make generate
```

This writes generated artifacts to:

- `example/chat/sdk/onedef.spec.json`
- `example/chat/sdk/lib/`

Generated files are intentionally ignored. Re-run `make generate` whenever the
backend definition changes.
