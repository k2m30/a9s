package fieldpath

import (
	"reflect"
	"strings"
	"time"
)

// EnumeratePaths recursively walks a struct type and returns all possible
// dot-notation paths based on JSON tags. Pointer types are dereferenced,
// slices get "[]" suffix, and leaf types (string, int, bool, time.Time,
// named string types) terminate recursion.
func EnumeratePaths(t reflect.Type, prefix string) []string {
	// Unwrap pointer
	for t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	// Must be a struct to enumerate
	if t.Kind() != reflect.Struct {
		return nil
	}

	// time.Time is a leaf
	if t == reflect.TypeOf(time.Time{}) {
		if prefix != "" {
			return []string{prefix}
		}
		return nil
	}

	var paths []string
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)

		// Skip unexported fields
		if !field.IsExported() {
			continue
		}

		// Get path name: prefer json tag, fall back to Go field name.
		// AWS SDK Go v2 structs have no json tags, so field name is used directly.
		// ExtractValue matches case-insensitively, so users can write paths in any case.
		name := ""
		tag := field.Tag.Get("json")
		if tag == "-" {
			continue
		}
		if tag != "" {
			name = strings.Split(tag, ",")[0]
		}
		if name == "" {
			name = field.Name
		}

		fullPath := name
		if prefix != "" {
			fullPath = prefix + "." + name
		}

		ft := field.Type
		// Unwrap pointer
		for ft.Kind() == reflect.Ptr {
			ft = ft.Elem()
		}

		// Check if leaf type
		if isLeafType(ft) {
			paths = append(paths, fullPath)
			continue
		}

		// Slice: recurse into element type with [] suffix
		if ft.Kind() == reflect.Slice {
			elemType := ft.Elem()
			for elemType.Kind() == reflect.Ptr {
				elemType = elemType.Elem()
			}
			if isLeafType(elemType) {
				paths = append(paths, fullPath+"[]")
			} else if elemType.Kind() == reflect.Struct {
				sub := EnumeratePaths(elemType, fullPath+"[]")
				paths = append(paths, sub...)
			}
			continue
		}

		// Struct: recurse
		if ft.Kind() == reflect.Struct {
			sub := EnumeratePaths(ft, fullPath)
			paths = append(paths, sub...)
			continue
		}

		// Map or other: just add the path
		paths = append(paths, fullPath)
	}
	return paths
}

func isLeafType(t reflect.Type) bool {
	if t == reflect.TypeOf(time.Time{}) {
		return true
	}
	switch t.Kind() {
	case reflect.String, reflect.Bool,
		reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64:
		return true
	}
	return false
}
