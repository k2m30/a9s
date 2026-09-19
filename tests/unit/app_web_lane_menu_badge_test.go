package unit_test

import (
	"testing"

	"github.com/k2m30/a9s/v3/core/app"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/runtime"
	"github.com/k2m30/a9s/v3/core/session"
)

func newWebLaneMenuBadgeController(t *testing.T) *app.Controller {
	t.Helper()
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
	s := session.New()
	s.Profile = "demo"
	s.Region = "us-east-1"
	core := runtime.New(s, nil)
	c := newBlessedController(t, core)
	t.Cleanup(c.Close)
	return c
}

// s3PatchFindings builds n wave2-sourced findings for distinct fake S3
// bucket IDs, matching the shape carried by runtime.ListEnrichmentPatch.
func s3PatchFindings(n int) map[string][]domain.Finding {
	findings := make(map[string][]domain.Finding, n)
	for i := range n {
		id := "arn:aws:s3:::a9s-test-bucket-" + string(rune('a'+i))
		findings[id] = []domain.Finding{{
			Code:     "s3.public_access",
			Phrase:   "public access not blocked",
			Severity: domain.SevWarn,
			Source:   "wave2:s3",
		}}
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

// A nil Issues field carries no badge patch; it is not a "0 issues" result.
func TestApplyIntents_PatchResourceList_NilIssues_LeavesMenuBadgeUnknown(t *testing.T) {
	c := newWebLaneMenuBadgeController(t)

	c.ApplyIntents([]runtime.UIIntent{
		runtime.PatchResourceList{
			ResourceType: "ec2",
			Issues:       nil,
			Enrichment: &runtime.ListEnrichmentPatch{
				Findings: map[string][]domain.Finding{},
			},
		},
	})

	if got := c.GetMenuIssueKnown()["ec2"]; got {
		t.Errorf("GetMenuIssueKnown()[ec2] = %v, want false (Issues was nil, badge state must stay unknown)", got)
	}
}
