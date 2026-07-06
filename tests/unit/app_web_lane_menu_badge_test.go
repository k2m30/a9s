// app_web_lane_menu_badge_test.go — RED pin for the web/headless lane's
// menu-badge sync through the runtime.PatchResourceList intent.
//
// Defect: Controller.applyIntents' runtime.PatchResourceList case
// (internal/app/intents.go) always calls
// c.applyEnrichmentState(v.ResourceType, 0, false, v.Enrichment.Findings, ...)
// — the issueCount and truncated arguments are hardcoded to 0/false,
// discarding v.Issues (*runtime.IssueBadgePatch) carried in the very same
// intent. Controller.applyEnrichmentState itself correctly raises the menu
// issue badge via syncMenuIssueCount when given a real issueCount/truncated
// (pinned directly in app_enrichment_menu_badge_test.go) — but the
// PatchResourceList intent, which is how the web/headless lane actually
// delivers Wave-2 results (via runtime.HandleEvent -> ApplyIntents), never
// passes the real numbers through. A live browser session flagged 5 s3 rows
// but the s3 menu entry showed no "! 5+" badge.
package unit_test

import (
	"testing"

	"github.com/k2m30/a9s/v3/internal/app"
	"github.com/k2m30/a9s/v3/internal/domain"
	"github.com/k2m30/a9s/v3/internal/runtime"
	"github.com/k2m30/a9s/v3/internal/session"
)

// newWebLaneMenuBadgeController mirrors newEnrichmentMenuBadgeController in
// app_enrichment_menu_badge_test.go — duplicated locally per that file's own
// stated precedent (no cross-file coupling to another test file's helper
// lifetime).
func newWebLaneMenuBadgeController(t *testing.T) *app.Controller {
	t.Helper()
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
	s := session.New()
	s.Profile = "demo"
	s.Region = "us-east-1"
	core := runtime.New(s, nil)
	c := app.New(core)
	t.Cleanup(c.Close)
	return c
}

// s3PatchFindings builds n wave2-sourced findings for distinct fake S3
// bucket IDs, matching the shape carried by runtime.ListEnrichmentPatch.
func s3PatchFindings(n int) map[string]domain.Finding {
	findings := make(map[string]domain.Finding, n)
	for i := range n {
		id := "arn:aws:s3:::a9s-test-bucket-" + string(rune('a'+i))
		findings[id] = domain.Finding{
			Code:     "s3.public_access",
			Phrase:   "public access not blocked",
			Severity: domain.SevWarn,
			Source:   "wave2:s3",
		}
	}
	return findings
}

// webLaneMenuEntryFor returns the MenuEntry for shortName from a MenuBody, or nil.
func webLaneMenuEntryFor(mb *app.MenuBody, shortName string) *app.MenuEntry {
	if mb == nil {
		return nil
	}
	for i := range mb.Entries {
		if mb.Entries[i].ShortName == shortName {
			return &mb.Entries[i]
		}
	}
	return nil
}

// TestApplyIntents_PatchResourceList_SyncsMenuIssueBadge_S3 pins the web-lane
// defect: a PatchResourceList intent carrying both Enrichment findings and a
// non-nil Issues badge patch (Count: 4, Truncated: true) for "s3" must raise
// the s3 menu issue badge to 4/Known/Truncated. Today the Issues field is
// discarded and the hardcoded 0/false wins, so the badge never appears.
func TestApplyIntents_PatchResourceList_SyncsMenuIssueBadge_S3(t *testing.T) {
	c := newWebLaneMenuBadgeController(t)

	findings := s3PatchFindings(4)
	c.ApplyIntents([]runtime.UIIntent{
		runtime.PatchResourceList{
			ResourceType: "s3",
			Issues:       &runtime.IssueBadgePatch{Count: 4, Truncated: true},
			Enrichment: &runtime.ListEnrichmentPatch{
				Findings: findings,
			},
		},
	})

	if got := c.GetMenuIssueCounts()["s3"]; got != 4 {
		t.Errorf("GetMenuIssueCounts()[s3] = %d, want 4", got)
	}
	if got := c.GetMenuIssueKnown()["s3"]; !got {
		t.Errorf("GetMenuIssueKnown()[s3] = %v, want true", got)
	}
	if got := c.GetMenuIssueTruncated()["s3"]; !got {
		t.Errorf("GetMenuIssueTruncated()[s3] = %v, want true", got)
	}

	snap := c.Snapshot()
	if snap.Body.Kind != app.BodyKindMenu {
		t.Fatalf("Body.Kind = %q, want %q", snap.Body.Kind, app.BodyKindMenu)
	}
	entry := webLaneMenuEntryFor(snap.Body.Menu, "s3")
	if entry == nil {
		t.Fatal("menu has no entry for s3")
	}
	if entry.IssueBadge.Count != 4 {
		t.Errorf("s3 IssueBadge.Count = %d, want 4 — this is the missing \"! 4+\" badge seen live (5 flagged rows, no badge)", entry.IssueBadge.Count)
	}
	if !entry.IssueBadge.Truncated {
		t.Error("s3 IssueBadge.Truncated = false, want true")
	}
}

// TestApplyIntents_PatchResourceList_NilIssues_LeavesMenuBadgeUnknown pins
// the companion no-op contract: when Issues is nil (no badge patch carried
// in this particular intent), the menu issue count for the type must remain
// unset/unknown — a nil Issues field must not itself be mistaken for a
// "0 issues" result.
func TestApplyIntents_PatchResourceList_NilIssues_LeavesMenuBadgeUnknown(t *testing.T) {
	c := newWebLaneMenuBadgeController(t)

	c.ApplyIntents([]runtime.UIIntent{
		runtime.PatchResourceList{
			ResourceType: "ec2",
			Issues:       nil,
			Enrichment: &runtime.ListEnrichmentPatch{
				Findings: map[string]domain.Finding{},
			},
		},
	})

	if got := c.GetMenuIssueKnown()["ec2"]; got {
		t.Errorf("GetMenuIssueKnown()[ec2] = %v, want false (Issues was nil, badge state must stay unknown)", got)
	}
}
