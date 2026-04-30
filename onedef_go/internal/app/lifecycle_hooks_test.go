package app

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/failer-dev/onedef/onedef_go/internal/meta"
)

type lifecycleRecorder struct {
	Calls    []string
	Outcomes []meta.Outcome
}

type lifecycleBefore struct {
	Provide struct {
		Recorder *lifecycleRecorder
		Value    string
	}
}

func (h *lifecycleBefore) BeforeHandle(context.Context) error {
	h.Provide.Recorder.Calls = append(h.Provide.Recorder.Calls, "before")
	h.Provide.Value = "from-before"
	return nil
}

type lifecycleEndpoint struct {
	meta.POST `path:"/{id}" status:"201"`
	Request   struct {
		ID             string
		IdempotencyKey string `header:"Idempotency-Key"`
		Name           string `json:"name"`
	}
	Provide struct {
		Recorder *lifecycleRecorder
		Value    string
	}
	Response struct {
		ID    string `json:"id"`
		Value string `json:"value"`
	}
}

func (h *lifecycleEndpoint) Handle(context.Context) error {
	h.Provide.Recorder.Calls = append(h.Provide.Recorder.Calls, "handle:"+h.Provide.Value)
	h.Response.ID = h.Request.ID
	h.Response.Value = h.Provide.Value
	return nil
}

type endpointLifecycleAfter struct {
	Request struct {
		ID             string
		IdempotencyKey string `header:"Idempotency-Key"`
	}
	Provide struct {
		Recorder *lifecycleRecorder
		Value    string
	}
	Response struct {
		ID    string `json:"id"`
		Value string `json:"value"`
	}
}

func (h *endpointLifecycleAfter) AfterHandle(context.Context) error {
	h.Provide.Recorder.Calls = append(h.Provide.Recorder.Calls, strings.Join([]string{
		"after-endpoint",
		h.Request.ID,
		h.Request.IdempotencyKey,
		h.Provide.Value,
		h.Response.ID,
		h.Response.Value,
	}, ":"))
	h.Response.Value = "mutated-by-after"
	return nil
}

type childLifecycleAfter struct {
	Provide struct {
		Recorder *lifecycleRecorder
		Value    string
	}
	Response struct {
		Value string `json:"value"`
	}
}

func (h *childLifecycleAfter) AfterHandle(context.Context) error {
	h.Provide.Recorder.Calls = append(h.Provide.Recorder.Calls, "after-child:"+h.Provide.Value+":"+h.Response.Value)
	return nil
}

type rootLifecycleAfter struct {
	Provide struct {
		Recorder *lifecycleRecorder
		Value    string
	}
	Response struct {
		Value string `json:"value"`
	}
}

func (h *rootLifecycleAfter) AfterHandle(context.Context) error {
	h.Provide.Recorder.Calls = append(h.Provide.Recorder.Calls, "after-root:"+h.Provide.Value+":"+h.Response.Value)
	return nil
}

type failingLifecycleAfter struct {
	Provide struct {
		Recorder *lifecycleRecorder
	}
}

func (h *failingLifecycleAfter) AfterHandle(context.Context) error {
	h.Provide.Recorder.Calls = append(h.Provide.Recorder.Calls, "after-fail")
	return meta.Conflict("after_failed", "after failed", nil)
}

type recordingObserver struct {
	Name     string
	Recorder *lifecycleRecorder
}

func (o *recordingObserver) Observe(_ context.Context, outcome meta.Outcome) {
	o.Recorder.Calls = append(o.Recorder.Calls, "observe:"+o.Name)
	o.Recorder.Outcomes = append(o.Recorder.Outcomes, outcome)
}

func TestAppLifecycle_AfterHandleSuccessReadOnlyAndObserverOrder(t *testing.T) {
	t.Parallel()

	recorder := &lifecycleRecorder{}
	app := newGroupedTestApp(
		"/api",
		meta.NewProvideBinding(recorder),
		meta.RequireHeader(idempotencyHeader),
		meta.BeforeHandle(&lifecycleBefore{}),
		meta.AfterHandle(&rootLifecycleAfter{}),
		meta.Observe(&recordingObserver{Name: "root", Recorder: recorder}),
		meta.Group(
			"/orders",
			meta.AfterHandle(&childLifecycleAfter{}),
			meta.Observe(&recordingObserver{Name: "child", Recorder: recorder}),
			meta.Endpoint(
				&lifecycleEndpoint{},
				meta.AfterHandle(&endpointLifecycleAfter{}),
				meta.Observe(&recordingObserver{Name: "endpoint", Recorder: recorder}),
			),
		),
	)

	req := httptest.NewRequest(http.MethodPost, "/api/orders/order-7", strings.NewReader(`{"name":"A"}`))
	req.Header.Set("Idempotency-Key", "req-1")
	res := httptest.NewRecorder()
	app.mux.ServeHTTP(res, req)

	got := assertSuccessResponse(t, res, http.StatusCreated, "created", "Created", "success")
	data := resSuccessData(t, got)
	if data["id"] != "order-7" {
		t.Fatalf("response data = %+v, want id order-7", data)
	}
	if data["value"] != "from-before" {
		t.Fatalf("response data = %+v, want AfterHandle mutation to be ignored", data)
	}

	wantCalls := strings.Join([]string{
		"before",
		"handle:from-before",
		"after-endpoint:order-7:req-1:from-before:order-7:from-before",
		"after-child:from-before:from-before",
		"after-root:from-before:from-before",
		"observe:root",
		"observe:child",
		"observe:endpoint",
	}, ",")
	if gotCalls := strings.Join(recorder.Calls, ","); gotCalls != wantCalls {
		t.Fatalf("calls = %q, want %q", gotCalls, wantCalls)
	}
	if len(recorder.Outcomes) != 3 {
		t.Fatalf("outcomes = %d, want 3", len(recorder.Outcomes))
	}
	for _, outcome := range recorder.Outcomes {
		if outcome.Status != http.StatusCreated {
			t.Fatalf("outcome status = %d, want %d", outcome.Status, http.StatusCreated)
		}
		if outcome.Error != nil {
			t.Fatalf("outcome error = %v, want nil", outcome.Error)
		}
		if outcome.Method != http.MethodPost || outcome.Path != "/api/orders/{id}" || outcome.Endpoint != "lifecycleEndpoint" {
			t.Fatalf("outcome = %#v, want endpoint identity", outcome)
		}
	}
}

func TestAppLifecycle_AfterHandleErrorUsesErrorPolicyAndSkipsRemainingAfterHandle(t *testing.T) {
	t.Parallel()

	recorder := &lifecycleRecorder{}
	app := newGroupedTestApp(
		"/api",
		meta.NewProvideBinding(recorder),
		meta.RequireHeader(idempotencyHeader),
		meta.BeforeHandle(&lifecycleBefore{}),
		meta.AfterHandle(&rootLifecycleAfter{}),
		meta.Observe(&recordingObserver{Name: "root", Recorder: recorder}),
		meta.Group(
			"/orders",
			meta.Endpoint(
				&lifecycleEndpoint{},
				meta.AfterHandle(&failingLifecycleAfter{}),
				meta.AfterHandle(&endpointLifecycleAfter{}),
			),
		),
	)

	req := httptest.NewRequest(http.MethodPost, "/api/orders/order-7", strings.NewReader(`{"name":"A"}`))
	req.Header.Set("Idempotency-Key", "req-1")
	res := httptest.NewRecorder()
	app.mux.ServeHTTP(res, req)

	assertErrorResponse(t, res, http.StatusConflict, "after_failed", "Conflict", "after failed")

	wantCalls := "before,handle:from-before,after-fail,observe:root"
	if gotCalls := strings.Join(recorder.Calls, ","); gotCalls != wantCalls {
		t.Fatalf("calls = %q, want %q", gotCalls, wantCalls)
	}
	if len(recorder.Outcomes) != 1 {
		t.Fatalf("outcomes = %d, want 1", len(recorder.Outcomes))
	}
	if recorder.Outcomes[0].Status != http.StatusConflict {
		t.Fatalf("outcome status = %d, want %d", recorder.Outcomes[0].Status, http.StatusConflict)
	}
	if recorder.Outcomes[0].Error == nil {
		t.Fatal("outcome error = nil, want AfterHandle error")
	}
}

func TestAppObserve_RunsForFailurePaths(t *testing.T) {
	tests := []struct {
		name       string
		buildApp   func(*lifecycleRecorder) *App
		request    func() *http.Request
		wantStatus int
	}{
		{
			name: "missing header",
			buildApp: func(recorder *lifecycleRecorder) *App {
				return newGroupedTestApp(
					"/api",
					meta.RequireHeader(idempotencyHeader),
					meta.Observe(&recordingObserver{Name: "root", Recorder: recorder}),
					meta.Endpoint(&SyntheticEndpointHeaderSpecTestEndpoint{}),
				)
			},
			request: func() *http.Request {
				return httptest.NewRequest(http.MethodGet, "/api/synthetic-header", nil)
			},
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "invalid body",
			buildApp: func(recorder *lifecycleRecorder) *App {
				return newGroupedTestApp(
					"/api",
					meta.Observe(&recordingObserver{Name: "root", Recorder: recorder}),
					meta.Endpoint(&createUserEndpoint{}),
				)
			},
			request: func() *http.Request {
				return httptest.NewRequest(http.MethodPost, "/api/users", strings.NewReader(`{"name":`))
			},
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "invalid path",
			buildApp: func(recorder *lifecycleRecorder) *App {
				return newGroupedTestApp(
					"/api",
					meta.Observe(&recordingObserver{Name: "root", Recorder: recorder}),
					meta.Endpoint(&getUserEndpoint{}),
				)
			},
			request: func() *http.Request {
				return httptest.NewRequest(http.MethodGet, "/api/users/not-int", nil)
			},
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "invalid query",
			buildApp: func(recorder *lifecycleRecorder) *App {
				return newGroupedTestApp(
					"/api",
					meta.Observe(&recordingObserver{Name: "root", Recorder: recorder}),
					meta.Endpoint(&listUsersEndpoint{}),
				)
			},
			request: func() *http.Request {
				return httptest.NewRequest(http.MethodGet, "/api/users?page=abc", nil)
			},
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "before handle",
			buildApp: func(recorder *lifecycleRecorder) *App {
				return newGroupedTestApp(
					"/api",
					meta.Observe(&recordingObserver{Name: "root", Recorder: recorder}),
					meta.BeforeHandle(&blockingBefore{}),
					meta.Endpoint(&beforeProvideEndpoint{}),
				)
			},
			request: func() *http.Request {
				return httptest.NewRequest(http.MethodGet, "/api/before", nil)
			},
			wantStatus: http.StatusForbidden,
		},
		{
			name: "handle",
			buildApp: func(recorder *lifecycleRecorder) *App {
				return newGroupedTestApp(
					"/api",
					meta.Observe(&recordingObserver{Name: "root", Recorder: recorder}),
					meta.Endpoint(&internalFailureEndpoint{}),
				)
			},
			request: func() *http.Request {
				return httptest.NewRequest(http.MethodGet, "/api/broken", nil)
			},
			wantStatus: http.StatusInternalServerError,
		},
		{
			name: "after handle",
			buildApp: func(recorder *lifecycleRecorder) *App {
				return newGroupedTestApp(
					"/api",
					meta.NewProvideBinding(recorder),
					meta.RequireHeader(idempotencyHeader),
					meta.BeforeHandle(&lifecycleBefore{}),
					meta.Observe(&recordingObserver{Name: "root", Recorder: recorder}),
					meta.Group(
						"/orders",
						meta.Endpoint(
							&lifecycleEndpoint{},
							meta.AfterHandle(&failingLifecycleAfter{}),
						),
					),
				)
			},
			request: func() *http.Request {
				req := httptest.NewRequest(http.MethodPost, "/api/orders/order-7", strings.NewReader(`{"name":"A"}`))
				req.Header.Set("Idempotency-Key", "req-1")
				return req
			},
			wantStatus: http.StatusConflict,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := &lifecycleRecorder{}
			app := tt.buildApp(recorder)
			res := httptest.NewRecorder()
			app.mux.ServeHTTP(res, tt.request())

			if res.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", res.Code, tt.wantStatus)
			}
			if len(recorder.Outcomes) != 1 {
				t.Fatalf("outcomes = %d, want 1; calls = %#v", len(recorder.Outcomes), recorder.Calls)
			}
			if recorder.Outcomes[0].Status != tt.wantStatus {
				t.Fatalf("outcome status = %d, want %d", recorder.Outcomes[0].Status, tt.wantStatus)
			}
			if recorder.Outcomes[0].Error == nil {
				t.Fatal("outcome error = nil, want failure")
			}
		})
	}
}

type afterHandleBodyRequest struct {
	Request struct {
		Body string
	}
}

func (h *afterHandleBodyRequest) AfterHandle(context.Context) error {
	return nil
}

type afterHandleUnknownResponse struct {
	Response struct {
		Missing string
	}
}

func (h *afterHandleUnknownResponse) AfterHandle(context.Context) error {
	return nil
}

func TestAppAfterHandle_RequestBodyPanicsAtRegistration(t *testing.T) {
	t.Parallel()

	defer func() {
		msg, ok := panicMessage(recover())
		if !ok {
			t.Fatal("expected panic, got nil")
		}
		if !strings.Contains(msg, "Request.Body is not available in AfterHandle") {
			t.Fatalf("panic = %q, want body rejection", msg)
		}
	}()

	newGroupedTestApp(
		"/api",
		meta.AfterHandle(&afterHandleBodyRequest{}),
		meta.Endpoint(&beforeProvideEndpoint{}),
	)
}

func TestAppAfterHandle_ResponseMustMatchEndpointResponse(t *testing.T) {
	t.Parallel()

	defer func() {
		msg, ok := panicMessage(recover())
		if !ok {
			t.Fatal("expected panic, got nil")
		}
		if !strings.Contains(msg, "does not exist on endpoint Response") {
			t.Fatalf("panic = %q, want response field rejection", msg)
		}
	}()

	newGroupedTestApp(
		"/api",
		meta.Endpoint(
			&lifecycleEndpoint{},
			meta.RequireHeader(idempotencyHeader),
			meta.AfterHandle(&afterHandleUnknownResponse{}),
		),
	)
}

func TestAppLifecycle_HooksStayOutOfIR(t *testing.T) {
	t.Parallel()

	recorder := &lifecycleRecorder{}
	app := newGroupedTestApp(
		"/api",
		meta.NewProvideBinding(recorder),
		meta.RequireHeader(idempotencyHeader),
		meta.BeforeHandle(&lifecycleBefore{}),
		meta.AfterHandle(&rootLifecycleAfter{}),
		meta.Observe(&recordingObserver{Name: "root", Recorder: recorder}),
		meta.Group(
			"/orders",
			meta.Endpoint(
				&lifecycleEndpoint{},
				meta.AfterHandle(&endpointLifecycleAfter{}),
				meta.Observe(&recordingObserver{Name: "endpoint", Recorder: recorder}),
			),
		),
	)

	specJSON, err := app.buildIRJSON(nil)
	if err != nil {
		t.Fatalf("buildIRJSON() error = %v", err)
	}

	spec := string(specJSON)
	for _, forbidden := range []string{"BeforeHandle", "AfterHandle", "Observe", "rootLifecycleAfter", "endpointLifecycleAfter", "recordingObserver"} {
		if strings.Contains(spec, forbidden) {
			t.Fatalf("spec = %s, should not contain hook implementation %q", spec, forbidden)
		}
	}
}
