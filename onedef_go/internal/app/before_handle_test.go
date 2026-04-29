package app

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/failer-dev/onedef/onedef_go/internal/meta"
)

type explicitHeaderValue struct {
	Value string
}

type textHeaderValue struct {
	Value string
}

func (v *textHeaderValue) UnmarshalText(text []byte) error {
	v.Value = "text:" + string(text)
	return nil
}

type parsedHeaderEndpoint struct {
	meta.GET `path:"/parsed"`
	Request  struct {
		Explicit explicitHeaderValue `header:"X-Explicit"`
		Text     textHeaderValue     `header:"X-Text"`
		Count    int                 `header:"X-Count"`
	}
	Response struct {
		Value string `json:"value"`
	}
}

func (h *parsedHeaderEndpoint) Handle(context.Context) error {
	h.Response.Value = fmt.Sprintf("%s/%s/%d", h.Request.Explicit.Value, h.Request.Text.Value, h.Request.Count)
	return nil
}

func TestAppHeaderParsing_ExplicitTextUnmarshalerAndScalar(t *testing.T) {
	t.Parallel()

	explicit := meta.NewHeader[explicitHeaderValue](
		"X-Explicit",
		meta.HeaderParse(func(raw string) (explicitHeaderValue, error) {
			return explicitHeaderValue{Value: "explicit:" + raw}, nil
		}),
	)
	text := meta.NewHeader[textHeaderValue]("X-Text")
	count := meta.NewHeader[int]("X-Count")
	app := newGroupedTestApp(
		"/api",
		meta.Endpoint(
			&parsedHeaderEndpoint{},
			meta.RequireHeader(explicit),
			meta.RequireHeader(text),
			meta.RequireHeader(count),
		),
	)

	req := httptest.NewRequest(http.MethodGet, "/api/parsed", nil)
	req.Header.Set("X-Explicit", "a")
	req.Header.Set("X-Text", "b")
	req.Header.Set("X-Count", "7")
	res := httptest.NewRecorder()
	app.mux.ServeHTTP(res, req)

	got := assertSuccessResponse(t, res, http.StatusOK, "ok", "OK", "success")
	data := resSuccessData(t, got)
	if data["value"] != "explicit:a/text:b/7" {
		t.Fatalf("response data = %+v, want parsed headers", data)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/parsed", nil)
	req.Header.Set("X-Explicit", "a")
	req.Header.Set("X-Text", "b")
	req.Header.Set("X-Count", "nope")
	res = httptest.NewRecorder()
	app.mux.ServeHTTP(res, req)
	assertErrorResponse(t, res, http.StatusBadRequest, "invalid_header_parameter", "Bad Request", "cannot convert value")
}

type beforeRecorder struct {
	Calls []string
}

type firstBefore struct {
	Provide struct {
		Recorder *beforeRecorder
		Value    string
	}
}

func (h *firstBefore) BeforeHandle(context.Context) error {
	h.Provide.Recorder.Calls = append(h.Provide.Recorder.Calls, "first")
	h.Provide.Value = "first"
	return nil
}

type secondBefore struct {
	Provide struct {
		Recorder *beforeRecorder
		Value    string
	}
}

func (h *secondBefore) BeforeHandle(context.Context) error {
	h.Provide.Recorder.Calls = append(h.Provide.Recorder.Calls, "second:"+h.Provide.Value)
	h.Provide.Value = "second"
	return nil
}

type beforeProvideEndpoint struct {
	meta.GET `path:"/before"`
	Request  struct{}
	Provide  struct {
		Recorder *beforeRecorder
		Value    string
	}
	Response struct {
		Value string `json:"value"`
	}
}

func (h *beforeProvideEndpoint) Handle(context.Context) error {
	h.Provide.Recorder.Calls = append(h.Provide.Recorder.Calls, "handle:"+h.Provide.Value)
	h.Response.Value = h.Provide.Value
	return nil
}

func TestAppBeforeHandle_OrderAndProvideLastWriterWins(t *testing.T) {
	t.Parallel()

	recorder := &beforeRecorder{}
	app := newGroupedTestApp(
		"/api",
		meta.NewProvideBinding(recorder),
		meta.BeforeHandle(&firstBefore{}),
		meta.BeforeHandle(&secondBefore{}),
		meta.Endpoint(&beforeProvideEndpoint{}),
	)

	req := httptest.NewRequest(http.MethodGet, "/api/before", nil)
	res := httptest.NewRecorder()
	app.mux.ServeHTTP(res, req)

	got := assertSuccessResponse(t, res, http.StatusOK, "ok", "OK", "success")
	data := resSuccessData(t, got)
	if data["value"] != "second" {
		t.Fatalf("response data = %+v, want second", data)
	}
	if strings.Join(recorder.Calls, ",") != "first,second:first,handle:second" {
		t.Fatalf("calls = %#v, want declaration order and last writer", recorder.Calls)
	}
}

type blockingBefore struct{}

func (h *blockingBefore) BeforeHandle(context.Context) error {
	return meta.Forbidden("blocked", "blocked before handle", nil)
}

func TestAppBeforeHandle_ErrorShortCircuits(t *testing.T) {
	t.Parallel()

	recorder := &beforeRecorder{}
	app := newGroupedTestApp(
		"/api",
		meta.NewProvideBinding(recorder),
		meta.BeforeHandle(&blockingBefore{}),
		meta.BeforeHandle(&firstBefore{}),
		meta.Endpoint(&beforeProvideEndpoint{}),
	)

	req := httptest.NewRequest(http.MethodGet, "/api/before", nil)
	res := httptest.NewRecorder()
	app.mux.ServeHTTP(res, req)

	assertErrorResponse(t, res, http.StatusForbidden, "blocked", "Forbidden", "blocked before handle")
	if len(recorder.Calls) != 0 {
		t.Fatalf("calls = %#v, want no later BeforeHandle or Handle calls", recorder.Calls)
	}
}

type bodyBefore struct {
	Request struct {
		Body string
	}
}

func (h *bodyBefore) BeforeHandle(context.Context) error {
	return nil
}

func TestAppBeforeHandle_RequestBodyPanicsAtRegistration(t *testing.T) {
	t.Parallel()

	defer func() {
		msg, ok := panicMessage(recover())
		if !ok {
			t.Fatal("expected panic, got nil")
		}
		if !strings.Contains(msg, "Request.Body is not available in BeforeHandle") {
			t.Fatalf("panic = %q, want body rejection", msg)
		}
	}()

	newGroupedTestApp(
		"/api",
		meta.BeforeHandle(&bodyBefore{}),
		meta.Endpoint(&beforeProvideEndpoint{}),
	)
}

type shadowHeaderEndpoint struct {
	meta.GET `path:"/shadow"`
	Request  struct {
		Authorization int `header:"Authorization"`
	}
	Response struct {
		Value int `json:"value"`
	}
}

func (h *shadowHeaderEndpoint) Handle(context.Context) error {
	h.Response.Value = h.Request.Authorization
	return nil
}

func TestAppHeaderPolicy_ChildShadowAndSameScopeDuplicate(t *testing.T) {
	t.Parallel()

	parentAuth := meta.NewHeader[string]("Authorization")
	childAuth := meta.NewHeader[int]("Authorization")
	app := newGroupedTestApp(
		"/api",
		meta.RequireHeader(parentAuth),
		meta.Group(
			"/child",
			meta.RequireHeader(childAuth),
			meta.Endpoint(&shadowHeaderEndpoint{}),
		),
	)

	req := httptest.NewRequest(http.MethodGet, "/api/child/shadow", nil)
	req.Header.Set("Authorization", "42")
	res := httptest.NewRecorder()
	app.mux.ServeHTTP(res, req)

	got := assertSuccessResponse(t, res, http.StatusOK, "ok", "OK", "success")
	data := resSuccessData(t, got)
	if data["value"] != float64(42) {
		t.Fatalf("response data = %+v, want child shadowed int header", data)
	}

	defer func() {
		msg, ok := panicMessage(recover())
		if !ok {
			t.Fatal("expected duplicate header panic, got nil")
		}
		if !strings.Contains(msg, "duplicate required header") {
			t.Fatalf("panic = %q, want duplicate required header", msg)
		}
	}()
	newGroupedTestApp(
		"/api",
		meta.RequireHeader(parentAuth),
		meta.RequireHeader(meta.NewHeader[string]("authorization")),
		meta.Endpoint(&GroupedSpecTestEndpoint{}),
	)
}
