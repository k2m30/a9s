package views

import (
	"reflect"
	"sort"
	"strconv"
	"time"

	"github.com/k2m30/a9s/v3/internal/fieldpath"
)

// SortColNone indicates no active sort column.
const SortColNone = -1

// compareRaw extracts raw values from two structs at the given path and compares
// them directly (numeric, time). Returns comparison result and true if raw
// comparison succeeded. Falls back to false when path doesn't resolve or types
// aren't comparable.
func compareRaw(a, b any, path string) (int, bool) {
	va, errA := fieldpath.ExtractValue(a, path)
	vb, errB := fieldpath.ExtractValue(b, path)
	if errA != nil || errB != nil {
		return 0, false
	}
	for va.Kind() == reflect.Pointer {
		if va.IsNil() {
			return 0, false
		}
		va = va.Elem()
	}
	for vb.Kind() == reflect.Pointer {
		if vb.IsNil() {
			return 0, false
		}
		vb = vb.Elem()
	}
	// time.Time
	if va.Type() == reflect.TypeFor[time.Time]() && vb.Type() == reflect.TypeFor[time.Time]() {
		return va.Interface().(time.Time).Compare(vb.Interface().(time.Time)), true
	}
	// Numeric (int/float)
	fa, okA := toFloat(va)
	fb, okB := toFloat(vb)
	if okA && okB {
		if fa < fb {
			return -1, true
		}
		if fa > fb {
			return 1, true
		}
		return 0, true
	}
	return 0, false
}

func toFloat(v reflect.Value) (float64, bool) {
	switch v.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return float64(v.Int()), true
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return float64(v.Uint()), true
	case reflect.Float32, reflect.Float64:
		return v.Float(), true
	default:
		return 0, false
	}
}

// sortFiltered sorts filteredResources in place based on current sort settings.
func (m *ResourceListModel) sortFiltered() {
	if m.sortColIdx == SortColNone {
		return
	}

	cols := m.resolveColumns()
	if m.sortColIdx >= len(cols) {
		return
	}

	col := cols[m.sortColIdx]

	sort.SliceStable(m.filteredResources, func(i, j int) bool {
		a := m.filteredResources[i]
		b := m.filteredResources[j]

		// Try raw struct comparison for path-backed columns (gives correct
		// numeric/time ordering without parsing humanized display strings).
		// sortPath overrides path when the display path differs from the sort path.
		rawPath := col.sortPath
		if rawPath == "" {
			rawPath = col.path
		}
		if rawPath != "" && a.RawStruct != nil && b.RawStruct != nil {
			if cmp, ok := compareRaw(a.RawStruct, b.RawStruct, rawPath); ok {
				if m.sortAsc {
					return cmp < 0
				}
				return cmp > 0
			}
		}

		// Fall back to display value comparison.
		var va, vb string
		if col.sortKey != "" {
			va = a.Fields[col.sortKey]
			vb = b.Fields[col.sortKey]
		} else {
			va = m.extractCellValue(col, a)
			vb = m.extractCellValue(col, b)
		}
		// Try numeric comparison for plain number strings (e.g. sortKey raw values).
		if fa, err := strconv.ParseFloat(va, 64); err == nil {
			if fb, err := strconv.ParseFloat(vb, 64); err == nil {
				if m.sortAsc {
					return fa < fb
				}
				return fa > fb
			}
		}
		if m.sortAsc {
			return va < vb
		}
		return va > vb
	})
}
