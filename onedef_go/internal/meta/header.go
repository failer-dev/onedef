package meta

import (
	"encoding"
	"fmt"
	"reflect"
	"strings"
	"unicode"

	"github.com/failer-dev/wherr"
)

type HeaderSymbol interface {
	HeaderContract() HeaderContract
}

type HeaderContract struct {
	Name        string
	WireName    string
	Type        reflect.Type
	Description string
	Examples    []string
	Parse       HeaderParser
}

type HeaderParser func(string) (reflect.Value, error)

type HeaderOption interface {
	applyHeaderOption(*HeaderContract)
}

type headerOptionFunc func(*HeaderContract)

func (f headerOptionFunc) applyHeaderOption(contract *HeaderContract) {
	f(contract)
}

type headerSymbol struct {
	contract HeaderContract
}

func (h headerSymbol) HeaderContract() HeaderContract {
	return h.contract
}

func NewHeader[T any](wireName string, opts ...HeaderOption) HeaderSymbol {
	wireName = strings.TrimSpace(wireName)
	if wireName == "" {
		panic("onedef: header wire name must not be empty")
	}

	t := reflect.TypeFor[T]()
	contract := HeaderContract{
		Name:     headerSymbolName(wireName),
		WireName: wireName,
		Type:     t,
	}
	for _, opt := range opts {
		if opt != nil {
			opt.applyHeaderOption(&contract)
		}
	}
	if contract.Name == "" {
		panic("onedef: header name must not be empty")
	}
	if contract.Parse == nil {
		contract.Parse = defaultHeaderParser(t)
	}
	return headerSymbol{contract: contract}
}

func HeaderName(name string) HeaderOption {
	return headerOptionFunc(func(contract *HeaderContract) {
		contract.Name = strings.TrimSpace(name)
	})
}

func HeaderDescription(description string) HeaderOption {
	return headerOptionFunc(func(contract *HeaderContract) {
		contract.Description = description
	})
}

func HeaderExample(example string) HeaderOption {
	return headerOptionFunc(func(contract *HeaderContract) {
		contract.Examples = append(contract.Examples, example)
	})
}

func HeaderParse[T any](parse func(string) (T, error)) HeaderOption {
	if parse == nil {
		panic("onedef: header parser must not be nil")
	}
	expected := reflect.TypeFor[T]()
	return headerOptionFunc(func(contract *HeaderContract) {
		if contract.Type != expected {
			panic(fmt.Sprintf("onedef: header parser type %s does not match header type %s", expected, contract.Type))
		}
		contract.Parse = func(raw string) (reflect.Value, error) {
			value, err := parse(raw)
			if err != nil {
				return reflect.Value{}, err
			}
			return reflect.ValueOf(value), nil
		}
	})
}

func MustHeaderContract(header HeaderSymbol) HeaderContract {
	if header == nil {
		panic("onedef: header must not be nil")
	}
	contract := header.HeaderContract()
	if strings.TrimSpace(contract.WireName) == "" {
		panic("onedef: header wire name must not be empty")
	}
	if strings.TrimSpace(contract.Name) == "" {
		panic("onedef: header name must not be empty")
	}
	if contract.Type == nil {
		panic("onedef: header type must not be nil")
	}
	if contract.Parse == nil {
		contract.Parse = defaultHeaderParser(contract.Type)
	}
	return contract
}

func headerSymbolName(wireName string) string {
	parts := strings.FieldsFunc(wireName, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
	var b strings.Builder
	for _, part := range parts {
		for i, r := range part {
			if i == 0 {
				b.WriteRune(unicode.ToUpper(r))
				continue
			}
			b.WriteRune(r)
		}
	}
	if b.Len() == 0 {
		return "Header"
	}
	return b.String()
}

func defaultHeaderParser(t reflect.Type) HeaderParser {
	return func(raw string) (reflect.Value, error) {
		return parseTextValue(raw, t)
	}
}

var textUnmarshalerType = reflect.TypeOf((*encoding.TextUnmarshaler)(nil)).Elem()

func parseTextValue(raw string, target reflect.Type) (reflect.Value, error) {
	if target.Kind() == reflect.Pointer {
		inner, err := parseTextValue(raw, target.Elem())
		if err != nil {
			return reflect.Value{}, err
		}
		ptr := reflect.New(target.Elem())
		ptr.Elem().Set(inner)
		return ptr, nil
	}

	ptr := reflect.New(target)
	if ptr.Type().Implements(textUnmarshalerType) {
		if err := ptr.Interface().(encoding.TextUnmarshaler).UnmarshalText([]byte(raw)); err != nil {
			return reflect.Value{}, err
		}
		return ptr.Elem(), nil
	}

	return ConvertStringValue(raw, target)
}

func AssignHeaderValue(value reflect.Value, target reflect.Type) (reflect.Value, error) {
	if value.Type().AssignableTo(target) {
		return value, nil
	}
	if target.Kind() == reflect.Pointer && value.Type().AssignableTo(target.Elem()) {
		ptr := reflect.New(target.Elem())
		ptr.Elem().Set(value)
		return ptr, nil
	}
	if value.Type().ConvertibleTo(target) {
		return value.Convert(target), nil
	}
	return reflect.Value{}, wherr.Errorf("cannot assign header value of type %s to %s", value.Type(), target)
}
