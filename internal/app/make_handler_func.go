package app

import (
	"encoding/json"
	"net/http"
	"reflect"

	"github.com/failer-dev/onedef/internal/meta"
)

func MakeHandlerFunc(es meta.EndpointStruct) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		endpointInstance := reflect.New(es.StructType)

		// Request 설정
		requestInstance := endpointInstance.Elem().FieldByName("Request")

		// 1. Body 파싱 (POST/PUT/PATCH)
		if r.Body != nil && es.Method != meta.EndpointMethodGet && es.Method != meta.EndpointMethodDelete {
			if err := json.NewDecoder(r.Body).Decode(requestInstance.Addr().Interface()); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
		}

		// 2. Path params 세팅 (body 값을 덮어씀)
		for _, p := range es.Request.PathParameterFields {
			raw := r.PathValue(p.VariableName)
			val, err := convertPathValue(raw, p.FieldType)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			requestInstance.Field(p.FieldIndex).Set(val)
		}

		// 3. Query params 세팅
		for _, q := range es.Request.QueryParameterFields {
			raw := r.URL.Query().Get(q.QueryKey)
			if raw == "" {
				continue
			}
			val, err := convertPathValue(raw, q.FieldType)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			requestInstance.Field(q.FieldIndex).Set(val)
		}

		handler := endpointInstance.Interface().(meta.Handler)
		if err := handler.Handle(r.Context()); err != nil {
			// handle error
		}

		response := endpointInstance.Elem().FieldByName("Response").Interface()
		json.NewEncoder(w).Encode(response)
	}
}
