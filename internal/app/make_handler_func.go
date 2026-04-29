package app

import (
	"encoding/json"
	"net/http"
	"reflect"

	"github.com/failer-dev/onedef/internal/meta"
)

func MakeHandlerFunc(es meta.EndpointStruct, deps []resolvedDependency) meta.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		endpointInstance := reflect.New(es.StructType)

		// Request 설정
		requestInstance := endpointInstance.Elem().FieldByName("Request")

		// 1. Body 파싱 (POST/PUT/PATCH)
		if r.Body != nil && es.Method != meta.EndpointMethodGet && es.Method != meta.EndpointMethodDelete {
			if err := decodeJSON(r, requestInstance.Addr().Interface()); err != nil {
				return meta.BadRequest(
					"invalid_body",
					"request body is invalid JSON",
					map[string]any{"error": err.Error()},
				)
			}
		}

		// 2. Path params 세팅 (body 값을 덮어씀)
		for _, p := range es.Request.PathParameterFields {
			raw := r.PathValue(p.VariableName)
			val, err := convertPathValue(raw, p.FieldType)
			if err != nil {
				return meta.BadRequest(
					"invalid_path_parameter",
					err.Error(),
					map[string]any{"parameter": p.VariableName},
				)
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
				return meta.BadRequest(
					"invalid_query_parameter",
					err.Error(),
					map[string]any{"parameter": q.QueryKey},
				)
			}
			requestInstance.Field(q.FieldIndex).Set(val)
		}

		// 4. Header params 세팅
		for _, h := range es.Request.HeaderParameterFields {
			raw := r.Header.Get(h.HeaderName)
			if raw == "" {
				if !h.Required {
					continue
				}
				return meta.BadRequest(
					"missing_header_parameter",
					"required request header is missing",
					map[string]any{"parameter": h.HeaderName},
				)
			}
			val, err := convertPathValue(raw, h.FieldType)
			if err != nil {
				return meta.BadRequest(
					"invalid_header_parameter",
					err.Error(),
					map[string]any{"parameter": h.HeaderName},
				)
			}
			requestInstance.Field(h.FieldIndex).Set(val)
		}

		if es.Dependencies.Exists {
			depsField := endpointInstance.Elem().Field(es.Dependencies.StructIndex)
			for _, dep := range deps {
				depsField.Field(dep.FieldIndex).Set(dep.Value)
			}
		}

		handler := endpointInstance.Interface().(meta.Handler)
		if err := handler.Handle(r.Context()); err != nil {
			return err
		}

		response := endpointInstance.Elem().FieldByName("Response").Interface()
		writeHTTPSuccess(w, es.SuccessStatus, response)
		return nil
	}
}

func decodeJSON(r *http.Request, target any) error {
	if err := json.NewDecoder(r.Body).Decode(target); err != nil {
		return err
	}
	return nil
}
