// Package resource provides the generic resource model used across all AWS
// resource types in a9s.
package resource

import "strconv"

// CellKind classifies how a string-formatted numeric cell value should be read.
//
//   - CellKindExact:        the number is the true count (e.g., "150").
//   - CellKindApproximate:  the number is a lower bound; more may exist beyond
//     the window the enricher could inspect (e.g., "150+").
//   - CellKindUnknown:      the true count could not be determined; the cell is
//     rendered as an em dash "—".
//
// All three helpers produce strings suitable for direct assignment into
// Resource.Fields[key]. The UI treats the textual suffix/dash as the marker,
// so downstream callers MUST use these helpers rather than ad-hoc string concat.
type CellKind int

const (
	CellKindExact CellKind = iota
	CellKindApproximate
	CellKindUnknown
)

// CellUnknownText is the canonical textual representation of an unknown cell.
// Matches the UI convention already used for truncated-unknown rows in list
// views (em dash "—").
const CellUnknownText = "—"

// FormatExact returns the exact decimal representation of n (e.g., "150").
// Use when the enricher walked the full set without truncation.
func FormatExact(n int) string {
	return strconv.Itoa(n)
}

// FormatApproximate returns n with a trailing "+" (e.g., "150+") signaling
// that n is a lower bound: the true count is >= n. Use when pagination was
// capped, an API error cut the walk short, or the enricher only inspected a
// prefix of the set.
//
// Never concatenate "+" manually onto a numeric string — use this helper so
// the guard test in tests/unit/ can verify nothing else mints approximate
// cells.
func FormatApproximate(n int) string {
	return strconv.Itoa(n) + "+"
}

// FormatUnknown returns the em-dash marker used for cells whose value the
// enricher could not determine at all (e.g., the API call failed before any
// data was observed).
func FormatUnknown() string {
	return CellUnknownText
}
