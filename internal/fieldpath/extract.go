package fieldpath

import (
	"fmt"
	"reflect"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// ExtractValue navigates a struct using a dot-separated path matched against JSON tags.
func ExtractValue(obj interface{}, dotPath string) (reflect.Value, error) {
	segments := strings.Split(dotPath, ".")
	current := reflect.ValueOf(obj)

	for _, seg := range segments {
		// Dereference pointers
		for current.Kind() == reflect.Ptr {
			if current.IsNil() {
				return reflect.Value{}, fmt.Errorf("nil pointer at segment %q", seg)
			}
			current = current.Elem()
		}

		if current.Kind() != reflect.Struct {
			return reflect.Value{}, fmt.Errorf("expected struct at segment %q, got %v", seg, current.Kind())
		}

		// Find field by JSON tag first, then by field name (case-insensitive).
		// AWS SDK Go v2 structs have no JSON tags, so field name matching is essential.
		found := false
		t := current.Type()
		for i := 0; i < t.NumField(); i++ {
			field := t.Field(i)
			tag := field.Tag.Get("json")
			if tag != "" {
				jsonName := strings.Split(tag, ",")[0]
				if jsonName == seg {
					current = current.Field(i)
					found = true
					break
				}
			}
		}
		if !found {
			for i := 0; i < t.NumField(); i++ {
				field := t.Field(i)
				if strings.EqualFold(field.Name, seg) {
					current = current.Field(i)
					found = true
					break
				}
			}
		}

		if !found {
			return reflect.Value{}, fmt.Errorf("no field matching %q", seg)
		}
	}

	return current, nil
}

// isScalar reports whether a reflect.Value holds a scalar type.
func isScalar(val reflect.Value) bool {
	if val.Type() == reflect.TypeOf(time.Time{}) {
		return true
	}
	switch val.Kind() {
	case reflect.String, reflect.Bool,
		reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64:
		return true
	}
	return false
}

// ExtractScalar extracts a scalar value as a formatted string.
// Returns "" for non-scalar types (slices, maps, structs), errors, or nil pointers.
func ExtractScalar(obj interface{}, dotPath string) string {
	val, err := ExtractValue(obj, dotPath)
	if err != nil {
		return ""
	}

	// Dereference pointer
	for val.Kind() == reflect.Ptr {
		if val.IsNil() {
			return ""
		}
		val = val.Elem()
	}

	// Non-scalar types return ""
	if !isScalar(val) {
		return ""
	}

	return FormatValue(val)
}

// ExtractSubtree extracts a value and returns it as a formatted string (scalar)
// or YAML (struct/slice/map).
func ExtractSubtree(obj interface{}, dotPath string) string {
	val, err := ExtractValue(obj, dotPath)
	if err != nil {
		return ""
	}

	// Dereference pointer
	for val.Kind() == reflect.Ptr {
		if val.IsNil() {
			return ""
		}
		val = val.Elem()
	}

	// Scalar types: format directly
	if isScalar(val) {
		return FormatValue(val)
	}

	// Struct, slice, map: convert to exported-fields-only map before YAML marshal.
	// AWS SDK structs contain unexported fields that cause yaml.Marshal to panic.
	switch val.Kind() {
	case reflect.Struct, reflect.Slice, reflect.Map:
		safe := ToSafeValue(val)
		if safe == nil {
			return ""
		}
		out, err := yaml.Marshal(safe)
		if err != nil {
			return ""
		}
		return strings.TrimRight(string(out), "\n")
	}

	return FormatValue(val)
}

// ToSafeValue recursively converts a reflect.Value into a representation
// that only contains exported fields, safe for yaml.Marshal.
func ToSafeValue(val reflect.Value) interface{} {
	for val.Kind() == reflect.Ptr {
		if val.IsNil() {
			return nil
		}
		val = val.Elem()
	}

	switch val.Kind() {
	case reflect.Struct:
		if isScalar(val) {
			return FormatValue(val)
		}
		m := make(map[string]interface{})
		t := val.Type()
		for i := 0; i < t.NumField(); i++ {
			field := t.Field(i)
			if !field.IsExported() {
				continue
			}
			fv := val.Field(i)
			if isZeroOrNil(fv) {
				continue
			}
			// Prefer json tag name, fall back to Go field name
			name := field.Name
			if tag := field.Tag.Get("json"); tag != "" && tag != "-" {
				if n := strings.Split(tag, ",")[0]; n != "" {
					name = n
				}
			}
			sv := ToSafeValue(fv)
			if sv != nil {
				m[name] = sv
			}
		}
		if len(m) == 0 {
			return nil
		}
		return m

	case reflect.Slice:
		if val.Len() == 0 {
			return nil
		}
		var result []interface{}
		for i := 0; i < val.Len(); i++ {
			sv := ToSafeValue(val.Index(i))
			if sv != nil {
				result = append(result, sv)
			}
		}
		if len(result) == 0 {
			return nil
		}
		return result

	case reflect.Map:
		if val.Len() == 0 {
			return nil
		}
		m := make(map[string]interface{})
		for _, key := range val.MapKeys() {
			m[fmt.Sprintf("%v", key.Interface())] = ToSafeValue(val.MapIndex(key))
		}
		return m

	default:
		if isScalar(val) {
			return FormatValue(val)
		}
		if val.CanInterface() {
			return val.Interface()
		}
		return fmt.Sprintf("%v", val)
	}
}

// isZeroOrNil checks if a value is nil (for pointers/slices/maps) or the zero value.
func isZeroOrNil(v reflect.Value) bool {
	switch v.Kind() {
	case reflect.Ptr, reflect.Interface:
		return v.IsNil()
	case reflect.Slice, reflect.Map:
		return v.IsNil() || v.Len() == 0
	default:
		return v.IsZero()
	}
}
