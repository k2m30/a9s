package fieldpath

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// ExtractValue navigates a struct using a dot-separated path matched against JSON tags.
func ExtractValue(obj any, dotPath string) (reflect.Value, error) {
	segments := strings.Split(dotPath, ".")
	current := reflect.ValueOf(obj)

	for _, seg := range segments {
		// Dereference pointers
		for current.Kind() == reflect.Pointer {
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
	if val.Type() == reflect.TypeFor[time.Time]() {
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
func ExtractScalar(obj any, dotPath string) string {
	val, err := ExtractValue(obj, dotPath)
	if err != nil {
		return ""
	}

	// Dereference pointer
	for val.Kind() == reflect.Pointer {
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
func ExtractSubtree(obj any, dotPath string) string {
	val, err := ExtractValue(obj, dotPath)
	if err != nil {
		// Fallback for paths that traverse slices without explicit indexes,
		// e.g. "SecurityGroups.GroupId" on []GroupIdentifier.
		if vals := extractMultiScalars(reflect.ValueOf(obj), strings.Split(dotPath, ".")); len(vals) > 0 {
			return strings.Join(vals, "\n")
		}
		return ""
	}

	// Dereference pointer
	for val.Kind() == reflect.Pointer {
		if val.IsNil() {
			return ""
		}
		val = val.Elem()
	}

	// Scalar types: format directly (but try JSON parsing for strings first)
	if isScalar(val) {
		if val.Kind() == reflect.String {
			if parsed := tryParseJSON(val.String()); parsed != nil {
				out, err := yaml.Marshal(parsed)
				if err == nil {
					return strings.TrimRight(string(out), "\n")
				}
			}
		}
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

// extractMultiScalars walks a dot path and supports traversing slices/arrays
// without explicit indices. It returns all scalar leaf values found.
func extractMultiScalars(v reflect.Value, segments []string) []string {
	for v.IsValid() && v.Kind() == reflect.Pointer {
		if v.IsNil() {
			return nil
		}
		v = v.Elem()
	}
	if !v.IsValid() {
		return nil
	}

	if len(segments) == 0 {
		if isScalar(v) {
			return []string{FormatValue(v)}
		}
		if v.Kind() == reflect.Slice || v.Kind() == reflect.Array {
			var out []string
			for i := 0; i < v.Len(); i++ {
				out = append(out, extractMultiScalars(v.Index(i), segments)...)
			}
			return out
		}
		return nil
	}

	seg := segments[0]
	switch v.Kind() {
	case reflect.Struct:
		t := v.Type()
		for i := 0; i < t.NumField(); i++ {
			f := t.Field(i)
			tag := f.Tag.Get("json")
			if tag != "" {
				jsonName := strings.Split(tag, ",")[0]
				if jsonName == seg {
					return extractMultiScalars(v.Field(i), segments[1:])
				}
			}
		}
		for i := 0; i < t.NumField(); i++ {
			f := t.Field(i)
			if strings.EqualFold(f.Name, seg) {
				return extractMultiScalars(v.Field(i), segments[1:])
			}
		}
		return nil
	case reflect.Slice, reflect.Array:
		var out []string
		for i := 0; i < v.Len(); i++ {
			out = append(out, extractMultiScalars(v.Index(i), segments)...)
		}
		return out
	default:
		return nil
	}
}

// ToSafeValue recursively converts a reflect.Value into a representation
// that only contains exported fields, safe for yaml.Marshal.
func ToSafeValue(val reflect.Value) any {
	for val.Kind() == reflect.Pointer {
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
		m := make(map[string]any)
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
		var result []any
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
		m := make(map[string]any)
		for _, key := range val.MapKeys() {
			m[fmt.Sprintf("%v", key.Interface())] = ToSafeValue(val.MapIndex(key))
		}
		return m

	default:
		if isScalar(val) {
			if val.Kind() == reflect.String {
				if parsed := tryParseJSON(val.String()); parsed != nil {
					return parsed
				}
			}
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
	case reflect.Pointer, reflect.Interface:
		return v.IsNil()
	case reflect.Slice, reflect.Map:
		return v.IsNil() || v.Len() == 0
	default:
		return v.IsZero()
	}
}

// tryParseJSON attempts to parse s as JSON. Returns the parsed structure
// (map/slice/scalar) on success, or nil on failure.
// Only attempts parsing if s starts with '{' or '[' (quick rejection for non-JSON strings).
func tryParseJSON(s string) any {
	s = strings.TrimSpace(s)
	if len(s) == 0 || (s[0] != '{' && s[0] != '[') {
		return nil
	}
	var parsed any
	if err := json.Unmarshal([]byte(s), &parsed); err != nil {
		return nil
	}
	return parsed
}

// FieldItem is one rendered line from the structured detail extraction pipeline.
type FieldItem struct {
	Path        string // original path key (e.g., "VpcId", "SecurityGroups")
	Key         string // display key (typically = Path for top-level)
	Value       string // rendered value (empty for section headers)
	IsHeader    bool   // true when value is multi-line (section header line)
	IsSubField  bool   // true for lines indented under a section header
	IndentLevel int    // 0 = top-level, 1 = sub-field
	IsNavigable bool   // true when FieldPath matches a NavigableField
	TargetType  string // non-empty when IsNavigable (e.g., "vpc")
	IsSection   bool   // NEW (v2.1): true for ct-events top-level section headers (ACTOR/ACTION/TARGET/CONTEXT/...)
	// Used only by the ct-events detail view branch; inert for all other resource types.
	ColorTier string // NEW (v2.1): severity tier for value coloring ("ct-info"|"ct-attention"|"ct-danger")
	// Set only on the Event row in ACTION by ct-events. Empty string falls through to neutral DetailVal.
	NavID string // Navigation ID override — used by ct-events Principal rows where the display Value is the
	// full ARN but navigation needs the bare name. Inert when empty.
}

// ToSnakeCase converts PascalCase to snake_case: "InstanceId" → "instance_id".
func ToSnakeCase(s string) string {
	var result strings.Builder
	for i, r := range s {
		if r >= 'A' && r <= 'Z' {
			if i > 0 {
				result.WriteByte('_')
			}
			result.WriteRune(r + 32) // toLower
		} else {
			result.WriteRune(r)
		}
	}
	return result.String()
}

// ExtractFieldList builds a structured []FieldItem for the given field paths.
//
// For each path in paths:
//  1. Check fields map first (case-insensitive + snake_case fallback)
//  2. If not found in map, call ExtractSubtree(obj, path)
//  3. If still empty, set Value = "-"
//
// If the extracted value contains "\n" (multi-line — struct/slice/map from ExtractSubtree):
//   - Emit one header FieldItem{Path: path, Key: path, IsHeader: true, Value: ""}
//   - Then one sub-field FieldItem per line: {Path: path, Key: lineContent, IsSubField: true, IndentLevel: 1}
//
// For scalar values:
//   - Emit one FieldItem{Path: path, Key: path, Value: value}
//
// Check navigable map for IsNavigable/TargetType annotation on top-level scalar items.
//
// Always returns a non-nil slice (empty []FieldItem{} for empty paths).
func ExtractFieldList(obj any, fields map[string]string, paths []string, navigable map[string]string) []FieldItem {
	if len(paths) == 0 {
		return []FieldItem{}
	}
	var items []FieldItem
	for _, path := range paths {
		val := ""
		nonScalarFromObj := false
		nonScalarFromFields := false
		// 1. Check fields map first (case-insensitive match)
		if len(fields) > 0 {
			for k, v := range fields {
				if strings.EqualFold(k, path) {
					val = v
					break
				}
			}
			// Try snake_case if not found
			if val == "" {
				snakeKey := ToSnakeCase(path)
				if v, ok := fields[snakeKey]; ok {
					val = v
				}
			}
			if nested := extractNestedFieldLines(fields, path); len(nested) > 0 {
				nonScalarFromFields = true
				val = strings.Join(nested, "\n")
			}
		}
		// 2. Fall back to ExtractSubtree
		if val == "" && obj != nil {
			val = ExtractSubtree(obj, path)
			if val != "" {
				if rv, err := ExtractValue(obj, path); err == nil {
					for rv.Kind() == reflect.Pointer {
						if rv.IsNil() {
							break
						}
						rv = rv.Elem()
					}
					nonScalarFromObj = !isScalar(rv)
				}
			}
		}
		// 3. Default to "-"
		if val == "" {
			val = "-"
		}

		// Check navigability for this path
		targetType := ""
		isNavigable := false
		if navigable != nil {
			if tt, ok := navigable[path]; ok {
				targetType = tt
				isNavigable = true
			}
		}

		// Multi-line (or non-scalar single-line YAML) → header + sub-fields.
		if strings.Contains(val, "\n") || (nonScalarFromObj && val != "-") || (nonScalarFromFields && val != "-") {
			items = append(items, FieldItem{
				Path:     path,
				Key:      path,
				Value:    "",
				IsHeader: true,
			})
			lines := strings.Split(val, "\n")
			if len(lines) == 1 {
				// Single-line YAML object, e.g. "Arn: arn:..."
				lines = []string{val}
			}
			for _, line := range lines {
				if line == "" {
					continue
				}
				items = append(items, FieldItem{
					Path:        path,
					Key:         line,
					Value:       line,
					IsSubField:  true,
					IndentLevel: 1,
				})
			}
		} else {
			// Scalar
			items = append(items, FieldItem{
				Path:        path,
				Key:         path,
				Value:       val,
				IsNavigable: isNavigable,
				TargetType:  targetType,
			})
		}
	}
	return items
}

type nestedFieldLine struct {
	index int
	key   string
	value string
}

// extractNestedFieldLines expands dotted map keys under a section path.
// Example:
//
//	path = "SecurityGroups"
//	map keys = SecurityGroups.GroupId.0, SecurityGroups.GroupName.0
//
// Produces lines:
//
//	GroupId: <...>
//	GroupName: <...>
func extractNestedFieldLines(fields map[string]string, path string) []string {
	prefix := path + "."
	var lines []nestedFieldLine
	for k, v := range fields {
		if !strings.HasPrefix(k, prefix) {
			continue
		}
		rest := strings.TrimPrefix(k, prefix)
		if rest == "" {
			continue
		}
		parts := strings.Split(rest, ".")
		key := parts[0]
		if key == "" {
			continue
		}
		idx := 0
		for _, p := range parts[1:] {
			if n, err := strconv.Atoi(p); err == nil {
				idx = n
				break
			}
		}
		lines = append(lines, nestedFieldLine{index: idx, key: key, value: v})
	}
	if len(lines) == 0 {
		return nil
	}
	sort.SliceStable(lines, func(i, j int) bool {
		if lines[i].index != lines[j].index {
			return lines[i].index < lines[j].index
		}
		return lines[i].key < lines[j].key
	})
	out := make([]string, 0, len(lines))
	for _, l := range lines {
		out = append(out, l.key+": "+l.value)
	}
	return out
}
