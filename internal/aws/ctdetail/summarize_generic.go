package ctdetail

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// SummarizeGeneric is the fallback summarizer for unrecognized event sources.
// It performs a flat walk over params, emitting one Row per top-level key.
// Values are rendered as human-readable strings; nested maps and slices are
// rendered compactly via fmt.Sprintf rather than being recursed into.
//
// SummarizeGeneric is called directly from BuildSections when no service-specific
// summarizer is registered for event.EventSource. It is never registered in
// summarizerByService — it is always available as a direct call.
//
// See specs/013-ct-event-detail-v2/contracts/ctdetail-api.md for the Summarizer contract.
//
// Guarantees:
//   - Returns a non-nil slice (empty []Row{} when params is nil or empty).
//   - Does not mutate params.
//   - Does not panic on nil or empty params.
//   - Pure function.
func SummarizeGeneric(_ string, params map[string]any) []Row {
	rows := []Row{}
	if len(params) == 0 {
		return rows
	}

	// Copy keys to sort — never mutate params.
	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		v := params[k]
		rows = append(rows, Row{
			Key:         k,
			Value:       renderGenericValue(v),
			IsNavigable: false,
			TargetType:  "",
		})
	}
	return rows
}

// renderGenericValue converts an arbitrary value to a display string.
// It does not recurse deeply — nested maps render as compact {k: v} strings.
func renderGenericValue(v any) string {
	if v == nil {
		return ""
	}
	switch val := v.(type) {
	case string:
		return val
	case bool:
		if val {
			return "true"
		}
		return "false"
	case int:
		return fmt.Sprintf("%d", val)
	case int64:
		return fmt.Sprintf("%d", val)
	case float64:
		return fmt.Sprintf("%v", val)
	case json.Number:
		return val.String()
	case []any:
		return renderSlice(val)
	default:
		return fmt.Sprintf("%v", v)
	}
}

// renderSlice joins primitive elements with ", " wrapped in brackets.
// Non-primitive elements fall back to fmt.Sprintf.
func renderSlice(s []any) string {
	parts := make([]string, len(s))
	for i, elem := range s {
		switch ev := elem.(type) {
		case string:
			parts[i] = ev
		case bool:
			if ev {
				parts[i] = "true"
			} else {
				parts[i] = "false"
			}
		default:
			parts[i] = fmt.Sprintf("%v", elem)
		}
	}
	return "[" + strings.Join(parts, ", ") + "]"
}
