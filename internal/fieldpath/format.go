package fieldpath

import (
	"fmt"
	"reflect"
	"time"
)

// FormatValue auto-formats a reflect.Value based on its Go type.
func FormatValue(val reflect.Value) string {
	// 1. Invalid (zero Value)
	if !val.IsValid() {
		return ""
	}

	// 2. Pointer: dereference
	if val.Kind() == reflect.Ptr {
		if val.IsNil() {
			return ""
		}
		return FormatValue(val.Elem())
	}

	// 3. time.Time (check BEFORE Kind-based dispatch, since time.Time is a struct)
	if val.Type() == reflect.TypeOf(time.Time{}) {
		t := val.Interface().(time.Time)
		if t.IsZero() {
			return ""
		}
		return t.Format("2006-01-02 15:04:05")
	}

	// 4. Bool
	if val.Kind() == reflect.Bool {
		if val.Bool() {
			return "Yes"
		}
		return "No"
	}

	// 5. String (works for named string types too)
	if val.Kind() == reflect.String {
		return val.String()
	}

	// 6. Int variants
	switch val.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return fmt.Sprintf("%d", val.Int())
	}

	// 7. Uint variants
	switch val.Kind() {
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return fmt.Sprintf("%d", val.Uint())
	}

	// 8. Float variants
	switch val.Kind() {
	case reflect.Float32, reflect.Float64:
		return fmt.Sprintf("%g", val.Float())
	}

	// 9. Default
	return fmt.Sprintf("%v", val.Interface())
}
