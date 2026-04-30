# onedef Go IR 출력 규칙

Canonical IR 계약은 `onedef_ir/spec/v1.md`를 따른다. 이 문서는 Go producer가
그 계약을 어떻게 채우는지만 요약한다.

## Group Header

Go route root 또는 group boundary에서 공급해야 하는 header는 IR의
`routes.headers` 또는 `routes.groups[].headers`로 출력한다.

```json
{
  "routes": {
    "headers": [
      {
        "key": "Authorization",
        "type": "string"
      }
    ],
    "groups": [
      {
        "name": "branch",
        "headers": [
          {
            "key": "X-Branch-Id",
            "type": "string"
          }
        ]
      }
    ]
  }
}
```

- `headers[].key`는 HTTP header 이름이다.
- `headers[]`에는 SDK 이름용 `name`을 넣지 않는다.
- `headers[]`는 존재하면 required group-scope contract로 본다.
- 기존 derived header summary는 출력하지 않는다.

## Request Binding

Endpoint request binding은 문맥별로 나누어 출력한다.

```json
{
  "request": {
    "paths": [
      {
        "name": "ID",
        "key": "id",
        "type": "uuid"
      }
    ],
    "queries": [
      {
        "name": "Cursor",
        "key": "cursor",
        "type": "string?"
      }
    ],
    "headers": [
      {
        "name": "IdempotencyKey",
        "key": "Idempotency-Key",
        "type": "int",
        "required": true
      }
    ]
  }
}
```

- `request.paths[]`: path template `{variable}`과 같은 `key`를 가진다.
- `request.queries[]`: GET/DELETE request의 non-path exported field에서 온다.
- `request.headers[]`: endpoint-level `RequireHeader` 중 method parameter로 받는
  header만 들어간다.
- Group-scope header binding은 `request.headers[]`에 넣지 않는다.

## Model Fields

Go struct field는 IR field로 출력한다.

```json
{
  "name": "CustomerID",
  "key": "customerId",
  "type": "uuid",
  "required": true
}
```

- `name`은 Go source field name이다.
- `key`는 JSON property name이다.
- nullability는 `type` 표현식의 `?`에서만 드러난다.
- `omitEmpty`는 `json:",omitempty"`에서 파생한다.
