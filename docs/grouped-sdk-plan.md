# Group Tree + SDK 생성 구현 계획

## 개요

이 문서는 `onedef`의 다음 방향을 구현하기 위한 세부 계획이다.

- 서버 endpoint는 **그룹 트리** 안에서 등록된다.
- 그룹은 path prefix와 required header 정책을 가진다.
- endpoint는 자기 **리프 path**와 자기 **엔드포인트 전용 요청/응답**만 정의한다.
- SDK 생성은 파일 스캔이 아니라 **실제로 등록된 그룹 + endpoint 결과**를 기준으로 한다.
- Dart SDK는 **group tree를 그대로 반영한 client 구조**를 생성한다.
- 사용자 입장에서는 `ApiClient` 하나만 보이고, 그룹별 required header 값은 **client/group 생성 시 async callback provider**로 주입한다.

이 방향의 목표는 다음 3가지를 동시에 만족하는 것이다.

- 계약 일치: 서버에 실제 등록된 결과만 SDK에 반영
- 단순함: 별도 manifest/spec/config 파일 최소화
- sqlc스러운 통제성: 계약에서 파생 가능한 interface/transport mismatch를 강하게 자동화

---

## 핵심 철학

### onedef가 책임지는 것

- endpoint 등록 구조
- 그룹 트리와 path prefix
- 그룹/엔드포인트 required header 계약
- request/path/query/body/header shape 추출
- response envelope / typed success-error 모델
- SDK client / sub-group class 생성
- 모델 encode/decode

### onedef가 책임지지 않는 것

- token refresh
- session lifecycle
- auth scope의 도메인 의미
- default header merge 정책
- Bearer prefix 자동화
- custom signing
- logging/reporting integration
- IP/User-Agent 기반 운영 정책

즉 `onedef`는 auth framework가 아니라 **typed contract generator**에 가깝게 유지한다.

---

## 구현 범위

### 이번 범위에 포함

- 계층적 group registration DSL
- group-level required header
- endpoint-level required header
- 제한적 `OmitHeader("Authorization")`
- group tree 기반 SDK generation
- Dart generated client의 nested group structure
- client 생성 시 async callback provider 주입

### 이번 범위에서 제외

- `GenerateRegistry`
- runtime session/token refresh
- default header injection / merge 정책
- header override 정책
- generic `OmitHeader(any)` 지원
- auth scope / customer/staff/branch 의미 해석
- custom transport policy 표준화

---

## Public API 방향

### 서버 측

기존 `Register(endpoints ...any)`만으로는 group tree 정보를 표현할 수 없으므로, group/endpoint를 함께 표현하는 DSL이 필요하다.

권장 shape:

```go
func (a *App) Group(path string, children ...Node) GroupRef

func RequireHeader(name string) Node
func OmitHeader(name string) Node
func Endpoint(endpoint any) Node
func Group(path string, children ...Node) Node
```

또는 `App.Group(...)`와 package-level `onedef.Group(...)`을 함께 둬도 된다.

최종 사용 예시는 다음과 비슷해야 한다.

```go
func Register(app *onedef.App) {
    app.Group("/api/v1",
        onedef.Group("/branch",
            onedef.RequireHeader("Authorization"),
            onedef.RequireHeader("X-Branch-Id"),

            onedef.Group("/booking",
                onedef.RequireHeader("X-Booking-Scope"),

                onedef.Endpoint(&GetBooking{}),
                onedef.Endpoint(&CreateBooking{}),
            ),
        ),

        onedef.Group("/customer",
            onedef.RequireHeader("Authorization"),
            onedef.RequireHeader("X-Customer-Id"),

            onedef.Group("/booking",
                onedef.Endpoint(&GetCustomerBooking{}),
            ),
        ),

        onedef.Group("/notices",
            onedef.OmitHeader("Authorization"),

            onedef.Endpoint(&ListNotices{}),
            onedef.Endpoint(&GetNotice{}),
        ),
    )
}
```

### endpoint struct

endpoint는 기존 onedef 스타일을 최대한 유지하되, endpoint 전용 헤더는 `Request` 필드 태그로 표현한다.

```go
type CreateBooking struct {
    onedef.POST `path:"" status:"201"`
    Request     struct {
        IdempotencyKey string `header:"Idempotency-Key"`
        Name           string `json:"name"`
        StartsAt       time.Time `json:"startsAt"`
    }
    Response Booking
}
```

여기서 `path:""` 또는 `path:"/{id}"` 같은 리프 경로만 적는다. 최종 path는 상위 group prefix와 합쳐서 계산한다.

---

## 서버 메타데이터 모델

내부적으로는 다음 두 계층이 필요하다.

### Group metadata

```go
type GroupMeta struct {
    Name            string
    PathPrefix      string
    RequiredHeaders []string
    Children        []*GroupMeta
    Endpoints       []EndpointMeta
}
```

### Endpoint metadata

기존 `meta.EndpointStruct`를 확장하거나 대체한다.

최소 필요 필드:

- StructName
- Method
- LeafPath
- FullPath
- SuccessStatus
- Request
- Dependencies
- StructType
- GroupPath
- InheritedRequiredHeaders
- EndpointRequiredHeaders
- FinalRequiredHeaders

중요:

- SDK 생성은 `FinalRequiredHeaders`를 본다.
- 최종 `FullPath`는 group prefix + leaf path로 계산한다.
- endpoint-specific `header:"..."`들은 `EndpointRequiredHeaders`로 분리해둔다.

---

## Header 상속 규칙

### 기본 규칙

- 부모 `RequireHeader`는 자식에게 누적 상속된다.
- 자식 group은 header를 추가만 가능하다.
- endpoint는 endpoint 전용 header를 추가할 수 있다.

### 금지 규칙

- 부모 header 제거는 기본적으로 불가
- 같은 이름의 header를 상위/하위/endpoint에서 중복 선언하는 것은 생성 에러

이렇게 해서 hidden precedence policy를 제거한다.

### 예외 규칙

이번 범위에서는 `Authorization`에 한정해서 `OmitHeader("Authorization")`를 허용한다.

목적:

- `/notices` 같은 public API를 예쁜 path로 유지
- `/public/...` / `/private/...` 같은 URL 강제를 피함

제약:

- v1에서는 `Authorization` 외 헤더에 대한 ignore는 지원하지 않음
- 상위에 `Authorization`이 없는데 `OmitHeader("Authorization")`가 오면 에러

---

## Path 합성 규칙

- group path는 prefix 역할
- endpoint struct path는 leaf path 역할
- 최종 path는 `join(parent prefixes..., endpoint leaf path)`

예:

- group `/api/v1`
- child `/branch`
- child `/booking`
- endpoint `/{id}`

최종 path:

```text
/api/v1/branch/booking/{id}
```

Path join 규칙은 중복 slash 없이 정규화해야 한다.

---

## SDK 생성 기준

### 중요 원칙

SDK 생성은 **등록된 결과**를 기준으로 한다.

즉:

1. `app := onedef.New()`
2. `api.Register(app)`
3. `app.GenerateSDK(...)`

흐름이어야 한다.

이유:

- endpoint struct만 보면 group prefix를 모름
- endpoint struct만 보면 inherited required headers를 모름
- group tree 안에 실제로 등록되지 않은 endpoint는 SDK에 들어가면 안 됨

### 권장 public API

```go
type GenerateSDKOptions struct {
    OutDir      string
    PackageName string
}

func (a *App) GenerateSDK(opts GenerateSDKOptions) error
```

이번 범위에서 `GenerateRegistry`는 구현하지 않는다.

---

## Dart generated client shape

### 설계 원칙

- 사용자는 `ApiClient` 하나만 본다.
- group는 property 형태로 노출한다.
- group-level required header 값은 `ApiClient` 생성 시 **async callback provider**로 주입한다.
- endpoint-specific header는 해당 메서드 인자로 받는다.

### 목표 사용 예시

```dart
final api = ApiClient(
  baseUrl: 'https://api.example.com',
  authorization: () async => await tokenStore.branchToken(),
  xBranchId: () async => await currentBranchId(),
  xBookingScope: () async => 'domestic',
  xCustomerId: () async => await currentCustomerId(),
);

final booking = await api.branch.booking.getBooking(id: 'bk_123');

final created = await api.branch.booking.createBooking(
  idempotencyKey: 'req_001',
  body: CreateBookingRequest(...),
);
```

### 생성 규칙

- root group -> `ApiClient` property
- child group -> parent group property
- endpoint -> leaf group method

즉 flat `BranchBookingApi`를 외부에 직접 노출하지 않고, 접근은:

```dart
api.branch.booking.getBooking(...)
```

형태로 간다.

### group provider 예시

```dart
typedef HeaderValueProvider = Future<String> Function();

class BranchGroup {
  BranchGroup(
    this._transport, {
    required this.authorization,
    required this.xBranchId,
    required this.xBookingScope,
  });

  final HeaderValueProvider authorization;
  final HeaderValueProvider xBranchId;
  final HeaderValueProvider xBookingScope;
}
```

### endpoint-specific header 예시

`CreateBooking`의 `Idempotency-Key`는 group provider가 아니라 메서드 인자로 생성한다.

```dart
Future<Result<Booking, DefaultError>> createBooking({
  required String idempotencyKey,
  required CreateBookingRequest body,
})
```

이렇게 하면 group-level contract와 endpoint-level contract가 분리된다.

---

## Dart transport 경계

Transport는 얇게 유지한다.

```dart
abstract interface class Transport {
  Future<TransportResponse> send(TransportRequest request);
}
```

generated client의 책임:

- group provider callback 호출
- endpoint-specific headers 수집
- 최종 headers map 조립
- path/query/body 조립
- transport 호출
- envelope decode

transport의 책임:

- 실제 HTTP 요청 전송
- raw response 반환

즉 refresh/session/policy는 transport 또는 사용자 코드가 책임지지, onedef core가 책임지지 않는다.

---

## Validation 규칙

### 서버 측 validation

- group path는 `/`로 시작해야 함
- endpoint leaf path는 `/` 또는 empty string이어야 함
- unsupported `OmitHeader`는 에러
- 상위에 없는 `OmitHeader("Authorization")`는 에러
- 중복 header 선언은 에러
- endpoint `header` field는 exported field만 허용
- 기존 request/deps/response validation은 유지

### SDK generation validation

- group provider가 필요한 group는 생성된 constructor에서 required field로 강제
- provider callback 누락은 compile-time에서 드러나게 generated constructor shape를 만든다

주의:

- callback이 runtime에 빈 문자열을 돌려주는 것까지 막지는 않는다
- 이는 transport/auth policy의 영역이다

---

## 구현 단계

### Phase 1: group DSL + metadata

- Node / GroupNode / EndpointNode 내부 모델 추가
- `App.Group(...)` / `onedef.Group(...)` / `RequireHeader(...)` / `OmitHeader(...)` / `Endpoint(...)` 추가
- group tree walk 구현
- full path + final required headers 계산

### Phase 2: endpoint header parsing

- `Request` struct에서 `header:"..."` 태그 파싱 지원
- endpoint-specific header를 메타데이터에 별도 저장
- 기존 json/path/query parsing과 충돌 없게 정리

### Phase 3: runtime registration

- group tree로부터 실제 mux registration 수행
- 기존 `Register(endpoints ...any)`와 공존 여부 결정
- 가능하면 v1에서는 기존 `Register()`는 유지하되, 새로운 group DSL 경로를 추가

### Phase 4: SDK generation

- `App.GenerateSDK(opts)` 추가
- IR 대신 등록된 메타데이터를 직접 소비
- Dart generated client를 nested group structure로 변경
- group class가 header provider를 직접 받도록 생성
- endpoint-specific header params generation 추가

### Phase 5: examples / docs / tests

- `example/hello_world` 또는 e2e 예제를 group DSL로 갱신
- public notices / private branch/customer 같은 구조 예시 추가
- 문서 업데이트

---

## 테스트 계획

### 서버 메타 테스트

- group path prefix 합성
- required header 누적 상속
- `OmitHeader("Authorization")` 처리
- duplicate header 선언 시 실패
- endpoint-specific header 파싱

### runtime 테스트

- 등록된 full path로 정상 routing
- 상위/하위 group 구성에서도 handler 호출 정상 동작

### SDK generation 테스트

- generated Dart tree가 `api.branch.booking.getBooking()` 형태를 반영
- group class가 header provider를 직접 받음
- endpoint-specific header가 메서드 인자로 생성
- public group는 auth provider가 없도록 생성

### e2e 테스트

- branch/customer/notices를 포함한 예제로 SDK 생성
- 생성된 SDK로 실제 호출

---

## 기존 코드와의 관계

현재 repo에는 이미 IR / generator 관련 작업 흔적이 있으나, 이번 방향에선 다음을 우선한다.

- 등록된 결과 기반 SDK generation
- group tree 기반 메타데이터
- `GenerateRegistry`는 보류

따라서 구현 에이전트는 IR 확장보다 **group tree 메타 모델**을 중심으로 작업해야 한다.

---

## 최종 요약

이번 설계는 다음 문장으로 요약된다.

> onedef는 그룹 트리 안에 실제로 등록된 계약을 기준으로, path/header 상속을 반영한 typed SDK를 생성한다.

그리고 Dart 쪽 UX는 다음 문장으로 요약된다.

> 사용자는 `ApiClient` 하나를 만들고, 그룹별 required header 값은 async provider로 주입하며, endpoint-specific header만 메서드 인자로 넘긴다.
