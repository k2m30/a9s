// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package domain

import "strings"

// HumanizeStatusPhrase rewrites a raw AWS status/lifecycle enum into a
// lowercase, operator-readable phrase for display in the status-cell
// fallback and in AttentionDetail rows. It is the single conversion point
// for the two raw-enum shapes AWS actually returns:
//
//   - UPPER_SNAKE_CASE (e.g. "CREATE_FAILED", "ACTIVE") — split on "_" and
//     lowercase each word.
//   - CamelCase (e.g. "PendingConfirmation") — split before each interior
//     uppercase letter and lowercase each word.
//
// A value that is already lowercase, or already contains a space (mixed
// prose composed elsewhere, e.g. "unhealthy targets: 2/5"), passes through
// unchanged — humanizing it again would be a no-op at best and would
// corrupt punctuation/identifiers embedded in prose at worst.
//
// This function must never be applied to resource identifiers (names,
// ARNs, IDs) — callers are responsible for routing only status/lifecycle
// values through it.
func HumanizeStatusPhrase(s string) string {
	if s == "" {
		return s
	}
	if s == strings.ToLower(s) {
		return s
	}
	if strings.Contains(s, " ") {
		return s
	}
	if strings.Contains(s, "_") {
		return humanizeSnakeCase(s)
	}
	return humanizeCamelCase(s)
}

// humanizeSnakeCase converts "CREATE_FAILED" to "create failed".
func humanizeSnakeCase(s string) string {
	parts := strings.Split(s, "_")
	for i, p := range parts {
		parts[i] = strings.ToLower(p)
	}
	return strings.Join(parts, " ")
}

// humanizeCamelCase converts "PendingConfirmation" to "pending confirmation".
func humanizeCamelCase(s string) string {
	var b strings.Builder
	runes := []rune(s)
	for i, r := range runes {
		if i > 0 && isUpperRune(r) && !isUpperRune(runes[i-1]) {
			b.WriteByte(' ')
		}
		b.WriteRune(r)
	}
	return strings.ToLower(b.String())
}

// isUpperRune reports whether r is an uppercase ASCII letter. AWS enum
// values are ASCII-only, so a full unicode.IsUpper is unnecessary here.
func isUpperRune(r rune) bool {
	return r >= 'A' && r <= 'Z'
}

// NotApplicable is what a cell says when the fact its column names does not
// exist for this row — an S3 prefix has no size, no last-modified instant and
// no storage class, because it is not an object.
//
// It is a written value, not an empty string, for two reasons. A blank cell is
// indistinguishable from a value nobody managed to fetch, which is the
// confident-zero shape a9s exists to disprove. And a row that omits the key
// altogether renders cells no formatter ever saw, so the key set of two rows
// the same columns render has to match.
//
// The hyphen sorts before every digit, so a listing ordered by a numeric
// column keeps the not-applicable rows at the ascending end rather than
// scattering them.
const NotApplicable = "-"
