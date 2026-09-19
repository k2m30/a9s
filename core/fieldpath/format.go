// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package fieldpath

import (
	"fmt"
	"reflect"
	"time"
)

// FormatValue auto-formats a reflect.Value based on its Go type.
func FormatValue(val reflect.Value) string {
	if !val.IsValid() {
		return ""
	}

	if val.Kind() == reflect.Pointer {
		if val.IsNil() {
			return ""
		}
		return FormatValue(val.Elem())
	}

	// time.Time is checked BEFORE Kind-based dispatch, since time.Time is a struct.
	if val.Type() == reflect.TypeFor[time.Time]() {
		t := val.Interface().(time.Time)
		if t.IsZero() {
			return ""
		}
		return t.Format("2006-01-02 15:04")
	}

	if val.Kind() == reflect.Bool {
		if val.Bool() {
			return "Yes"
		}
		return "No"
	}

	if val.Kind() == reflect.String {
		return val.String()
	}

	switch val.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return fmt.Sprintf("%d", val.Int())
	}

	switch val.Kind() {
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return fmt.Sprintf("%d", val.Uint())
	}

	switch val.Kind() {
	case reflect.Float32, reflect.Float64:
		return fmt.Sprintf("%g", val.Float())
	}

	return fmt.Sprintf("%v", val.Interface())
}
