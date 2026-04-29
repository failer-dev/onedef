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
}

func defaultRunConfig() runConfig {
	return runConfig{
		readHeaderTimeout: 5 * time.Second,
		readTimeout:       15 * time.Second,
		writeTimeout:      30 * time.Second,
		idleTimeout:       60 * time.Second,
		maxHeaderBytes:    1 << 20,
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

func (a *App) newHTTPServer(addr string, opts ...RunOption) *http.Server {
	cfg := defaultRunConfig()
	for _, opt := range opts {
		if opt != nil {
			opt(&cfg)
		}
	}

	return &http.Server{
		Addr:              addr,
		Handler:           a.mux,
		ReadHeaderTimeout: cfg.readHeaderTimeout,
		ReadTimeout:       cfg.readTimeout,
		WriteTimeout:      cfg.writeTimeout,
		IdleTimeout:       cfg.idleTimeout,
		MaxHeaderBytes:    cfg.maxHeaderBytes,
	}
}
