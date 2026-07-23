// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// Package jsonyaml holds renderer-free JSON→YAML text helpers shared by the
// app core (core/semantics/projection) and the TUI views. It lives outside
// internal/tui so shared-core packages can use it without transitively pulling
// in lipgloss (SC-009): the lipgloss-dependent text helpers stay in
// internal/tui/text.
package jsonyaml

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/big"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// YAMLIndentSpaces is the indent width emitted by TryJSONToYAMLLines. Callers
// that infer nesting depth from leading-space counts MUST divide by this
// constant rather than hard-coding 2 — it is configured via yaml.NewEncoder
// because yaml.Marshal's default is 4.
const YAMLIndentSpaces = 2

// TryJSONToYAMLLines attempts to parse s as JSON. On success, it converts the
// parsed structure to YAML and returns the individual lines. Returns nil if s
// is not valid JSON, or represents an empty object ({}) or empty array ([]).
//
// Lines are indented at YAMLIndentSpaces spaces per nesting level.
func TryJSONToYAMLLines(s string) []string {
	s = strings.TrimSpace(s)
	if len(s) == 0 {
		return nil
	}
	if s[0] != '{' && s[0] != '[' {
		return nil
	}
	dec := json.NewDecoder(strings.NewReader(s))
	dec.UseNumber()
	var parsed any
	if err := dec.Decode(&parsed); err != nil {
		return nil
	}
	// json.Unmarshal rejects trailing content; a single Decode does not —
	// keep the stricter contract.
	if dec.More() {
		return nil
	}
	parsed = NormalizeJSONNumbers(parsed)
	switch v := parsed.(type) {
	case map[string]any:
		if len(v) == 0 {
			return nil
		}
	case []any:
		if len(v) == 0 {
			return nil
		}
	}
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(YAMLIndentSpaces)
	if err := enc.Encode(parsed); err != nil {
		return nil
	}
	if err := enc.Close(); err != nil {
		return nil
	}
	raw := strings.TrimRight(buf.String(), "\n")
	if raw == "" {
		return nil
	}
	return strings.Split(raw, "\n")
}

// NormalizeJSONNumbers recursively walks v (as decoded by a json.Decoder with
// UseNumber() enabled) and converts each json.Number to the narrowest native
// type that renders it losslessly AND bare — the quoted-scalar convention
// UseNumber otherwise forces on yaml.v3 belongs only to the SDK-struct
// rendering path, not to a parsed JSON document:
//
//  1. Int64() succeeds -> int64 (ordinary signed integers).
//  2. Else a non-negative integer parses via strconv.ParseUint -> uint64
//     (integers between max int64 and max uint64).
//  3. Else, only when s contains '.'/'e'/'E' (i.e. is not a bare integer
//     token — a plain integer this big already had its shot at cases 1/2),
//     Float64() succeeds AND the decimal string's exact rational value
//     (big.Rat.SetString(s)) equals the float64's own exact rational value
//     (big.Rat.SetFloat64(f)) -> float64. NUMERIC equality, not a lexical
//     re-formatting comparison: the prior FormatFloat-string-equality check
//     rejected exact values whose canonical 'g' formatting merely looked
//     different from the source digits (e.g. "1.0" or "1e3" reformat to "1"
//     / "1000"), which is a formatting mismatch, not a precision loss. A
//     decimal fraction float64 cannot hold exactly (0.1, most non-power-of-
//     two fractions) still falls through to case 4, same as before.
//  4. Else left as json.Number: a bare integer with more significant digits
//     than int64/uint64 can hold (regardless of whether that particular
//     value happens to be float64-exact, e.g. 2^64 — promoting only the
//     rare power-of-two exception would make same-shaped big integers
//     render inconsistently), or a decimal/scientific value float64 cannot
//     reproduce exactly. yaml.v3 renders this one as a quoted scalar; it is
//     the only quoted case, and it is exactly the value UseNumber exists to
//     protect from silent precision loss.
//
// map[string]any and []any are walked in place; every other type (string,
// bool, nil, already-native numerics) passes through unchanged.
func NormalizeJSONNumbers(v any) any {
	switch t := v.(type) {
	case json.Number:
		s := string(t)
		if i, err := t.Int64(); err == nil {
			return i
		}
		if u, err := strconv.ParseUint(s, 10, 64); err == nil {
			return u
		}
		// A plain integer token (no '.', no exponent marker) that reaches
		// here already overflowed both Int64 and ParseUint above — per JSON
		// grammar, the absence of '.'/'e'/'E' means s IS a bare integer, so
		// this is tier 4 material regardless of float64 exactness. Skipping
		// the float attempt for this shape preserves the pre-existing
		// contract that any integer beyond uint64 range survives as
		// json.Number uniformly — including the rare case where the value
		// happens to be a power of two and would otherwise round-trip
		// exactly (e.g. 2^64), which would make otherwise-identical-looking
		// big integers render inconsistently (sometimes plain, sometimes
		// scientific notation) depending on an implementation accident.
		if !strings.ContainsAny(s, ".eE") {
			return t
		}
		if f, err := t.Float64(); err == nil {
			r1, ok1 := new(big.Rat).SetString(s)
			r2 := new(big.Rat).SetFloat64(f)
			if ok1 && r2 != nil && r1.Cmp(r2) == 0 {
				return f
			}
		}
		return t
	case map[string]any:
		for k, val := range t {
			t[k] = NormalizeJSONNumbers(val)
		}
		return t
	case []any:
		for i, val := range t {
			t[i] = NormalizeJSONNumbers(val)
		}
		return t
	default:
		return v
	}
}

// CompactValue renders an arbitrary value as a compact, single-line string.
// nil renders as "". Strings pass through bare (not JSON-quoted). Bools and
// numbers render in their bare form. Maps and slices render as compact JSON
// via json.Marshal, so map keys sort alphabetically. If json.Marshal errors,
// it falls back to fmt.Sprintf("%v", v).
func CompactValue(v any) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	b, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprintf("%v", v)
	}
	return string(b)
}
