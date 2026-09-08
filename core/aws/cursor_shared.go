// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package aws

import (
	"encoding/json"
	"io"
	"strings"
)

// strictDecodeCursor decodes token into T, the shape every compound
// continuation-cursor decoder in this package needs: DisallowUnknownFields,
// whole-input validation (a second Decode call must hit io.EOF, so trailing
// bytes after an otherwise-valid JSON value are rejected rather than
// silently ignored the way a single json.Unmarshal or json.Decoder.Decode
// call would accept them), and the zero-value guard — encode() never
// itself produces the zero value, since a token is only ever encoded when
// there is real state to resume from, so a successful decode that lands on
// the zero value has no legitimate origin other than a foreign or
// corrupted cursor.
//
// decodeErr isolates the underlying json.Decode failure (bad syntax, wrong
// shape, an unknown field) from the trailing-bytes/zero-value checks, since
// some callers treat the two differently — e.g. a decoder that falls back
// to interpreting a non-JSON token as some other pre-existing external
// format only for a genuine decodeErr, not for a well-formed decode that
// turned out to be malformed some other way. malformed is true whenever
// either the decode itself failed or the well-formedness checks did; token
// == "" (the "first page" case) is not handled here since which tokens
// legitimately mean "first page" is not uniform across callers.
func strictDecodeCursor[T comparable](token string) (cur T, decodeErr error, malformed bool) {
	dec := json.NewDecoder(strings.NewReader(token))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&cur); err != nil {
		var zero T
		return zero, err, true
	}
	var zero T
	var trailing json.RawMessage
	if cur == zero || dec.Decode(&trailing) != io.EOF {
		return zero, nil, true
	}
	return cur, nil, false
}
