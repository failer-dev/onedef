package meta

import (
	"reflect"
	"strconv"

	"github.com/failer-dev/wherr"
	"github.com/google/uuid"
)

var UUIDType = reflect.TypeOf(uuid.UUID{})

func ConvertStringValue(raw string, target reflect.Type) (reflect.Value, error) {
	if target == UUIDType {
		v, err := uuid.Parse(raw)
		if err != nil {
			return reflect.Value{}, wherr.Errorf("cannot convert value %q to uuid.UUID: %w", raw, err)
		}
		return reflect.ValueOf(v), nil
	}

	switch target.Kind() {
	case reflect.String:
		return reflect.ValueOf(raw).Convert(target), nil
	case reflect.Int:
		v, err := strconv.Atoi(raw)
		if err != nil {
			return reflect.Value{}, wherr.Errorf("cannot convert value %q to int: %w", raw, err)
		}
		return reflect.ValueOf(v).Convert(target), nil
	case reflect.Int8:
		v, err := strconv.ParseInt(raw, 10, 8)
		if err != nil {
			return reflect.Value{}, wherr.Errorf("cannot convert value %q to int8: %w", raw, err)
		}
		return reflect.ValueOf(int8(v)).Convert(target), nil
	case reflect.Int16:
		v, err := strconv.ParseInt(raw, 10, 16)
		if err != nil {
			return reflect.Value{}, wherr.Errorf("cannot convert value %q to int16: %w", raw, err)
		}
		return reflect.ValueOf(int16(v)).Convert(target), nil
	case reflect.Int32:
		v, err := strconv.ParseInt(raw, 10, 32)
		if err != nil {
			return reflect.Value{}, wherr.Errorf("cannot convert value %q to int32: %w", raw, err)
		}
		return reflect.ValueOf(int32(v)).Convert(target), nil
	case reflect.Int64:
		v, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			return reflect.Value{}, wherr.Errorf("cannot convert value %q to int64: %w", raw, err)
		}
		return reflect.ValueOf(v).Convert(target), nil
	case reflect.Uint:
		v, err := strconv.ParseUint(raw, 10, 0)
		if err != nil {
			return reflect.Value{}, wherr.Errorf("cannot convert value %q to uint: %w", raw, err)
		}
		return reflect.ValueOf(uint(v)).Convert(target), nil
	case reflect.Uint8:
		v, err := strconv.ParseUint(raw, 10, 8)
		if err != nil {
			return reflect.Value{}, wherr.Errorf("cannot convert value %q to uint8: %w", raw, err)
		}
		return reflect.ValueOf(uint8(v)).Convert(target), nil
	case reflect.Uint16:
		v, err := strconv.ParseUint(raw, 10, 16)
		if err != nil {
			return reflect.Value{}, wherr.Errorf("cannot convert value %q to uint16: %w", raw, err)
		}
		return reflect.ValueOf(uint16(v)).Convert(target), nil
	case reflect.Uint32:
		v, err := strconv.ParseUint(raw, 10, 32)
		if err != nil {
			return reflect.Value{}, wherr.Errorf("cannot convert value %q to uint32: %w", raw, err)
		}
		return reflect.ValueOf(uint32(v)).Convert(target), nil
	case reflect.Uint64:
		v, err := strconv.ParseUint(raw, 10, 64)
		if err != nil {
			return reflect.Value{}, wherr.Errorf("cannot convert value %q to uint64: %w", raw, err)
		}
		return reflect.ValueOf(v).Convert(target), nil
	case reflect.Bool:
		v, err := strconv.ParseBool(raw)
		if err != nil {
			return reflect.Value{}, wherr.Errorf("cannot convert value %q to bool: %w", raw, err)
		}
		return reflect.ValueOf(v).Convert(target), nil
	default:
		return reflect.Value{}, wherr.Errorf("unsupported value type: %s", target)
	}
}
