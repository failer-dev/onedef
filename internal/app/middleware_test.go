package app

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/failer-dev/onedef/internal/meta"
)

type middlewareOKEndpoint struct {
	meta.GET `path:""`
	Request  struct{}
	Response struct {
		OK bool `json:"ok"`
	}
}

func (h *middlewareOKEndpoint) Handle(context.Context) error {
	h.Response.OK = true
	return nil
}

type middlewareErrorEndpoint struct {
	meta.GET `path:""`
	Request  struct{}
	Response struct{}
}

func (h *middlewareErrorEndpoint) Handle(context.Context) error {
	return errors.New("endpoint boom")
}

type middlewarePolicyError struct {
	Path    string `json:"path"`
	Message string `json:"message"`
}

func TestAppMiddleware_OrderIsParentChildEndpoint(t *testing.T) {
	t.Parallel()

	app := New()
	calls := make([]string, 0)
	app.Group(
		"/api",
		meta.Use(recordingMiddleware("parent", &calls)),
		meta.Group(
			"/items",
			meta.Use(recordingMiddleware("child", &calls)),
			meta.Endpoint(
				&middlewareOKEndpoint{},
				meta.EndpointMiddleware(recordingMiddleware("endpoint", &calls)),
			),
		),
	)

	req := httptest.NewRequest(http.MethodGet, "/api/items", nil)
	res := httptest.NewRecorder()

	app.mux.ServeHTTP(res, req)

	assertSuccessResponse(t, res, http.StatusOK, "ok", "OK", "success")
	want := strings.Join([]string{
		"parent:before",
		"child:before",
		"endpoint:before",
		"endpoint:after",
		"child:after",
		"parent:after",
	}, ",")
	if got := strings.Join(calls, ","); got != want {
		t.Fatalf("middleware calls = %q, want %q", got, want)
	}
}

func TestAppMiddleware_SkipGroupMiddleware(t *testing.T) {
	t.Parallel()

	app := New()
	auth := blockingAuthMiddleware()
	app.Group(
		"/api",
		meta.Use(auth),
		meta.Group(
			"/public",
			meta.SkipMiddleware("auth"),
			meta.Endpoint(&middlewareOKEndpoint{}),
		),
		meta.Group(
			"/private",
			meta.Endpoint(&middlewareOKEndpoint{}),
		),
	)

	publicReq := httptest.NewRequest(http.MethodGet, "/api/public", nil)
	publicRes := httptest.NewRecorder()
	app.mux.ServeHTTP(publicRes, publicReq)
	assertSuccessResponse(t, publicRes, http.StatusOK, "ok", "OK", "success")

	privateReq := httptest.NewRequest(http.MethodGet, "/api/private", nil)
	privateRes := httptest.NewRecorder()
	app.mux.ServeHTTP(privateRes, privateReq)
	assertErrorResponse(t, privateRes, http.StatusUnauthorized, "blocked", "Unauthorized", "blocked")
}

func TestAppMiddleware_SkipEndpointMiddleware(t *testing.T) {
	t.Parallel()

	app := New()
	app.Group(
		"/api",
		meta.Use(blockingAuthMiddleware()),
		meta.Group(
			"/endpoint-public",
			meta.Endpoint(&middlewareOKEndpoint{}, meta.SkipEndpointMiddleware("auth")),
		),
		meta.Group(
			"/endpoint-private",
			meta.Endpoint(&middlewareOKEndpoint{}),
		),
	)

	publicReq := httptest.NewRequest(http.MethodGet, "/api/endpoint-public", nil)
	publicRes := httptest.NewRecorder()
	app.mux.ServeHTTP(publicRes, publicReq)
	assertSuccessResponse(t, publicRes, http.StatusOK, "ok", "OK", "success")

	privateReq := httptest.NewRequest(http.MethodGet, "/api/endpoint-private", nil)
	privateRes := httptest.NewRecorder()
	app.mux.ServeHTTP(privateRes, privateReq)
	assertErrorResponse(t, privateRes, http.StatusUnauthorized, "blocked", "Unauthorized", "blocked")
}

func TestAppMiddleware_CustomErrorPolicyHandlesEndpointAndMiddlewareErrors(t *testing.T) {
	t.Parallel()

	app := New()
	app.Group(
		"/api",
		meta.ErrorPolicy(func(_ *http.Request, rerr error) (int, middlewarePolicyError) {
			return http.StatusTeapot, middlewarePolicyError{Message: rerr.Error()}
		}),
		meta.Group(
			"/endpoint-error",
			meta.Endpoint(&middlewareErrorEndpoint{}),
		),
		meta.Group(
			"/middleware-error",
			meta.Endpoint(
				&middlewareOKEndpoint{},
				meta.EndpointMiddleware(errorMiddleware("middleware boom")),
			),
		),
	)

	endpointReq := httptest.NewRequest(http.MethodGet, "/api/endpoint-error", nil)
	endpointRes := httptest.NewRecorder()
	app.mux.ServeHTTP(endpointRes, endpointReq)
	if endpointRes.Code != http.StatusTeapot {
		t.Fatalf("endpoint status = %d, want %d", endpointRes.Code, http.StatusTeapot)
	}
	var endpointBody middlewarePolicyError
	decodeJSONBody(t, strings.NewReader(endpointRes.Body.String()), &endpointBody)
	if endpointBody.Message != "endpoint boom" {
		t.Fatalf("endpoint error body = %#v", endpointBody)
	}

	middlewareReq := httptest.NewRequest(http.MethodGet, "/api/middleware-error", nil)
	middlewareRes := httptest.NewRecorder()
	app.mux.ServeHTTP(middlewareRes, middlewareReq)
	if middlewareRes.Code != http.StatusTeapot {
		t.Fatalf("middleware status = %d, want %d", middlewareRes.Code, http.StatusTeapot)
	}
	var middlewareBody middlewarePolicyError
	decodeJSONBody(t, strings.NewReader(middlewareRes.Body.String()), &middlewareBody)
	if middlewareBody.Message != "middleware boom" {
		t.Fatalf("middleware error body = %#v", middlewareBody)
	}
}

func TestAppMiddleware_RecoverTurnsPanicIntoInternalError(t *testing.T) {
	t.Parallel()

	app := New()
	app.Group(
		"/api",
		meta.Use(meta.Recover(), panicMiddleware()),
		meta.Endpoint(&middlewareOKEndpoint{}),
	)

	req := httptest.NewRequest(http.MethodGet, "/api", nil)
	res := httptest.NewRecorder()

	app.mux.ServeHTTP(res, req)

	assertErrorResponse(t, res, http.StatusInternalServerError, "internal_error", "Internal Server Error", "internal server error")
}

func TestAppMiddleware_HTTPMiddlewareAdapter(t *testing.T) {
	t.Parallel()

	app := New()
	app.Group(
		"/api",
		meta.Endpoint(
			&middlewareOKEndpoint{},
			meta.EndpointMiddleware(meta.HTTPMiddleware(func(next http.Handler) http.Handler {
				return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("X-Std-Middleware", "yes")
					next.ServeHTTP(w, r)
				})
			})),
		),
	)

	req := httptest.NewRequest(http.MethodGet, "/api", nil)
	res := httptest.NewRecorder()

	app.mux.ServeHTTP(res, req)

	assertSuccessResponse(t, res, http.StatusOK, "ok", "OK", "success")
	if got := res.Header().Get("X-Std-Middleware"); got != "yes" {
		t.Fatalf("X-Std-Middleware = %q, want yes", got)
	}
}

func TestAppMiddleware_GenerateIRJSONIgnoresMiddlewareNodes(t *testing.T) {
	t.Parallel()

	app := New()
	app.Group(
		"/api/v1",
		meta.Use(meta.NamedMiddleware("auth", passMiddleware())),
		meta.Group(
			"/public",
			meta.SkipMiddleware("auth"),
			meta.Endpoint(
				&GroupedSpecTestEndpoint{},
				meta.EndpointMiddleware(passMiddleware()),
			),
		),
	)

	specJSON, err := app.GenerateIRJSON(GenerateIROptions{})
	if err != nil {
		t.Fatalf("GenerateIRJSON() error = %v", err)
	}
	if !strings.Contains(string(specJSON), `"path": "/api/v1/public/{id}"`) {
		t.Fatalf("spec = %s, want public endpoint path", specJSON)
	}
}

func TestAppMiddleware_PanicsForDuplicateActiveMiddlewareName(t *testing.T) {
	t.Parallel()

	app := New()
	defer func() {
		msg, ok := panicMessage(recover())
		if !ok {
			t.Fatal("expected panic, got nil")
		}
		if !strings.Contains(msg, "duplicate middleware name") {
			t.Fatalf("panic = %q, want duplicate middleware name", msg)
		}
	}()

	app.Group(
		"/api",
		meta.Use(meta.NamedMiddleware("auth", passMiddleware())),
		meta.Group(
			"/private",
			meta.Use(meta.NamedMiddleware("auth", passMiddleware())),
			meta.Endpoint(&middlewareOKEndpoint{}),
		),
	)
}

func TestAppMiddleware_AllowsSkipThenReaddMiddlewareName(t *testing.T) {
	t.Parallel()

	app := New()
	app.Group(
		"/api",
		meta.Use(blockingAuthMiddleware()),
		meta.Group(
			"/private",
			meta.SkipMiddleware("auth"),
			meta.Use(meta.NamedMiddleware("auth", passMiddleware())),
			meta.Endpoint(&middlewareOKEndpoint{}),
		),
	)

	req := httptest.NewRequest(http.MethodGet, "/api/private", nil)
	res := httptest.NewRecorder()

	app.mux.ServeHTTP(res, req)

	assertSuccessResponse(t, res, http.StatusOK, "ok", "OK", "success")
}

func recordingMiddleware(name string, calls *[]string) meta.Middleware {
	return meta.MiddlewareFunc(func(next meta.HandlerFunc) meta.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) error {
			*calls = append(*calls, name+":before")
			err := next(w, r)
			*calls = append(*calls, name+":after")
			return err
		}
	})
}

func blockingAuthMiddleware() meta.Middleware {
	return meta.NamedMiddleware("auth", meta.MiddlewareFunc(func(next meta.HandlerFunc) meta.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) error {
			return meta.Unauthorized("blocked", "blocked", nil)
		}
	}))
}

func errorMiddleware(message string) meta.Middleware {
	return meta.MiddlewareFunc(func(next meta.HandlerFunc) meta.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) error {
			return errors.New(message)
		}
	})
}

func panicMiddleware() meta.Middleware {
	return meta.MiddlewareFunc(func(next meta.HandlerFunc) meta.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) error {
			panic("boom")
		}
	})
}

func passMiddleware() meta.Middleware {
	return meta.MiddlewareFunc(func(next meta.HandlerFunc) meta.HandlerFunc {
		return next
	})
}
