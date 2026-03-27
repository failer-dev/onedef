package onedef

import (
	"github.com/failer-dev/onedef/internal/app"
	"github.com/failer-dev/onedef/internal/meta"
)

type GET = meta.GET
type POST = meta.POST
type PUT = meta.PUT
type PATCH = meta.PATCH
type DELETE = meta.DELETE
type HEAD = meta.HEAD
type OPTIONS = meta.OPTIONS

type App = app.App

func New() *App {
	return app.New()
}
