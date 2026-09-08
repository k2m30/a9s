// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// tui5_wave2_sentinel_test.go — the provenance the detail controller stamps
// on a Wave-2 finding that reached it without one.
//
// It lives in the package because the Source is internal state: no rendered
// surface shows it, and the readers that branch on it
// (domain.Finding.IsWave2Sourced, primaryWave2Finding, stripWave2) read it
// straight off the finding.
package app

import (
	"strings"
	"testing"

	_ "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime"
	"github.com/k2m30/a9s/v3/core/session"
)

// TestDetailWave2Source_IsASentinelNotAShortName asserts the stamp is
// recognisable as Wave-2 and unreadable as a registered type's short name —
// every other "wave2:<x>" in the codebase is one, so a sentinel shaped like
// one invites a reader to resolve it.
func TestDetailWave2Source_IsASentinelNotAShortName(t *testing.T) {
	if !strings.HasPrefix(wave2SourceUnattributed, "wave2:") {
		t.Errorf("sentinel = %q, want a wave2-prefixed provenance", wave2SourceUnattributed)
	}
	short := strings.TrimPrefix(wave2SourceUnattributed, "wave2:")
	if resource.FindResourceType(short) != nil {
		t.Errorf("sentinel = %q reads as registered type %q", wave2SourceUnattributed, short)
	}
	if !strings.ContainsAny(short, "()") {
		t.Errorf("sentinel = %q — %q is spelled like a short name and must not be",
			wave2SourceUnattributed, short)
	}
	if !(domain.Finding{Source: wave2SourceUnattributed}).IsWave2Sourced() {
		t.Errorf("sentinel = %q is not recognised as Wave-2 sourced", wave2SourceUnattributed)
	}
}

// TestDetailWave2Source_StampedOnAFindingWithoutProvenance drives the real
// controller path: a finding delivered with no Source at all comes out of
// applyDetailFindingsForResource carrying the sentinel.
func TestDetailWave2Source_StampedOnAFindingWithoutProvenance(t *testing.T) {
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
	res := resource.Resource{ID: "i-0abc123def4567890", Name: "acme-web", Type: "ec2"}

	s := session.New()
	s.Profile = "test-profile"
	s.Region = "us-east-1"
	c := New(runtime.New(s, nil))
	t.Cleanup(c.Close)
	c.ApplyIntents([]runtime.UIIntent{runtime.PushScreen{ID: runtime.ScreenDetail}})
	c.EnsureDetailState(res, "ec2")
	c.ApplyDetailFindingForResource("ec2", res.ID, &domain.Finding{
		Code:     domain.FindingCode("ec2.tui5-probe"),
		Phrase:   "a finding delivered without provenance",
		Severity: domain.SevWarn,
	}, nil)

	ds := c.topDetailState()
	if ds == nil {
		t.Fatal("no detail state")
	}
	var got string
	for _, f := range ds.Findings {
		if f.Code == domain.FindingCode("ec2.tui5-probe") {
			got = f.Source
		}
	}
	if got != wave2SourceUnattributed {
		t.Errorf("Source = %q, want the sentinel %q", got, wave2SourceUnattributed)
	}
}
