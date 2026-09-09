// SPDX-License-Identifier: GPL-3.0-or-later

package domain

import (
	"maps"
	"reflect"
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
// s, whose introducer is the rune r of size bytes.
//
// It parses what a terminal parses, because anything it does not consume is
// left on the screen as text nobody wrote. A CSI runs over its parameter and
// intermediate bytes to a final byte in 0x40-0x7e — a BEL inside it is just
// another byte, not a terminator. A string control (OSC, DCS, SOS, PM, APC)
// runs to BEL or ST, scanned rune by rune so that the 0x9c byte inside a
// multi-byte rune cannot be read as ST. An ESC followed by intermediates runs
// to its own final byte (ESC ( B selects a character set, and stopping after
// the "(" would leave a capital B on screen). An ESC followed by anything
// else consumes the ESC alone, which keeps a multi-byte rune after it whole
// rather than splitting it into an invalid tail.
//
// An unterminated sequence is the whole rest of the string: it would have
// swallowed everything after it on a real terminal too.
func sequenceLen(s string, r rune, size int) int {
	kind := byte(0)
	switch r {
	case 0x1b:
		if len(s) < 2 {
			return len(s)
		}
		next := s[1]
		switch {
		case next == '[', next == ']', next == 'P', next == 'X', next == '^', next == '_':
			kind = next
			size = 2
		case next >= 0x20 && next <= 0x2f: // intermediate bytes, then a final
			for i := 1; i < len(s); i++ {
				if s[i] >= 0x30 && s[i] <= 0x7e {
					return i + 1
				}
				if s[i] < 0x20 || s[i] > 0x2f {
					return i
				}
			}
			return len(s)
		case next >= 0x30 && next <= 0x7e: // a two-character escape
			return 2
		default:
			// A control byte, or the first byte of a rune. The ESC goes, the
			// rune stays.
			return 1
		}
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
	if kind == '[' {
		for i := size; i < len(s); i++ {
			if s[i] >= 0x40 && s[i] <= 0x7e {
				return i + 1
			}
		}
		return len(s)
	}
	// A string control: BEL or ST ends it, and the payload is scanned on rune
	// boundaries.
	for i := size; i < len(s); {
		c, n := utf8.DecodeRuneInString(s[i:])
		switch {
		case c == 0x07:
			return i + 1
		case c == 0x9c:
			return i + n
		case c == utf8.RuneError && n == 1 && s[i] == 0x9c:
			return i + 1
		case c == 0x1b && i+1 < len(s) && s[i+1] == '\\':
			return i + 2
		}
		i += n
	}
	return len(s)
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
// RawStruct is cleaned too, string leaf by string leaf. It is not a
// presentation copy of Fields: a configured Path column and the generic detail
// projection read it directly, so an alarm description that never reaches
// Fields still reaches a painter. Cleaning it here is what lets every reader
// downstream stay a reader.
//
// ID is the one string deliberately left raw. It is what the next AWS call and
// the clipboard need byte for byte, so the places that PAINT an ID ask for its
// displayed form instead (resource.DisplayID).
func (r Resource) Sanitized() Resource {
	if !r.NeedsSanitizing() {
		return r
	}
	r.Name = Sanitize(r.Name)
	r.Fields = sanitizedFields(r.Fields)
	r.Findings = SanitizedFindings(r.Findings)
	r.AttentionDetails = SanitizedAttentionDetails(r.AttentionDetails)
	r.RawStruct = SanitizedRawStruct(r.RawStruct)
	return r
}

// NeedsSanitizing reports whether Sanitized would change r. It is the scan
// half of the boundary on its own, for the caller that has to know whether
// anything moved — the row store returns the caller's own page when nothing
// did, rather than paying a deep copy per page to hand back rows identical to
// the ones it was given.
func (r Resource) NeedsSanitizing() bool {
	if Sanitize(r.Name) != r.Name {
		return true
	}
	for k, v := range r.Fields {
		if Sanitize(k) != k || Sanitize(v) != v {
			return true
		}
	}
	for _, f := range r.Findings {
		if Sanitize(f.Phrase) != f.Phrase || Sanitize(f.Detail) != f.Detail {
			return true
		}
	}
	for _, ad := range r.AttentionDetails {
		for _, row := range ad.Rows {
			if Sanitize(row.Label) != row.Label || Sanitize(row.Value) != row.Value {
				return true
			}
		}
	}
	return r.RawStruct != nil && hasHostileString(reflect.ValueOf(r.RawStruct), 0)
}

// maxRawStructDepth bounds the walk over an SDK struct. The AWS types are
// trees a handful of levels deep; the bound is what keeps a type that ever
// grows a cycle from hanging a fetch.
const maxRawStructDepth = 32

// SanitizedRawStruct returns v with every string leaf inert, as a deep copy
// when one of them changed and v itself when none did — which is the case for
// every well-behaved resource, and the reason this costs a walk and not an
// allocation. The copy matters: the value is the fetcher's, and the row store
// hands the same pointer to whoever asks.
func SanitizedRawStruct(v any) any {
	if v == nil {
		return nil
	}
	src := reflect.ValueOf(v)
	if !hasHostileString(src, 0) {
		return v
	}
	dst := reflect.New(src.Type()).Elem()
	copyClean(dst, src, 0)
	return dst.Interface()
}

// hasHostileString reports whether any string reachable from v would change.
func hasHostileString(v reflect.Value, depth int) bool {
	if depth > maxRawStructDepth || !v.IsValid() {
		return false
	}
	switch v.Kind() {
	case reflect.String:
		return Sanitize(v.String()) != v.String()
	case reflect.Pointer, reflect.Interface:
		return !v.IsNil() && hasHostileString(v.Elem(), depth+1)
	case reflect.Struct:
		for i := range v.NumField() {
			if v.Type().Field(i).IsExported() && hasHostileString(v.Field(i), depth+1) {
				return true
			}
		}
	case reflect.Slice, reflect.Array:
		for i := range v.Len() {
			if hasHostileString(v.Index(i), depth+1) {
				return true
			}
		}
	case reflect.Map:
		for _, key := range v.MapKeys() {
			if hasHostileString(key, depth+1) || hasHostileString(v.MapIndex(key), depth+1) {
				return true
			}
		}
	}
	return false
}

// copyClean writes src into the addressable dst with every string leaf
// sanitised. An unexported field is copied as it stands by the struct
// assignment below and never visited: the AWS types carry only serde markers
// there, and reflect could not write to it anyway.
func copyClean(dst, src reflect.Value, depth int) {
	if depth > maxRawStructDepth || !src.IsValid() {
		return
	}
	switch src.Kind() {
	case reflect.String:
		dst.SetString(Sanitize(src.String()))
	case reflect.Pointer:
		if src.IsNil() {
			return
		}
		dst.Set(reflect.New(src.Type().Elem()))
		copyClean(dst.Elem(), src.Elem(), depth+1)
	case reflect.Interface:
		if src.IsNil() {
			return
		}
		inner := reflect.New(src.Elem().Type()).Elem()
		copyClean(inner, src.Elem(), depth+1)
		dst.Set(inner)
	case reflect.Struct:
		dst.Set(src)
		for i := range src.NumField() {
			if src.Type().Field(i).IsExported() {
				copyClean(dst.Field(i), src.Field(i), depth+1)
			}
		}
	case reflect.Slice:
		if src.IsNil() {
			return
		}
		dst.Set(reflect.MakeSlice(src.Type(), src.Len(), src.Len()))
		for i := range src.Len() {
			copyClean(dst.Index(i), src.Index(i), depth+1)
		}
	case reflect.Array:
		for i := range src.Len() {
			copyClean(dst.Index(i), src.Index(i), depth+1)
		}
	case reflect.Map:
		if src.IsNil() {
			return
		}
		dst.Set(reflect.MakeMapWithSize(src.Type(), src.Len()))
		for _, key := range src.MapKeys() {
			cleanKey := reflect.New(key.Type()).Elem()
			copyClean(cleanKey, key, depth+1)
			cleanVal := reflect.New(src.Type().Elem()).Elem()
			copyClean(cleanVal, src.MapIndex(key), depth+1)
			dst.SetMapIndex(cleanKey, cleanVal)
		}
	default:
		dst.Set(src)
	}
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
