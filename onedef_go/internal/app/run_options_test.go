package app

import (
	"strings"
	"testing"
	"time"
)

func TestNewHTTPServer_UsesSecureDefaults(t *testing.T) {
	t.Parallel()

	app := New()

	server := app.newHTTPServer(":0")

	if server.ReadHeaderTimeout != 5*time.Second {
		t.Fatalf("ReadHeaderTimeout = %v, want %v", server.ReadHeaderTimeout, 5*time.Second)
	}
	if server.ReadTimeout != 15*time.Second {
		t.Fatalf("ReadTimeout = %v, want %v", server.ReadTimeout, 15*time.Second)
	}
	if server.WriteTimeout != 30*time.Second {
		t.Fatalf("WriteTimeout = %v, want %v", server.WriteTimeout, 30*time.Second)
	}
	if server.IdleTimeout != 60*time.Second {
		t.Fatalf("IdleTimeout = %v, want %v", server.IdleTimeout, 60*time.Second)
	}
	if server.MaxHeaderBytes != 1<<20 {
		t.Fatalf("MaxHeaderBytes = %d, want %d", server.MaxHeaderBytes, 1<<20)
	}
	if got := maxBodyBytesFromHandler(t, server.Handler); got != defaultMaxBodyBytes {
		t.Fatalf("maxBodyBytes = %d, want %d", got, defaultMaxBodyBytes)
	}
}

func TestNewHTTPServer_OverridesSecureDefaults(t *testing.T) {
	t.Parallel()

	app := New()

	server := app.newHTTPServer(":0",
		WithReadHeaderTimeout(2*time.Second),
		WithReadTimeout(7*time.Second),
		WithWriteTimeout(11*time.Second),
		WithIdleTimeout(13*time.Second),
		WithMaxHeaderBytes(2048),
		WithMaxBodyBytes(4096),
	)

	if server.ReadHeaderTimeout != 2*time.Second {
		t.Fatalf("ReadHeaderTimeout = %v, want %v", server.ReadHeaderTimeout, 2*time.Second)
	}
	if server.ReadTimeout != 7*time.Second {
		t.Fatalf("ReadTimeout = %v, want %v", server.ReadTimeout, 7*time.Second)
	}
	if server.WriteTimeout != 11*time.Second {
		t.Fatalf("WriteTimeout = %v, want %v", server.WriteTimeout, 11*time.Second)
	}
	if server.IdleTimeout != 13*time.Second {
		t.Fatalf("IdleTimeout = %v, want %v", server.IdleTimeout, 13*time.Second)
	}
	if server.MaxHeaderBytes != 2048 {
		t.Fatalf("MaxHeaderBytes = %d, want %d", server.MaxHeaderBytes, 2048)
	}
	if got := maxBodyBytesFromHandler(t, server.Handler); got != 4096 {
		t.Fatalf("maxBodyBytes = %d, want 4096", got)
	}
}

func TestNewHTTPServer_ZeroValuesDisableLimits(t *testing.T) {
	t.Parallel()

	app := New()

	server := app.newHTTPServer(":0",
		WithReadHeaderTimeout(0),
		WithReadTimeout(0),
		WithWriteTimeout(0),
		WithIdleTimeout(0),
		WithMaxHeaderBytes(0),
		WithMaxBodyBytes(0),
	)

	if server.ReadHeaderTimeout != 0 {
		t.Fatalf("ReadHeaderTimeout = %v, want 0", server.ReadHeaderTimeout)
	}
	if server.ReadTimeout != 0 {
		t.Fatalf("ReadTimeout = %v, want 0", server.ReadTimeout)
	}
	if server.WriteTimeout != 0 {
		t.Fatalf("WriteTimeout = %v, want 0", server.WriteTimeout)
	}
	if server.IdleTimeout != 0 {
		t.Fatalf("IdleTimeout = %v, want 0", server.IdleTimeout)
	}
	if server.MaxHeaderBytes != 0 {
		t.Fatalf("MaxHeaderBytes = %d, want 0", server.MaxHeaderBytes)
	}
	if server.Handler != app.mux {
		t.Fatalf("Handler = %#v, want mux without body limit wrapper", server.Handler)
	}
}

func TestRunOptions_PanicOnNegativeValues(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name string
		run  func()
		want string
	}{
		{
			name: "read header timeout",
			run:  func() { WithReadHeaderTimeout(-time.Second) },
			want: "read header timeout cannot be negative",
		},
		{
			name: "read timeout",
			run:  func() { WithReadTimeout(-time.Second) },
			want: "read timeout cannot be negative",
		},
		{
			name: "write timeout",
			run:  func() { WithWriteTimeout(-time.Second) },
			want: "write timeout cannot be negative",
		},
		{
			name: "idle timeout",
			run:  func() { WithIdleTimeout(-time.Second) },
			want: "idle timeout cannot be negative",
		},
		{
			name: "max header bytes",
			run:  func() { WithMaxHeaderBytes(-1) },
			want: "max header bytes cannot be negative",
		},
		{
			name: "max body bytes",
			run:  func() { WithMaxBodyBytes(-1) },
			want: "max body bytes cannot be negative",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			defer func() {
				msg, ok := panicMessage(recover())
				if !ok {
					t.Fatal("expected panic, got nil")
				}
				if !strings.Contains(msg, tc.want) {
					t.Fatalf("panic = %q, want substring %q", msg, tc.want)
				}
			}()

			tc.run()
		})
	}
}

func maxBodyBytesFromHandler(t *testing.T, handler any) int64 {
	t.Helper()

	wrapped, ok := handler.(*maxBodyBytesHandler)
	if !ok {
		t.Fatalf("Handler = %#v, want *maxBodyBytesHandler", handler)
	}
	return wrapped.maxBodyBytes
}
