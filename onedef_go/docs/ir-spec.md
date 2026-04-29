# onedef IR v1 스펙

이 문서는 onedef IR JSON을 생성하는 서버/툴 작성자와, 그 JSON을 읽어 SDK 또는 HTTP 클라이언트 파서를 만드는 구현자를 위한 계약 문서다.

IR은 Go endpoint 정의에서 추출된 HTTP 계약을 언어 중립 JSON으로 표현한다. v1의 핵심 산출물은 다음이다.

- endpoint method/path/status
- path/query/header/body request shape
- success response envelope와 response data type
- error body type
- group tree와 group-level required header provider 계약
- 공유 model type table
- SDK naming에 필요한 initialism 힌트

## 문서 형태

최상위 JSON object는 `version`, 선택적 `naming`, endpoint 목록 또는 group tree, 그리고 `types`를 가진다.

```json
{
  "version": "v1",
  "naming": {
    "initialisms": ["OAuth", "JWT", "ID"]
  },
  "groups": [],
  "endpoints": [],
  "types": []
}
```

필드 규칙:

- `version`은 필수이며 현재 값은 `"v1"`이다.
- `types`는 필수 배열이다. 참조할 named type이 없으면 producer는 빈 배열을 쓰는 것을 권장한다.
- `naming`은 선택이다. 없으면 `initialisms`는 빈 배열로 해석한다.
- `groups`는 group-aware SDK 생성을 위한 권장 형태다.
- `endpoints`는 flat IR 형태다. Go runtime의 ungrouped endpoint 등록이 이 형태를 만든다.
- `onedef.Spec.GenerateIRJSON`은 group tree 기반 JSON을 만든다.
- 클라이언트 파서는 `groups`와 `endpoints`를 둘 다 받아야 한다. 둘 다 있으면 둘 다 처리하되, SDK 구조 생성은 `groups`를 우선한다.
- `omitempty` 때문에 빈 배열/빈 문자열/false 성격의 필드는 생략될 수 있다. 파서는 생략된 배열 또는 `null` 배열을 빈 배열로, 생략된 문자열을 빈 문자열로, 생략된 bool을 false로 해석해야 한다.
- 알 수 없는 필드는 무시하는 쪽이 좋다. v1 안에서 additive 확장을 하기 위해서다.

## Naming

```json
{
  "initialisms": ["OAuth", "JWT", "ID"]
}
```

`naming.initialisms`는 identifier casing을 위한 힌트다. 예를 들어 Dart generator는 `ID`를 `id`/`ID`로 보존할지 판단할 때 이 값을 사용한다.

Go parser는 initialism 입력을 trim하고, 대소문자 무시 중복 제거 후, 긴 문자열 우선으로 정렬한다. 다른 서버 구현도 가능하면 같은 normalization을 적용한다.

## Group

`groups`는 recursive tree다.

```json
{
  "id": "branch.booking",
  "name": "booking",
  "pathSegments": ["branch", "booking"],
  "requiredHeaders": ["X-Booking-Scope"],
  "providerHeaders": [
    {
      "name": "BookingScope",
      "wireName": "X-Booking-Scope",
      "type": { "kind": "string" },
      "required": true
    }
  ],
  "endpoints": [],
  "groups": []
}
```

필드:

| 필드 | 타입 | 필수 | 의미 |
| --- | --- | --- | --- |
| `id` | string | 권장 | group의 안정 ID. 보통 `pathSegments`를 `.`로 join한다. |
| `name` | string | 필수 | group leaf 이름. SDK property/class 이름의 source다. |
| `pathSegments` | string[] | 선택 | root부터 이 group까지의 노출 segment. 없으면 parser는 `[name]`으로 보정할 수 있다. |
| `requiredHeaders` | string[] | 선택 | 이 group boundary에서 새로 공급해야 하는 ambient required header의 호환용 wire-name summary. |
| `providerHeaders` | Parameter[] | 선택 | 이 group boundary에서 새로 공급해야 하는 typed provider header 계약. SDK generator는 이 값을 우선한다. |
| `endpoints` | Endpoint[] | 선택 | 이 group에 직접 속한 endpoints. |
| `groups` | Group[] | 선택 | child groups. |

Group provider header는 endpoint request field가 아니다. SDK client 또는 group client 생성 시 provider/callback/constructor argument로 공급되는 ambient header 계약이다. child group은 ancestor group의 provider를 함께 사용한다.

예를 들어 parent `branch`가 `Authorization`, `X-Branch-Id`를 요구하고 child `branch.booking`이 `X-Booking-Scope`만 추가하면:

- `branch.providerHeaders = [Authorization, X-Branch-Id]`
- `branch.booking.providerHeaders = [X-Booking-Scope]`
- `branch.booking` endpoint 호출 시 세 header 모두 필요하다.

## Endpoint

```json
{
  "name": "CreateBooking",
  "sdkName": "create",
  "method": "POST",
  "path": "/api/v1/branch/booking",
  "successStatus": 201,
  "requiredHeaders": ["Authorization", "X-Branch-Id", "X-Booking-Scope", "Idempotency-Key"],
  "request": {
    "headerParams": [
      {
        "name": "IdempotencyKey",
        "wireName": "Idempotency-Key",
        "type": { "kind": "int" },
        "required": true
      }
    ],
    "body": { "kind": "named", "name": "CreateBookingRequest" }
  },
  "response": {
    "envelope": true,
    "body": { "kind": "named", "name": "Booking" }
  },
  "error": {
    "body": { "kind": "named", "name": "DefaultError" }
  }
}
```

필드:

| 필드 | 타입 | 필수 | 의미 |
| --- | --- | --- | --- |
| `name` | string | 필수 | 원본 endpoint struct/type 이름. |
| `sdkName` | string | 선택 | SDK method 이름 override. 없으면 generator가 `name`에서 method 이름을 만든다. |
| `method` | string | 필수 | `GET`, `POST`, `PUT`, `PATCH`, `DELETE`, `HEAD`, `OPTIONS`. |
| `path` | string | 필수 | 최종 HTTP path. group prefix가 적용된 full path다. |
| `successStatus` | number | 필수 | 2xx 성공 status. 기본값은 200. |
| `group` | string | flat IR 선택 | flat endpoint를 단순 grouping할 때 쓰는 group 이름. group tree IR에서는 보통 생략된다. |
| `requiredHeaders` | string[] | 선택 | 이 endpoint 호출에 적용되는 header policy 전체 요약. inherited group provider header와 endpoint-level required header가 포함된다. |
| `request` | Request | 필수 | request 계약. |
| `response` | Response | 필수 | success response 계약. |
| `error` | Error | 선택 권장 | non-2xx error body 계약. 없으면 `DefaultError`로 해석한다. |

`path` 안의 path variable은 `{id}`처럼 중괄호를 사용한다. `request.pathParams[].wireName`은 이 중괄호 안 이름과 일치해야 한다.

## Request

```json
{
  "pathParams": [],
  "queryParams": [],
  "headerParams": [],
  "body": { "kind": "named", "name": "CreateBookingRequest" }
}
```

필드:

- `pathParams`: path template을 채우는 required parameter 목록.
- `queryParams`: URL query parameter 목록. 현재 Go runtime에서는 GET/DELETE request의 non-path, non-header exported field가 query parameter가 되며 IR에서는 항상 `required: false`다.
- `headerParams`: endpoint method 호출자가 넘기는 header parameter 목록. endpoint-level `RequireHeader`만 여기에 들어간다. group provider header는 여기에 넣지 않는다. struct binding이 있으면 binding field의 이름/type/optional 여부를 쓰고, 없으면 required `string` parameter를 합성한다.
- `body`: request JSON body type. body가 없으면 생략한다.

현재 IR parser는 body 계약을 `POST`, `PUT`, `PATCH`에만 만든다. GET/DELETE의 request field는 path/query/header로만 해석된다.

## Parameter

```json
{
  "name": "ID",
  "wireName": "id",
  "type": { "kind": "uuid" },
  "required": true
}
```

필드:

| 필드 | 타입 | 필수 | 의미 |
| --- | --- | --- | --- |
| `name` | string | 필수 | 원본 언어의 field/property 이름. |
| `wireName` | string | 필수 | HTTP wire 이름. path variable, query key, header name 중 하나다. |
| `type` | TypeRef | 필수 | parameter type. |
| `required` | bool | 필수 | 호출자가 반드시 값을 제공해야 하는지 여부. |

## Response

```json
{
  "envelope": true,
  "body": { "kind": "named", "name": "Booking" }
}
```

onedef success response는 204를 제외하고 envelope를 쓴다.

```json
{
  "code": "created",
  "title": "Created",
  "message": "success",
  "data": {}
}
```

규칙:

- `envelope: true`면 HTTP response body는 success envelope object이고, 실제 payload는 `data` 안에 있다. 이때 `body`가 있어야 한다.
- `envelope: false`면 success body가 없다. 현재 Go parser는 `successStatus == 204`이고 `Response struct{}`일 때 이 형태를 만든다.
- 204 response는 HTTP body를 쓰지 않는다.

## Error

```json
{
  "body": { "kind": "named", "name": "DefaultError" }
}
```

non-2xx response는 success envelope를 쓰지 않는다. 서버의 error policy가 반환한 body를 그대로 JSON으로 쓴다.

기본 error body는 built-in `DefaultError`다.

```json
{
  "code": "invalid_body",
  "message": "request body is invalid JSON",
  "details": {}
}
```

`DefaultError`는 built-in type으로 취급한다. `types`에 없을 수 있다. custom error policy를 쓰는 서버는 `error.body`를 custom named type으로 지정하고, 해당 type을 `types`에 포함해야 한다.

## TypeDef

```json
{
  "name": "Booking",
  "kind": "object",
  "fields": [
    {
      "name": "ID",
      "wireName": "id",
      "type": { "kind": "uuid" },
      "required": true
    }
  ]
}
```

필드:

| 필드 | 타입 | 필수 | 의미 |
| --- | --- | --- | --- |
| `name` | string | 필수 | type table 안에서 유일한 이름. |
| `kind` | string | 필수 | 현재 object model은 `"object"`를 사용한다. |
| `fields` | FieldDef[] | 선택 | object field 목록. |

`types`는 named type table이다. `TypeRef.kind == "named"`인 값은 보통 여기의 `name`을 참조한다. 예외는 built-in `DefaultError`다.

## FieldDef

```json
{
  "name": "Profile",
  "wireName": "profile",
  "type": { "kind": "named", "name": "UserProfile", "nullable": true },
  "required": false,
  "nullable": true,
  "omitEmpty": true
}
```

필드:

| 필드 | 타입 | 필수 | 의미 |
| --- | --- | --- | --- |
| `name` | string | 필수 | 원본 struct field 이름. |
| `wireName` | string | 필수 | JSON property 이름. |
| `type` | TypeRef | 필수 | field type. |
| `required` | bool | 필수 | JSON payload에 값이 있어야 하는지 여부. |
| `nullable` | bool | 선택 | null을 허용하는지 여부. 생략 시 false. |
| `omitEmpty` | bool | 선택 | encode 시 empty/null 값을 생략할 수 있는지 여부. 생략 시 false. |

Go parser 기준:

- exported field만 포함한다.
- anonymous field와 `json:"-"` field는 제외한다.
- `wireName`은 `json` tag의 첫 segment를 쓰고, tag가 없으면 lower camel case field 이름을 쓴다.
- pointer type은 `nullable: true`가 된다.
- `required`는 `!nullable && !omitempty`다.
- `omitempty`가 있으면 `omitEmpty: true`이고 `required: false`다.

## TypeRef

```json
{ "kind": "list", "elem": { "kind": "named", "name": "Booking" } }
```

공통 필드:

| 필드 | 타입 | 의미 |
| --- | --- | --- |
| `kind` | string | type kind. |
| `name` | string | `kind == "named"`일 때 참조 type 이름. |
| `nullable` | bool | null 허용 여부. 생략 시 false. |
| `elem` | TypeRef | `kind == "list"`일 때 element type. |
| `key` | TypeRef | `kind == "map"`일 때 key type. 현재는 string key만 지원한다. |
| `value` | TypeRef | `kind == "map"`일 때 value type. |

지원 kind:

| kind | JSON wire type | 비고 |
| --- | --- | --- |
| `any` | any JSON value | 언어별 dynamic/object type. |
| `bool` | boolean | |
| `int` | number | 정수. |
| `float` | number | 부동소수. |
| `string` | string | |
| `uuid` | string | UUID string. Go parser는 `github.com/google/uuid.UUID`를 이 타입으로 낸다. |
| `list` | array | `elem` 필수. |
| `map` | object | string key만 지원. `value` 필수. |
| `named` | object 또는 alias | `name` 필수. |

`object`는 현재 `TypeDef.kind`에서 쓰는 값이다. Go parser는 inline object `TypeRef`를 만들지 않고 named type으로 빼낸다. 서버 구현자는 inline object 대신 `types`에 named object를 추가하는 것을 권장한다.

## Go 정의에서 IR로 추출되는 규칙

endpoint struct는 다음 모양을 기준으로 한다.

```go
func Definition() *onedef.Spec {
    Authorization := onedef.Header[string]("Authorization")
    IdempotencyKey := onedef.Header[int]("Idempotency-Key")
    RequestID := onedef.Header[string]("X-Request-Id")

    return onedef.Group(
        "/api",
        onedef.RequireHeader(Authorization),
        onedef.Endpoint(
            &CreateUser{},
            onedef.RequireHeader(IdempotencyKey),
            onedef.RequireHeader(RequestID),
        ),
    )
}

type CreateUser struct {
    onedef.POST `path:"/users" status:"201"`
    Request     struct {
        Authorization  string  `header:"Authorization"`
        IdempotencyKey int     `header:"Idempotency-Key"`
        RequestID      *string `header:"X-Request-Id"`
        Name           string  `json:"name"`
    }
    Response User
}
```

추출 규칙:

- endpoint는 `onedef.GET`, `POST`, `PUT`, `PATCH`, `DELETE`, `HEAD`, `OPTIONS` 중 하나를 embed해야 한다.
- method marker에는 `path` tag가 필수다.
- `status` tag가 없으면 `successStatus`는 200이다. 있으면 200-299여야 한다.
- `Request` field와 `Response` field가 필요하다.
- path tag의 `{name}`은 path param이다. 같은 의미의 request field는 field 이름을 lowercase 처리하고 `_`를 제거해서 찾는다. 예를 들어 `{user_id}`는 `UserID`와 매칭된다.
- `header:"Name"` tag가 있는 request field는 새 header contract 선언이 아니라 `RequireHeader(Header[T])`에 붙는 typed binding이다.
- inherited group 또는 same endpoint의 matching `RequireHeader(Header[T])`가 없으면 producer는 reject해야 한다.
- group-bound header binding은 runtime typed access만 제공하고 SDK method parameter가 되지 않는다.
- endpoint-bound header binding은 `request.headerParams`가 된다.
- GET/DELETE에서 path/header가 아닌 exported request field는 query parameter다.
- POST/PUT/PATCH에서 path/header가 아닌 exported request field는 JSON body field다.
- request body가 일부 field만 포함하면 synthetic type 이름은 `<EndpointName>Request`다.
- response가 204 `struct{}`이면 `response.envelope`은 false다. 그 외에는 true이고 `response.body`가 있다.
- Provide, BeforeHandle, AfterHandle, Observe, runtime handler 구현은 IR shape에 들어가지 않는다.

주의: 현재 Go parser에서 특별 scalar로 취급하는 외부 struct는 `uuid.UUID`뿐이다. 날짜/시간 같은 도메인 scalar는 아직 별도 kind가 없으므로 서버와 클라이언트가 `string` 등으로 명시적으로 합의하는 편이 안전하다.

## 서버 산출 체크리스트

서버 또는 다른 언어의 IR producer는 다음을 만족해야 한다.

- 최상위 `version`을 `"v1"`로 쓴다.
- endpoint `path`는 full path로 쓴다.
- path template의 `{variable}`마다 `request.pathParams`에 같은 `wireName`의 parameter를 둔다.
- request/response/error/body/field의 모든 named type 참조를 `types`에 포함한다. 단 `DefaultError`는 built-in이므로 생략 가능하다.
- `types[].name`은 문서 안에서 유일해야 한다.
- 같은 이름의 type은 같은 shape이어야 한다.
- map key는 string으로 제한한다.
- group tree를 산출할 때 child group의 `requiredHeaders`에는 그 group boundary에서 새로 공급할 header만 둔다.
- endpoint의 `requiredHeaders`를 산출한다면 inherited group provider header와 endpoint-level required header를 포함한 전체 header policy 요약으로 둔다.
- `request.headerParams`에는 endpoint-level required header만 넣는다. struct binding이 있으면 binding field의 이름/type/optional 여부를 쓰고, 없으면 required `string` parameter를 합성한다.
- success 204는 `response.envelope: false`와 body 없음으로 표현한다.
- non-204 success는 `response.envelope: true`와 `body`를 둔다.

## 클라이언트 파서 체크리스트

클라이언트 parser/generator는 다음 흐름을 권장한다.

1. JSON object인지 확인한다.
2. `version == "v1"`인지 확인한다.
3. 생략된 `naming`, `groups`, `endpoints`, `types`, nested arrays를 빈 값으로 보정한다.
4. `types`를 `name -> TypeDef` table로 만든다. 중복 이름은 에러로 처리한다.
5. `DefaultError`는 built-in named type으로 등록한다.
6. 모든 `TypeRef`를 검증한다.
   - `named`는 `name`이 있어야 한다.
   - `list`는 `elem`이 있어야 한다.
   - `map`은 string key와 `value`가 있어야 한다.
   - recursive type을 무한히 펼치지 말고 참조로 유지한다.
7. `groups`를 DFS/BFS로 순회해 endpoints를 수집한다.
8. top-level `endpoints`도 root-level endpoints로 함께 처리한다.
9. endpoint path, method, status, path param 매칭을 검증한다.
10. HTTP client를 만들 때:
    - path param은 `{wireName}` 위치에 URL-escaped 값으로 치환한다.
    - query param은 값이 있을 때만 query string에 넣는다.
    - group `requiredHeaders`는 provider/constructor/header context로 공급한다.
    - `request.headerParams`는 method parameter로 받고 header에 넣는다. string 외 scalar도 header wire 값으로 넣기 전에 문자열화한다.
    - request body는 `request.body` type에 맞게 JSON으로 encode한다.
    - 204 success는 body를 decode하지 않는다.
    - envelope success는 JSON object의 `data`를 `response.body` type으로 decode한다.
    - non-2xx error는 `error.body` type으로 decode한다.

## 예시

```json
{
  "version": "v1",
  "naming": {
    "initialisms": ["ID"]
  },
  "groups": [
    {
      "id": "user",
      "name": "user",
      "pathSegments": ["user"],
      "requiredHeaders": ["Authorization"],
      "endpoints": [
        {
          "name": "CreateUser",
          "sdkName": "create",
          "method": "POST",
          "path": "/api/v1/users",
          "successStatus": 201,
          "requiredHeaders": ["Authorization", "Idempotency-Key"],
          "request": {
            "headerParams": [
              {
                "name": "IdempotencyKey",
                "wireName": "Idempotency-Key",
                "type": { "kind": "int" },
                "required": true
              }
            ],
            "body": { "kind": "named", "name": "CreateUserRequest" }
          },
          "response": {
            "envelope": true,
            "body": { "kind": "named", "name": "User" }
          },
          "error": {
            "body": { "kind": "named", "name": "DefaultError" }
          }
        },
        {
          "name": "DeleteUser",
          "method": "DELETE",
          "path": "/api/v1/users/{id}",
          "successStatus": 204,
          "requiredHeaders": ["Authorization"],
          "request": {
            "pathParams": [
              {
                "name": "ID",
                "wireName": "id",
                "type": { "kind": "uuid" },
                "required": true
              }
            ]
          },
          "response": {
            "envelope": false
          },
          "error": {
            "body": { "kind": "named", "name": "DefaultError" }
          }
        }
      ]
    }
  ],
  "types": [
    {
      "name": "CreateUserRequest",
      "kind": "object",
      "fields": [
        {
          "name": "Name",
          "wireName": "name",
          "type": { "kind": "string" },
          "required": true
        }
      ]
    },
    {
      "name": "User",
      "kind": "object",
      "fields": [
        {
          "name": "ID",
          "wireName": "id",
          "type": { "kind": "uuid" },
          "required": true
        },
        {
          "name": "Name",
          "wireName": "name",
          "type": { "kind": "string" },
          "required": true
        },
        {
          "name": "Aliases",
          "wireName": "aliases",
          "type": {
            "kind": "list",
            "elem": { "kind": "string" }
          },
          "required": false,
          "omitEmpty": true
        }
      ]
    }
  ]
}
```

## 호환성 메모

- v1 parser는 additive field를 무시해야 한다.
- v1 producer는 기존 필드 의미를 바꾸지 않아야 한다.
- 새로운 scalar kind가 추가되면 클라이언트 generator는 모르는 kind를 명시적 에러로 처리하는 편이 좋다. 임의로 `any`로 낮추면 계약 깨짐을 놓칠 수 있다.
- flat `endpoints`만 있는 문서는 여전히 유효하다. 다만 group-level header provider와 nested client 구조가 필요하면 `groups`를 사용한다.
