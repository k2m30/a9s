package unit_test

import (
	"reflect"
	"strings"

	"github.com/k2m30/a9s/v3/core/catalog"
)

// t9cRowTypes reads the row struct types a catalog entry declares. The field
// is reached by name so a tree without the declaration still compiles and
// the caller reports the type as declaring none.
func t9cRowTypes(td catalog.ResourceTypeDef) []reflect.Type {
	f := reflect.ValueOf(td).FieldByName("RowTypes")
	if !f.IsValid() || f.Kind() != reflect.Slice {
		return nil
	}
	out := make([]reflect.Type, 0, f.Len())
	for i := 0; i < f.Len(); i++ {
		if rt, ok := f.Index(i).Interface().(reflect.Type); ok && rt != nil {
			out = append(out, rt)
		}
	}
	return out
}

func t9cListOmits(td catalog.ResourceTypeDef) []string {
	f := reflect.ValueOf(td).FieldByName("ListOmits")
	if !f.IsValid() || f.Kind() != reflect.Slice {
		return nil
	}
	out := make([]string, 0, f.Len())
	for i := 0; i < f.Len(); i++ {
		if s, ok := f.Index(i).Interface().(string); ok {
			out = append(out, s)
		}
	}
	return out
}

func t9cDeref(t reflect.Type) reflect.Type {
	for t != nil && t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	return t
}

// t9cField finds seg on struct type st the way fieldpath.ExtractValue does on
// a value: a JSON tag, then the exported field name under case folding, then
// a field promoted through an anonymous embed.
func t9cField(st reflect.Type, seg string) (reflect.Type, bool) {
	for i := 0; i < st.NumField(); i++ {
		f := st.Field(i)
		if f.IsExported() && strings.Split(f.Tag.Get("json"), ",")[0] == seg {
			return f.Type, true
		}
	}
	for i := 0; i < st.NumField(); i++ {
		f := st.Field(i)
		if f.IsExported() && strings.EqualFold(f.Name, seg) {
			return f.Type, true
		}
	}
	for i := 0; i < st.NumField(); i++ {
		f := st.Field(i)
		if !f.Anonymous {
			continue
		}
		if et := t9cDeref(f.Type); et.Kind() == reflect.Struct {
			if ft, ok := t9cField(et, seg); ok {
				return ft, true
			}
		}
	}
	return nil, false
}

// t9cResolve walks a dotted detail or column path over a struct type and
// returns the leaf type. A slice is entered through its element, as the
// detail renders every element; a map is a leaf the path cannot enter.
func t9cResolve(rt reflect.Type, path string) (reflect.Type, bool) {
	cur := rt
	for _, seg := range strings.Split(path, ".") {
		seg = strings.TrimSuffix(seg, "[]")
		cur = t9cDeref(cur)
		for cur.Kind() == reflect.Slice || cur.Kind() == reflect.Array {
			cur = t9cDeref(cur.Elem())
		}
		if cur.Kind() != reflect.Struct {
			return nil, false
		}
		next, ok := t9cField(cur, seg)
		if !ok {
			return nil, false
		}
		cur = next
	}
	return cur, true
}

func t9cResolves(rt reflect.Type, path string) bool {
	_, ok := t9cResolve(rt, path)
	return ok
}

func t9cContainsType(types []reflect.Type, rt reflect.Type) bool {
	for _, x := range types {
		if x == rt {
			return true
		}
	}
	return false
}

func t9cComputed(td catalog.ResourceTypeDef, path string) bool {
	for _, c := range td.ComputedDetailPaths {
		if strings.EqualFold(c, path) {
			return true
		}
	}
	return false
}

func t9cAllDefs() []catalog.ResourceTypeDef {
	return append(append([]catalog.ResourceTypeDef{}, catalog.All()...), catalog.AllChildren()...)
}
