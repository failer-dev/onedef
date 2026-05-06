package app

import (
	"net/http"
	"time"
)

type RunOption func(*runConfig)

type runConfig struct {
	readHeaderTimeout time.Duration
	readTimeout       time.Duration
	writeTimeout      time.Duration
	idleTimeout       time.Duration
	maxHeaderBytes    int
	maxBodyBytes      int64
}

const defaultMaxBodyBytes int64 = 10 << 20

func defaultRunConfig() runConfig {
	return runConfig{
		readHeaderTimeout: 5 * time.Second,
		readTimeout:       15 * time.Second,
		writeTimeout:      30 * time.Second,
		idleTimeout:       60 * time.Second,
		maxHeaderBytes:    1 << 20,
		maxBodyBytes:      defaultMaxBodyBytes,
	}
}

func WithReadHeaderTimeout(d time.Duration) RunOption {
	if d < 0 {
		panic("onedef: read header timeout cannot be negative")
	}
	return func(cfg *runConfig) {
		cfg.readHeaderTimeout = d
	}
}

func WithReadTimeout(d time.Duration) RunOption {
	if d < 0 {
		panic("onedef: read timeout cannot be negative")
	}
	return func(cfg *runConfig) {
		cfg.readTimeout = d
	}
}

func WithWriteTimeout(d time.Duration) RunOption {
	if d < 0 {
		panic("onedef: write timeout cannot be negative")
	}
	return func(cfg *runConfig) {
		cfg.writeTimeout = d
	}
}

func WithIdleTimeout(d time.Duration) RunOption {
	if d < 0 {
		panic("onedef: idle timeout cannot be negative")
	}
	return func(cfg *runConfig) {
		cfg.idleTimeout = d
	}
}

func WithMaxHeaderBytes(n int) RunOption {
	if n < 0 {
		panic("onedef: max header bytes cannot be negative")
	}
	return func(cfg *runConfig) {
		cfg.maxHeaderBytes = n
	}
}

func WithMaxBodyBytes(n int64) RunOption {
	if n < 0 {
		panic("onedef: max body bytes cannot be negative")
	}
	return func(cfg *runConfig) {
		cfg.maxBodyBytes = n
	}
}

func (a *App) newHTTPServer(addr string, opts ...RunOption) *http.Server {
	cfg := defaultRunConfig()
	for _, opt := range opts {
		if opt != nil {
			opt(&cfg)
		}
	}

	handler := http.Handler(a.mux)
	if cfg.maxBodyBytes > 0 {
		handler = &maxBodyBytesHandler{
			next:         handler,
			maxBodyBytes: cfg.maxBodyBytes,
		}
	}

	return &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: cfg.readHeaderTimeout,
		ReadTimeout:       cfg.readTimeout,
		WriteTimeout:      cfg.writeTimeout,
		IdleTimeout:       cfg.idleTimeout,
		MaxHeaderBytes:    cfg.maxHeaderBytes,
	}
}

type maxBodyBytesHandler struct {
	next         http.Handler
	maxBodyBytes int64
}

func (h *maxBodyBytesHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, h.maxBodyBytes)
	h.next.ServeHTTP(w, r)
}
