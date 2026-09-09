// SPDX-License-Identifier: GPL-3.0-or-later

package domain

import (
	"maps"
	"slices"
	"strings"
	"unicode/utf8"
)

// Sanitize returns s with every terminal control sequence replaced by a single
// space: an ESC and the sequence it introduces, and every remaining C0, DEL or
// C1 byte.
//
// AWS hands back whatever was put in: a tag value, a description, a user agent
// and a name are all written by whoever can tag the resource. Painted raw,
// an ESC in one of them is a command to the operator's terminal — it recolours
// the rest of the screen, and it costs the cell columns the width measure
// never counted. The escape cannot be removed downstream, because by then it
// is indistinguishable from the styling a9s itself writes.
//
// A space, rather than nothing, is what the removed byte leaves behind: a
// control character has no width but the words on either side of it were not
// one word, and the value has to stay something the operator can type into a
// filter and match. It is the substitution the painter already makes for the
// C0 bytes it meets.
func Sanitize(s string) string {
	if !needsSanitizing(s) {
		return s
	}
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); {
		r, size := utf8.DecodeRuneInString(s[i:])
		if r == utf8.RuneError && size == 1 && isControlRune(rune(s[i])) {
			// A C1 control as the single 8-bit byte a terminal in that mode
			// obeys, which is not valid UTF-8 and so decodes as no rune at all.
			r = rune(s[i])
		}
		switch {
		case r == 0x1b || isC1Introducer(r):
			b.WriteByte(' ')
			i += sequenceLen(s[i:], r, size)
		case isControlRune(r):
			b.WriteByte(' ')
			i += size
		default:
			b.WriteString(s[i : i+size])
			i += size
		}
	}
	return b.String()
}

// needsSanitizing reports whether s holds anything Sanitize would change: a
// control rune, or a C1 control written as the raw 8-bit byte that is not
// valid UTF-8 on its own. A continuation byte inside a CJK rune is neither,
// which is why this decodes rather than scanning bytes.
func needsSanitizing(s string) bool {
	for i := 0; i < len(s); {
		r, size := utf8.DecodeRuneInString(s[i:])
		if isControlRune(r) {
			return true
		}
		if r == utf8.RuneError && size == 1 && isControlRune(rune(s[i])) {
			return true
		}
		i += size
	}
	return false
}

// isControlRune reports whether r is a C0 control, DEL, or a C1 control.
func isControlRune(r rune) bool {
	return r < 0x20 || r == 0x7f || (r >= 0x80 && r <= 0x9f)
}

// isC1Introducer reports whether r is a C1 control that opens a sequence in
// its own right: CSI, OSC, DCS, SOS, PM and APC each have a single-character
// C1 spelling beside the two-character ESC one.
func isC1Introducer(r rune) bool {
	switch r {
	case 0x9b, 0x9d, 0x90, 0x98, 0x9e, 0x9f:
		return true
	}
	return false
}

// sequenceLen returns the byte length of the control sequence at the start of
// s, which begins with the introducer r of size bytes. A CSI runs to its final
// byte, a string sequence (OSC, DCS, SOS, PM, APC) to its BEL or ST
// terminator, and anything else is the introducer and the byte after it. An
// unterminated sequence is the whole rest of the string: it would have
// swallowed everything after it on a real terminal too.
//
// The payload goes with the introducer, never only the control byte: "31m"
// left behind as text is the sequence still on the screen, minus the part that
// made it invisible.
func sequenceLen(s string, r rune, size int) int {
	kind := byte(0)
	switch r {
	case 0x1b:
		if len(s) < 2 {
			return len(s)
		}
		kind = s[1]
		size = 2
	case 0x9b:
		kind = '['
	case 0x9d:
		kind = ']'
	case 0x90:
		kind = 'P'
	case 0x98:
		kind = 'X'
	case 0x9e:
		kind = '^'
	case 0x9f:
		kind = '_'
	}
	switch kind {
	case '[': // CSI: parameter and intermediate bytes, then a final byte.
		for i := size; i < len(s); i++ {
			if s[i] >= 0x40 && s[i] <= 0x7e {
				return i + 1
			}
		}
		return len(s)
	case ']', 'P', 'X', '^', '_': // string sequences, terminated by BEL or ST.
		for i := size; i < len(s); i++ {
			if s[i] == 0x07 {
				return i + 1
			}
			if s[i] == 0x1b && i+1 < len(s) && s[i+1] == '\\' {
				return i + 2
			}
			if s[i] == 0x9c {
				return i + 1
			}
			if s[i] == 0xc2 && i+1 < len(s) && s[i+1] == 0x9c {
				return i + 2
			}
		}
		return len(s)
	default:
		return size
	}
}

// Sanitized returns r with every AWS-supplied string that reaches a surface in
// its inert form: the name, every field key and value, and the wording of
// every finding and attention row. This is what the writers of a resource call
// on the way in; nothing downstream of them has to know a value was hostile.
//
// Nothing is written in place. A page of rows is shared — the store keeps it,
// the cache writer copies it on its own goroutine — so a value that needs
// cleaning is replaced by cleaning a copy, and a resource that needs none (all
// of them, nearly always) is returned as it came, at the cost of one scan per
// string and no allocation at all.
//
// RawStruct is left alone: the one surface that shows it marshals it to YAML,
// which escapes a control byte to literal text rather than emitting it.
func (r Resource) Sanitized() Resource {
	r.Name = Sanitize(r.Name)
	r.Fields = sanitizedFields(r.Fields)
	r.Findings = SanitizedFindings(r.Findings)
	r.AttentionDetails = SanitizedAttentionDetails(r.AttentionDetails)
	return r
}

func sanitizedFields(in map[string]string) map[string]string {
	dirty := false
	for k, v := range in {
		if Sanitize(k) != k || Sanitize(v) != v {
			dirty = true
			break
		}
	}
	if !dirty {
		return in
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[Sanitize(k)] = Sanitize(v)
	}
	return out
}

// SanitizedFindings returns fs with the operator-facing wording of every
// finding inert, as a fresh slice when any of it changed and fs itself when
// none did.
func SanitizedFindings(fs []Finding) []Finding {
	dirty := false
	for _, f := range fs {
		if Sanitize(f.Phrase) != f.Phrase || Sanitize(f.Detail) != f.Detail {
			dirty = true
			break
		}
	}
	if !dirty {
		return fs
	}
	out := slices.Clone(fs)
	for i := range out {
		out[i].Phrase = Sanitize(out[i].Phrase)
		out[i].Detail = Sanitize(out[i].Detail)
	}
	return out
}

// SanitizedAttentionDetails returns m with the label and value of every
// attention row inert, as a fresh map when any of it changed and m itself when
// none did.
func SanitizedAttentionDetails(m map[FindingCode]AttentionDetail) map[FindingCode]AttentionDetail {
	dirty := false
	for _, ad := range m {
		for _, row := range ad.Rows {
			if Sanitize(row.Label) != row.Label || Sanitize(row.Value) != row.Value {
				dirty = true
				break
			}
		}
		if dirty {
			break
		}
	}
	if !dirty {
		return m
	}
	out := maps.Clone(m)
	for code, ad := range out {
		ad.Rows = slices.Clone(ad.Rows)
		for i := range ad.Rows {
			ad.Rows[i].Label = Sanitize(ad.Rows[i].Label)
			ad.Rows[i].Value = Sanitize(ad.Rows[i].Value)
		}
		out[code] = ad
	}
	return out
}
