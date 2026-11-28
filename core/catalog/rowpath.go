// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package catalog

import (
	"reflect"
	"slices"
	"strings"
	"time"
)

// CellPath reports whether a list cell can read path from a row of this
// type: on at least one RowTypes struct, every segment names a field the way
// fieldpath.ExtractValue resolves one, no segment crosses a slice or a map,
// and the leaf is a scalar. Any other path renders blank on every row.
func (d ResourceTypeDef) CellPath(path string) bool {
	for _, rt := range d.RowTypes {
		if scalarLeaf(rt, path) {
			return true
		}
	}
	return false
}

// Produces reports whether key is a Fields key the type's fetcher, its
// Wave 2 enricher or a field alias writes. A column that reads such a key is
// answered by Fields, an empty value included, and reads no path.
func (d ResourceTypeDef) Produces(key string) bool {
	if key == "" {
		return false
	}
	if slices.Contains(d.FieldKeys, key) || slices.Contains(d.IssueEnricherFieldKeys, key) {
		return true
	}
	for _, alias := range d.FieldAliases {
		if alias == key {
			return true
		}
	}
	return false
}

// ReadsPath reports whether a column keyed key reads path from the row's
// struct: it names a path, and its key is none the type produces. The cell,
// the render-time materialization and the save lane all ask it, so a
// produced key is never written from a struct member its producer left
// empty. A nil d is a column set no type owns.
func (d *ResourceTypeDef) ReadsPath(key, path string) bool {
	return path != "" && (d == nil || !d.Produces(key))
}

func scalarLeaf(rt reflect.Type, path string) bool {
	cur := rt
	for seg := range strings.SplitSeq(path, ".") {
		cur = derefType(cur)
		if cur.Kind() != reflect.Struct {
			return false
		}
		next, ok := fieldOfType(cur, seg)
		if !ok {
			return false
		}
		cur = next
	}
	cur = derefType(cur)
	if cur == reflect.TypeFor[time.Time]() {
		return true
	}
	switch cur.Kind() {
	case reflect.String, reflect.Bool,
		reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64:
		return true
	}
	return false
}

func derefType(t reflect.Type) reflect.Type {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	return t
}

// fieldOfType is fieldpath's resolveField on a type: a JSON tag, then the
// exported name under case folding, then a field promoted through an
// anonymous struct embed. active holds the embeds already being searched: a
// type that embeds itself, directly or through another, adds no field the
// outer search has not looked at.
func fieldOfType(st reflect.Type, seg string, active ...reflect.Type) (reflect.Type, bool) {
	for f := range st.Fields() {
		if tag := f.Tag.Get("json"); f.IsExported() && tag != "" && strings.Split(tag, ",")[0] == seg {
			return f.Type, true
		}
	}
	for f := range st.Fields() {
		if f.IsExported() && strings.EqualFold(f.Name, seg) {
			return f.Type, true
		}
	}
	for f := range st.Fields() {
		if !f.Anonymous || strings.Split(f.Tag.Get("json"), ",")[0] != "" {
			continue
		}
		if et := derefType(f.Type); et.Kind() == reflect.Struct && et != st && !slices.Contains(active, et) {
			if ft, ok := fieldOfType(et, seg, slices.Concat(active, []reflect.Type{st})...); ok {
				return ft, true
			}
		}
	}
	return nil, false
}
