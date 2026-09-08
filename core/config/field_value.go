// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package config

import (
	"maps"
	"time"
)

// CanonicalFieldValue renders a Fields-map scalar the one way a9s shows it,
// on every surface a Fields value reaches: the list cell, the detail row and
// the text filter the operator types into.
//
// fieldpath.FormatValue applies these conventions to a value read off a
// RawStruct. A Fields value arrives as whatever the fetcher chose to write —
// a bool as "true", a timestamp as RFC3339 — so the same conventions are
// applied to it here rather than in the fetcher, which would leave every
// fetcher free to spell it differently again.
//
// Only the shapes with a settled convention are canonicalised, and none is
// given precision it did not arrive with. Numbers are deliberately left alone:
// a decimal is an engine version at least as often as it is a number, and
// rendering "1.10" as "1.1" is a wrong answer rather than a tidier one. A
// column whose two declarations disagree about a number is fixed in the
// defaults instead.
//
// It applies to the Fields map only. A value read out of a raw document — a
// CloudTrail request-parameter block, a policy JSON, a YAML subtree — is the
// document's own text and is rendered verbatim.
func CanonicalFieldValue(v string) string {
	switch v {
	case "true":
		return "Yes"
	case "false":
		return "No"
	}
	// RFC3339 only. A fetcher that writes a bare date knows the day and not
	// the hour, and "<date> 00:00" is a midnight AWS never reported — the same
	// reason numbers are left alone below. The rendered shape parses as
	// neither, so this is idempotent.
	if t, err := time.Parse(time.RFC3339, v); err == nil {
		return t.Format("2006-01-02 15:04")
	}
	return v
}

// CanonicalFieldValues returns fields with every value in the form
// CanonicalFieldValue renders, or fields itself when nothing needed settling.
// The input map is never written to: a fetcher's own map is shared with the
// cache-save lane and the row store.
func CanonicalFieldValues(fields map[string]string) map[string]string {
	var out map[string]string
	for k, v := range fields {
		c := CanonicalFieldValue(v)
		if c == v {
			continue
		}
		if out == nil {
			out = make(map[string]string, len(fields))
			maps.Copy(out, fields)
		}
		out[k] = c
	}
	if out == nil {
		return fields
	}
	return out
}
