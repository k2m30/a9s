// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package unit_test

// misc4_r7_header_identity_test.go — misc4 row 7.
//
// The header's error arm reserved all but six columns for the message, which
// left the frame renderer nothing to put the left side in: the profile and
// region were truncated away. "Which account am I looking at" outranks the
// text of one failure, and it is the question the operator cannot answer from
// anywhere else on the screen.

import (
	"strings"
	"testing"

	"github.com/k2m30/a9s/v3/core/runtime/messages"
)

// misc4HeaderLine renders the root model with an error flash of the given
// length and returns the header row.
func misc4HeaderLine(t *testing.T, width int, msg string) string {
	t.Helper()
	m := issue119RootModel(width, 30, true)
	m = issue119LoadEC2List(t, m)
	m = issue119ApplyMsg(m, messages.Flash{Text: msg, IsError: true})
	return strings.SplitN(m.View().Content, "\n", 2)[0]
}

// TestHeaderKeepsAccountIdentityUnderALongError pins that a 200-column error
// in a 120-column terminal still leaves profile:region on the header, and
// that the message is cut to what remains rather than the other way round.
func TestHeaderKeepsAccountIdentityUnderALongError(t *testing.T) {
	const width = 120
	long := "Error: " + strings.Repeat("x", 200)
	header := misc4HeaderLine(t, width, long)

	if !strings.Contains(header, "demo:us-east-1") {
		t.Errorf("a long error took the account identity off the header:\n%s", header)
	}
	if !strings.Contains(header, "Error: xxx") {
		t.Errorf("the error message is not on the header at all:\n%s", header)
	}
	if strings.Contains(header, strings.Repeat("x", 120)) {
		t.Errorf("the error message was not cut to the remaining slot:\n%s", header)
	}
}

// TestHeaderShowsAShortErrorWhole pins that capping the error slot did not
// start truncating messages that always fitted.
func TestHeaderShowsAShortErrorWhole(t *testing.T) {
	header := misc4HeaderLine(t, 120, "Error: load failed")
	if !strings.Contains(header, "Error: load failed") {
		t.Errorf("a short error is no longer shown whole:\n%s", header)
	}
	if !strings.Contains(header, "demo:us-east-1") {
		t.Errorf("the account identity is missing beside a short error:\n%s", header)
	}
}
