// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// failure_line_test.go — the prefetch sweep's failure routing and the one
// sentence every failed call gets. Package runtime so the routing predicate
// the sweep and the per-type handler share is driven directly.
package runtime

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"
	"testing"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
)

func regionGapChainErr() error {
	return fmt.Errorf("operation error codeartifact: ListRepositories, request send failed, %w",
		&url.Error{Op: "Post", URL: "https://codeartifact.eu-central-2.amazonaws.com/",
			Err: &net.DNSError{Err: "no such host", Name: "codeartifact.eu-central-2.amazonaws.com", IsNotFound: true}})
}

// TestSoftFailure_RoutesOnTheClass pins spec row 2 for the prefetch sweep: a
// region gap goes to the error log because the class says so, not because a
// second predicate agreed with the class by luck. Rows already on screen are
// the other soft case.
func TestSoftFailure_RoutesOnTheClass(t *testing.T) {
	for _, tc := range []struct {
		name    string
		err     error
		hasRows bool
		want    bool
	}{
		{"region gap, no rows", regionGapChainErr(), false, true},
		{"partial result", errors.New("throttled on 1 of 3 IDs"), true, true},
		{"row-less hard failure", errors.New("connection reset"), false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := softFailure(tc.err, tc.hasRows); got != tc.want {
				t.Errorf("softFailure = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestFailureLine_OneSentence pins spec row 3 at its source: subject, then the
// cause the class supplies, with the region named only when the class is about
// the region.
func TestFailureLine_OneSentence(t *testing.T) {
	line := failureLine("codeartifact", regionGapChainErr(), "eu-central-2")
	if !strings.HasPrefix(line, "codeartifact: ") {
		t.Errorf("failure line %q does not lead with its subject", line)
	}
	if !strings.Contains(line, awsclient.CauseForClass(awsclient.ClassRegionUnavailable)) {
		t.Errorf("failure line %q does not carry the class's phrase", line)
	}
	if !strings.Contains(line, "eu-central-2") {
		t.Errorf("failure line %q does not name the region", line)
	}
	for _, banned := range []string{"operation error", "https://", "no such host"} {
		if strings.Contains(line, banned) {
			t.Errorf("failure line %q carries %q", line, banned)
		}
	}

	plain := failureLine("ec2", errors.New("connection reset"), "eu-central-2")
	if strings.Contains(plain, "eu-central-2") {
		t.Errorf("failure line %q names a region for a failure that is not about the region", plain)
	}
}
