package app

import (
	"fmt"
	"net/http"
	"reflect"

	"github.com/failer-dev/onedef/internal/inspect"
	"github.com/failer-dev/onedef/internal/meta"
	"github.com/failer-dev/onedef/internal/sdk/dart"
)

type App struct {
	mux       *http.ServeMux
	endpoints []meta.EndpointStruct
}

func New() *App {
	return &App{mux: http.NewServeMux()}
}

func (a *App) Register(endpoints ...any) {
	for _, ep := range endpoints {
		structType := reflect.TypeOf(ep)
		if structType.Kind() == reflect.Pointer {
			structType = structType.Elem()
		}

		method, path, pathParams, err := inspect.InspectEndpointMethodMarker(structType)
		if err != nil {
			panic(err)
		}

		request, err := inspect.InspectRequest(structType, method, pathParams)
		if err != nil {
			panic(err)
		}

		handlerType := reflect.TypeFor[meta.Handler]()
		ptrType := reflect.PointerTo(structType)
		if !ptrType.Implements(handlerType) {
			panic("onedef: " + structType.Name() + " must implement Handle(context.Context) error")
		}

		es := meta.EndpointStruct{
			StructName: structType.Name(),
			Method:     method,
			Path:       path,
			Request:    request,
			StructType: structType,
		}

		a.endpoints = append(a.endpoints, es)

		handler := MakeHandlerFunc(es)
		a.mux.HandleFunc(string(method)+" "+path, handler)
	}
}

func (a *App) Run(addr string) error {
	a.mux.HandleFunc("GET /onedef/sdk/dart", a.handleDartSDK)
	a.printEndpoints(addr)
	return http.ListenAndServe(addr, a.mux)
}

func (a *App) handleDartSDK(w http.ResponseWriter, r *http.Request) {
	name := r.URL.Query().Get("name")
	if name == "" {
		name = "onedef_sdk"
	}

	zipBytes, err := dart.Generate(a.endpoints, name)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.zip"`, name))
	w.Write(zipBytes)
}

func (a *App) printEndpoints(addr string) {
	fmt.Println("onedef server starting on", addr)
	fmt.Println()
	for _, es := range a.endpoints {
		fmt.Printf("  %-8s %s  (%s)\n", es.Method, es.Path, es.StructName)
	}
	fmt.Println()
	fmt.Println("  SDK")
	fmt.Printf("  %-8s %s\n", "GET", "/onedef/sdk/dart")
	fmt.Println()
}
