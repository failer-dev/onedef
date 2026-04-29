package onedef

import (
	"time"

	"github.com/failer-dev/onedef/internal/app"
)

type RunOption = app.RunOption

func WithReadHeaderTimeout(d time.Duration) RunOption {
	return app.WithReadHeaderTimeout(d)
}

func WithReadTimeout(d time.Duration) RunOption {
	return app.WithReadTimeout(d)
}

func WithWriteTimeout(d time.Duration) RunOption {
	return app.WithWriteTimeout(d)
}

func WithIdleTimeout(d time.Duration) RunOption {
	return app.WithIdleTimeout(d)
}

func WithMaxHeaderBytes(n int) RunOption {
	return app.WithMaxHeaderBytes(n)
}
