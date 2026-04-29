# Dependency Injection 구현 계획

## 개요

onedef DI v1의 목표는 endpoint struct 안에서 의존성을 선언하고, 앱 부트스트랩 단계에서 타입 기반으로 안전하게 주입하는 것이다.

핵심 원칙:

- endpoint는 선택적으로 `Deps` 필드를 가진다.
- 앱은 `app.Inject(onedef.Dependency(...))`로 dependency를 등록한다.
- dependency 누락과 잘못된 `Deps` 선언은 `Register()` 시점에 fail-fast로 검증한다.
- runtime에는 검증된 dependency snapshot만 주입한다.

---

## Public API

```go
func Dependency[T any](value T) DependencyBinding
func (a *App) Inject(bindings ...DependencyBinding)
```

사용 예시:

```go
repo := &PostgresUserRepo{db: db}
logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

app.Inject(
    onedef.Dependency(repo),
    onedef.Dependency[UserRepo](repo),
    onedef.Dependency(logger),
)
```

규칙:

- `Dependency[T]`는 `reflect.TypeFor[T]()`를 key로 사용한다.
- nil / typed nil dependency는 즉시 panic 한다.
- 동일한 타입 key를 두 번 `Inject()`하면 panic 한다.
- 같은 concrete를 concrete 타입과 interface 타입으로 각각 등록하는 것은 허용한다.

---

## `Deps` 계약

endpoint는 선택적으로 `Deps` 필드를 선언할 수 있다.

```go
type GetUserAPI struct {
    onedef.GET `path:"/users/{id}"`
    Request  struct{ ID string }
    Response User
    Deps     struct {
        Users  UserRepo
        Logger *slog.Logger
    }
}
```

허용 규칙:

- `Deps`가 없어도 된다.
- `Deps`가 있으면 value struct만 허용한다.
- `Deps` 내부 필드는 exported, non-anonymous 여야 한다.
- exact type match로만 주입한다.

비허용 예시:

```go
Deps any
Deps *MyDeps
Deps struct {
    users UserRepo
    *slog.Logger
}
```

즉 `Deps.Users UserRepo`를 쓰면 반드시 `onedef.Dependency[UserRepo](repo)`로 등록해야 한다.

---

## 동작 방식

1. `Inject()`는 앱 내부 registry에 dependency를 타입별로 저장한다.
2. 첫 `Register()` 호출부터 dependency registry는 잠긴다.
3. 이후 `Inject()`를 다시 호출하면 panic 한다.
4. 각 endpoint 등록 시 `Deps` shape와 dependency 누락을 검증한다.
5. 검증된 dependency snapshot을 handler에 고정해 runtime에 주입한다.

중요한 제약:

- v1은 startup-only DI다.
- `Inject()`와 `Register()`를 섞어 쓰는 고급 워크플로는 지원하지 않는다.
- request-scoped DI, named DI, lifecycle hooks, auto-construction은 범위 밖이다.

---

## 테스트 기준

- `Dependency(repo)`는 concrete type key로 등록된다.
- `Dependency[UserRepo](repo)`는 interface type key로 등록된다.
- nil / typed nil, duplicate inject, missing dependency는 panic 한다.
- `Deps any`, `Deps *MyDeps`, anonymous field, unexported field는 등록 시 실패한다.
- 실제 HTTP 요청에서 `Deps`가 주입되어 `Handle()`이 정상 동작한다.
- Dart SDK generation 결과에는 `Deps`가 노출되지 않는다.
