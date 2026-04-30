package app

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"reflect"

	"github.com/failer-dev/onedef/onedef_go/internal/meta"
)

func MakeHandlerFunc(es meta.EndpointStruct, provides provideRegistry) meta.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		parsedHeaders, err := parseRequiredHeaders(r, es.FinalRequiredHeaders)
		if err != nil {
			return err
		}

		scope := newProvideScope()
		for _, before := range es.BeforeHandlers {
			beforeInstance := reflect.New(before.StructType)
			if err := populateRequestFields(beforeInstance.Elem().FieldByName("Request"), before.Request, r, parsedHeaders, false); err != nil {
				return err
			}
			if err := fillProvideFieldSet(beforeInstance.Elem(), before.Provide, provides, scope, false, before.StructName); err != nil {
				return err
			}

			handler := beforeInstance.Interface().(meta.BeforeHandler)
			if err := handler.BeforeHandle(r.Context()); err != nil {
				return err
			}
			mergeProvideFieldSet(beforeInstance.Elem(), before.Provide, scope)
		}

		endpointInstance := reflect.New(es.StructType)
		requestInstance := endpointInstance.Elem().FieldByName("Request")

		if r.Body != nil && es.Method != meta.EndpointMethodGet && es.Method != meta.EndpointMethodDelete {
			if err := decodeJSON(r, requestInstance.Addr().Interface()); err != nil {
				var maxBytesErr *http.MaxBytesError
				if errors.As(err, &maxBytesErr) {
					return meta.NewHTTPError(
						http.StatusRequestEntityTooLarge,
						"request_body_too_large",
						http.StatusText(http.StatusRequestEntityTooLarge),
						"request body is too large",
						nil,
					)
				}
				return meta.BadRequest(
					"invalid_body",
					"request body is invalid JSON",
					nil,
				)
			}
		}

		if err := populateRequestFields(requestInstance, es.Request, r, parsedHeaders, true); err != nil {
			return err
		}

		if err := fillProvideFieldSet(endpointInstance.Elem(), es.Provide, provides, scope, true, es.StructName); err != nil {
			return err
		}

		handler := endpointInstance.Interface().(meta.Handler)
		if err := handler.Handle(r.Context()); err != nil {
			return err
		}

		responseField := endpointInstance.Elem().FieldByName("Response")
		for _, after := range es.AfterHandlers {
			afterInstance := reflect.New(after.StructType)
			if err := populateRequestFields(afterInstance.Elem().FieldByName("Request"), after.Request, r, parsedHeaders, false); err != nil {
				return err
			}
			if err := fillProvideFieldSet(afterInstance.Elem(), after.Provide, provides, scope, true, after.StructName); err != nil {
				return err
			}
			copyResponseFieldSet(afterInstance.Elem(), after.Response, responseField)

			handler := afterInstance.Interface().(meta.AfterHandler)
			if err := handler.AfterHandle(r.Context()); err != nil {
				return err
			}
		}

		response := responseField.Interface()
		writeHTTPSuccess(w, es.SuccessStatus, response)
		return nil
	}
}

func parseRequiredHeaders(r *http.Request, headers []meta.HeaderContract) (map[string]reflect.Value, error) {
	parsed := make(map[string]reflect.Value, len(headers))
	for _, header := range headers {
		raw := r.Header.Get(header.WireName)
		if raw == "" {
			return nil, meta.BadRequest(
				"missing_header_parameter",
				"required request header is missing",
				map[string]any{"parameter": header.WireName},
			)
		}
		value, err := header.Parse(raw)
		if err != nil {
			return nil, meta.BadRequest(
				"invalid_header_parameter",
				err.Error(),
				map[string]any{"parameter": header.WireName},
			)
		}
		parsed[normalizeHeaderName(header.WireName)] = value
	}
	return parsed, nil
}

func populateRequestFields(
	requestInstance reflect.Value,
	request meta.RequestField,
	r *http.Request,
	parsedHeaders map[string]reflect.Value,
	includeQuery bool,
) error {
	if !requestInstance.IsValid() {
		return nil
	}

	for _, p := range request.PathParameterFields {
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

	if includeQuery {
		for _, q := range request.QueryParameterFields {
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
	}

	for _, h := range request.HeaderParameterFields {
		if h.FieldIndex < 0 {
			continue
		}
		parsed, ok := parsedHeaders[normalizeHeaderName(h.Header.WireName)]
		if !ok {
			return meta.BadRequest(
				"missing_header_parameter",
				"required request header is missing",
				map[string]any{"parameter": h.Header.WireName},
			)
		}
		val, err := meta.AssignHeaderValue(parsed, h.FieldType)
		if err != nil {
			return meta.BadRequest(
				"invalid_header_parameter",
				err.Error(),
				map[string]any{"parameter": h.Header.WireName},
			)
		}
		requestInstance.Field(h.FieldIndex).Set(val)
	}

	return nil
}

func decodeJSON(r *http.Request, target any) error {
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return errors.New("request body must contain a single JSON value")
		}
		return err
	}
	return nil
}

func copyResponseFieldSet(target reflect.Value, fields meta.ResponseFieldSet, sourceResponse reflect.Value) {
	if !fields.Exists {
		return
	}

	targetResponse := target.Field(fields.StructIndex)
	for _, field := range fields.Fields {
		value := sourceResponse.Field(field.SourceFieldIndex)
		if !value.Type().AssignableTo(field.FieldType) && value.Type().ConvertibleTo(field.FieldType) {
			value = value.Convert(field.FieldType)
		}
		targetResponse.Field(field.FieldIndex).Set(value)
	}
}
