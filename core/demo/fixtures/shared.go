// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package fixtures

import (
	"reflect"
	"sync"
)

// sharedSets is every fixture set shared() built, for MutationCheckForTest.
var sharedSets []func() any

// shared builds a fixture set once and hands every caller its own deep copy:
// a demo fake, a test or a fetcher that writes into what it was given writes
// into its copy, never into the set every later caller reads.
func shared[T any](build func() T) func() T {
	once := sync.OnceValue(build)
	sharedSets = append(sharedSets, func() any { return once() })
	return func() T { return deepCopy(once()) }
}

// watchedVars are the exported fixture variables a consumer can reach
// without a copy.
func watchedVars() map[string]any {
	return map[string]any{
		"CostsAnchorMonth":           CostsAnchorMonth,
		"CostsGrowthMonth":           CostsGrowthMonth,
		"CostsResourceRowsByService": CostsResourceRowsByService,
		"EKSVersionSupport":          EKSVersionSupport,
		"EKSEndOfStandardSupport":    EKSEndOfStandardSupport,
		"Pins":                       pins,
	}
}

// MutationCheckForTest takes a copy of every fixture set and exported
// fixture variable, and returns a func naming the ones that differ from it
// when called. Test-only: a test binary calls it before its tests and the
// returned func after them.
func MutationCheckForTest() func() []string {
	sets := make([]any, len(sharedSets))
	for i, set := range sharedSets {
		sets[i] = deepCopy(set())
	}
	vars := deepCopy(watchedVars())
	return func() []string {
		var changed []string
		for i, set := range sharedSets {
			if !reflect.DeepEqual(set(), sets[i]) {
				changed = append(changed, reflect.TypeOf(sets[i]).String())
			}
		}
		for name, v := range watchedVars() {
			if !reflect.DeepEqual(v, vars[name]) {
				changed = append(changed, name)
			}
		}
		return changed
	}
}

// deepCopy returns v with every pointer, slice, map and interface it reaches
// through exported fields copied. Unexported fields are copied as values:
// in the SDK types they are markers and in time.Time a shared *Location.
func deepCopy[T any](v T) T {
	src := reflect.ValueOf(&v).Elem()
	dst := reflect.New(src.Type()).Elem()
	copyValue(dst, src)
	return dst.Interface().(T)
}

func copyValue(dst, src reflect.Value) {
	switch src.Kind() {
	case reflect.Pointer:
		if src.IsNil() {
			return
		}
		p := reflect.New(src.Elem().Type())
		copyValue(p.Elem(), src.Elem())
		dst.Set(p)
	case reflect.Interface:
		if src.IsNil() {
			return
		}
		inner := reflect.New(src.Elem().Type()).Elem()
		copyValue(inner, src.Elem())
		dst.Set(inner)
	case reflect.Slice:
		if src.IsNil() {
			return
		}
		s := reflect.MakeSlice(src.Type(), src.Len(), src.Len())
		for i := range src.Len() {
			copyValue(s.Index(i), src.Index(i))
		}
		dst.Set(s)
	case reflect.Array:
		for i := range src.Len() {
			copyValue(dst.Index(i), src.Index(i))
		}
	case reflect.Map:
		if src.IsNil() {
			return
		}
		m := reflect.MakeMapWithSize(src.Type(), src.Len())
		for iter := src.MapRange(); iter.Next(); {
			v := reflect.New(iter.Value().Type()).Elem()
			copyValue(v, iter.Value())
			m.SetMapIndex(iter.Key(), v)
		}
		dst.Set(m)
	case reflect.Struct:
		dst.Set(src)
		for i := range src.NumField() {
			if dst.Field(i).CanSet() {
				copyValue(dst.Field(i), src.Field(i))
			}
		}
	default:
		dst.Set(src)
	}
}
