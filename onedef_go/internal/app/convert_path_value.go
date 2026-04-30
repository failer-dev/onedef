package app

import (
	"reflect"
	"strconv"

	"github.com/failer-dev/wherr"
	"github.com/google/uuid"
)

var uuidType = reflect.TypeOf(uuid.UUID{})

// convertPathValue는 path parameter 문자열을 대상 필드의 reflect.Type에 맞게 변환한다.
func convertPathValue(raw string, target reflect.Type) (reflect.Value, error) {
	if target.Kind() == reflect.Pointer {
		inner, err := convertPathValue(raw, target.Elem())
		if err != nil {
			return reflect.Value{}, err
		}
		ptr := reflect.New(target.Elem())
		ptr.Elem().Set(inner)
		return ptr, nil
	}

	// uuid.UUID는 Kind가 [16]byte라서 별도 처리
	if target == uuidType {
		v, err := uuid.Parse(raw)
		if err != nil {
			return reflect.Value{}, wherr.Errorf("cannot convert path value %q to uuid.UUID: %w", raw, err)
		}
		return reflect.ValueOf(v), nil
	}

	switch target.Kind() {
	case reflect.String:
		return reflect.ValueOf(raw), nil

	case reflect.Int:
		v, err := strconv.Atoi(raw)
		if err != nil {
			return reflect.Value{}, wherr.Errorf("cannot convert path value %q to int: %w", raw, err)
		}
		return reflect.ValueOf(v), nil

	case reflect.Int8:
		v, err := strconv.ParseInt(raw, 10, 8)
		if err != nil {
			return reflect.Value{}, wherr.Errorf("cannot convert path value %q to int8: %w", raw, err)
		}
		return reflect.ValueOf(int8(v)), nil

	case reflect.Int16:
		v, err := strconv.ParseInt(raw, 10, 16)
		if err != nil {
			return reflect.Value{}, wherr.Errorf("cannot convert path value %q to int16: %w", raw, err)
		}
		return reflect.ValueOf(int16(v)), nil

	case reflect.Int32:
		v, err := strconv.ParseInt(raw, 10, 32)
		if err != nil {
			return reflect.Value{}, wherr.Errorf("cannot convert path value %q to int32: %w", raw, err)
		}
		return reflect.ValueOf(int32(v)), nil

	case reflect.Int64:
		v, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			return reflect.Value{}, wherr.Errorf("cannot convert path value %q to int64: %w", raw, err)
		}
		return reflect.ValueOf(v), nil

	case reflect.Uint:
		v, err := strconv.ParseUint(raw, 10, 0)
		if err != nil {
			return reflect.Value{}, wherr.Errorf("cannot convert path value %q to uint: %w", raw, err)
		}
		return reflect.ValueOf(uint(v)), nil

	case reflect.Uint8:
		v, err := strconv.ParseUint(raw, 10, 8)
		if err != nil {
			return reflect.Value{}, wherr.Errorf("cannot convert path value %q to uint8: %w", raw, err)
		}
		return reflect.ValueOf(uint8(v)), nil

	case reflect.Uint16:
		v, err := strconv.ParseUint(raw, 10, 16)
		if err != nil {
			return reflect.Value{}, wherr.Errorf("cannot convert path value %q to uint16: %w", raw, err)
		}
		return reflect.ValueOf(uint16(v)), nil

	case reflect.Uint32:
		v, err := strconv.ParseUint(raw, 10, 32)
		if err != nil {
			return reflect.Value{}, wherr.Errorf("cannot convert path value %q to uint32: %w", raw, err)
		}
		return reflect.ValueOf(uint32(v)), nil

	case reflect.Uint64:
		v, err := strconv.ParseUint(raw, 10, 64)
		if err != nil {
			return reflect.Value{}, wherr.Errorf("cannot convert path value %q to uint64: %w", raw, err)
		}
		return reflect.ValueOf(v), nil

	case reflect.Bool:
		v, err := strconv.ParseBool(raw)
		if err != nil {
			return reflect.Value{}, wherr.Errorf("cannot convert path value %q to bool: %w", raw, err)
		}
		return reflect.ValueOf(v), nil

	default:
		return reflect.Value{}, wherr.Errorf("unsupported path parameter type: %s", target.String())
	}
}
