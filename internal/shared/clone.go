package shared

import "reflect"

type cloneVisit struct {
	typ reflect.Type
	ptr uintptr
}

// CloneInterfaceValue performs a deep clone of an arbitrary interface value using reflection.
func CloneInterfaceValue(value interface{}) interface{} {
	if value == nil {
		return nil
	}
	cloned := cloneReflectValue(reflect.ValueOf(value), make(map[cloneVisit]reflect.Value))
	if !cloned.IsValid() {
		return nil
	}
	return cloned.Interface()
}

// CloneStringInterfaceMap performs a deep clone of a map[string]interface{}.
// It delegates to CloneInterfaceValue to preserve cyclic references within the map.
func CloneStringInterfaceMap(source map[string]interface{}) map[string]interface{} {
	if source == nil {
		return nil
	}
	cloned, ok := CloneInterfaceValue(source).(map[string]interface{})
	if !ok {
		return nil
	}
	return cloned
}

func cloneReflectValue(value reflect.Value, seen map[cloneVisit]reflect.Value) reflect.Value {
	if !value.IsValid() {
		return value
	}

	switch value.Kind() {
	case reflect.Map:
		if value.IsNil() {
			return reflect.Zero(value.Type())
		}

		visit := cloneVisit{typ: value.Type(), ptr: value.Pointer()}
		if visit.ptr != 0 {
			if cached, ok := seen[visit]; ok {
				return cached
			}
		}

		clonedMap := reflect.MakeMapWithSize(value.Type(), value.Len())
		if visit.ptr != 0 {
			seen[visit] = clonedMap
		}
		for _, key := range value.MapKeys() {
			clonedKey := cloneReflectValue(key, seen)
			clonedMap.SetMapIndex(clonedKey, cloneReflectValue(value.MapIndex(key), seen))
		}
		return clonedMap

	case reflect.Slice:
		if value.IsNil() {
			return reflect.Zero(value.Type())
		}

		visit := cloneVisit{typ: value.Type(), ptr: value.Pointer()}
		if visit.ptr != 0 {
			if cached, ok := seen[visit]; ok {
				return cached
			}
		}

		clonedSlice := reflect.MakeSlice(value.Type(), value.Len(), value.Len())
		if visit.ptr != 0 {
			seen[visit] = clonedSlice
		}
		for i := 0; i < value.Len(); i++ {
			clonedSlice.Index(i).Set(cloneReflectValue(value.Index(i), seen))
		}
		return clonedSlice

	case reflect.Array:
		clonedArray := reflect.New(value.Type()).Elem()
		for i := 0; i < value.Len(); i++ {
			clonedArray.Index(i).Set(cloneReflectValue(value.Index(i), seen))
		}
		return clonedArray

	case reflect.Ptr:
		if value.IsNil() {
			return reflect.Zero(value.Type())
		}

		visit := cloneVisit{typ: value.Type(), ptr: value.Pointer()}
		if cached, ok := seen[visit]; ok {
			return cached
		}

		clonedPointer := reflect.New(value.Type().Elem())
		seen[visit] = clonedPointer
		clonedPointer.Elem().Set(cloneReflectValue(value.Elem(), seen))
		return clonedPointer

	case reflect.Interface:
		if value.IsNil() {
			return reflect.Zero(value.Type())
		}
		return cloneReflectValue(value.Elem(), seen)

	case reflect.Struct:
		clonedStruct := reflect.New(value.Type()).Elem()
		clonedStruct.Set(value)
		for i := 0; i < value.NumField(); i++ {
			destinationField := clonedStruct.Field(i)
			if !destinationField.CanSet() {
				continue
			}

			clonedField := cloneReflectValue(value.Field(i), seen)
			if !clonedField.IsValid() {
				continue
			}

			if clonedField.Type().AssignableTo(destinationField.Type()) {
				destinationField.Set(clonedField)
			} else if clonedField.Type().ConvertibleTo(destinationField.Type()) {
				destinationField.Set(clonedField.Convert(destinationField.Type()))
			}
		}
		return clonedStruct

	default:
		return value
	}
}
