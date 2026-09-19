// The web lane runs no background availability/issue sweep, so
// Controller.ApplyEnrichmentState (core/app/list_filter.go) is where the menu
// badge learns an in-session Wave-2 result: it sets the menu issue count,
// marks it Known, and clears a truncated flag once an equal-count exact
// observation lands.
package unit_test

import (
	"testing"

	"github.com/k2m30/a9s/v3/core/app"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
	"github.com/k2m30/a9s/v3/core/session"
)

// newEnrichmentMenuBadgeController builds a Controller + its backing Core.
func newEnrichmentMenuBadgeController(t *testing.T) *app.Controller {
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

// s3EnrichmentFindings builds n wave2-sourced findings for distinct fake S3
// bucket IDs, matching the shape ResourceListModel.SetEnrichmentState passes
// in production (internal/tui/views/resourcelist.go).
func s3EnrichmentFindings(n int) map[string][]domain.Finding {
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

// menuEntryFor returns the MenuEntry for shortName from a MenuBody, or nil.
func menuEntryFor(mb *app.MenuBody, shortName string) *app.MenuEntry {
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

func TestApplyEnrichmentState_SyncsMenuIssueBadge_S3(t *testing.T) {
	c := newEnrichmentMenuBadgeController(t)

	findings := s3EnrichmentFindings(5)
	details := map[string]map[domain.FindingCode]domain.AttentionDetail{}
	c.ApplyEnrichmentState("s3", 5, true, findings, details)

	if got := c.GetMenuIssueCounts()["s3"]; got != 5 {
		t.Errorf("GetMenuIssueCounts()[s3] = %d, want 5", got)
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
	entry := menuEntryFor(snap.Body.Menu, "s3")
	if entry == nil {
		t.Fatal("menu has no entry for s3")
	}
	if entry.IssueBadge.Count != 5 {
		t.Errorf("s3 IssueBadge.Count = %d, want 5 — this is the missing \"! 5+\" badge", entry.IssueBadge.Count)
	}
	if !entry.IssueBadge.Truncated {
		t.Error("s3 IssueBadge.Truncated = false, want true")
	}
}

// A Wave-2 result IS the type's issue count as of now, so it lowers the badge
// as readily as it raises it; otherwise five cached issues healed and
// re-verified would keep showing, and keep being persisted, forever.
// Monotonicity applies only to the rows-derived lane, which cannot prove an
// issue gone; that half is pinned by
// TestApplyEnrichmentState_MenuBadge_RowsDerivedResultNeverLowersCount below.
func TestApplyEnrichmentState_MenuBadge_AuthoritativeResultLowersCount(t *testing.T) {
	c := newEnrichmentMenuBadgeController(t)

	c.ApplyIntents([]runtime.UIIntent{
		runtime.PatchMenu{ResourceType: "s3", Issues: 7, Truncated: false},
	})
	if got := c.GetMenuIssueCounts()["s3"]; got != 7 {
		t.Fatalf("precondition failed: GetMenuIssueCounts()[s3] = %d, want 7", got)
	}

	c.ApplyEnrichmentState("s3", 5, true, s3EnrichmentFindings(5), nil)

	if got := c.GetMenuIssueCounts()["s3"]; got != 5 {
		t.Errorf("GetMenuIssueCounts()[s3] = %d, want 5 — a fresh Wave-2 result assigns the badge, it does not only raise it", got)
	}
}

// A non-authoritative observation — a number from bare list rows, where Wave-2
// may not have run — only raises the badge: it cannot prove an issue gone.
func TestApplyEnrichmentState_MenuBadge_RowsDerivedResultNeverLowersCount(t *testing.T) {
	c := newEnrichmentMenuBadgeController(t)

	c.ApplyIntents([]runtime.UIIntent{
		runtime.PatchMenu{ResourceType: "s3", Issues: 7, Truncated: false},
	})

	// A PatchResourceList carrying no issue result at all: the controller
	// stands in 0/false for it, non-authoritatively.
	c.ApplyIntents([]runtime.UIIntent{
		runtime.PatchResourceList{ResourceType: "s3", Enrichment: &runtime.ListEnrichmentPatch{}},
	})

	if got := c.GetMenuIssueCounts()["s3"]; got != 7 {
		t.Errorf("GetMenuIssueCounts()[s3] = %d, want 7 — an observation that carries no Wave-2 result must not lower the badge", got)
	}
}

func TestApplyEnrichmentState_MenuBadge_ClearsTruncationAtEqualCount(t *testing.T) {
	c := newEnrichmentMenuBadgeController(t)

	c.ApplyIntents([]runtime.UIIntent{
		runtime.PatchMenu{ResourceType: "s3", Issues: 5, Truncated: true},
	})
	if got := c.GetMenuIssueTruncated()["s3"]; !got {
		t.Fatalf("precondition failed: GetMenuIssueTruncated()[s3] = %v, want true", got)
	}

	c.ApplyEnrichmentState("s3", 5, false, s3EnrichmentFindings(5), nil)

	if got := c.GetMenuIssueCounts()["s3"]; got != 5 {
		t.Errorf("GetMenuIssueCounts()[s3] = %d, want 5", got)
	}
	if got := c.GetMenuIssueTruncated()["s3"]; got {
		t.Errorf("GetMenuIssueTruncated()[s3] = %v, want false (equal-count exact result must clear a stale truncation flag)", got)
	}
}

// Production passes the canonical ShortName (ResourceListModel passes
// m.typeDef.ShortName), and the menu-sync chokepoint also resolves an alias to
// the canonical key, as handleResourcesLoadedEvent (core/app/handle.go) and
// menuActiveKey (core/app/menu.go) do. "workgroups" is a registered alias of
// "athena" (core/aws/catalog_data.go).
func TestApplyEnrichmentState_MenuBadge_CanonicalizesAlias(t *testing.T) {
	c := newEnrichmentMenuBadgeController(t)

	findings := map[string][]domain.Finding{
		"workgroup-1": {{Code: "athena.stale_config", Phrase: "stale workgroup config", Severity: domain.SevWarn, Source: "wave2:athena"}},
	}
	c.ApplyEnrichmentState("workgroups", 1, false, findings, nil)

	if got := c.GetMenuIssueCounts()["athena"]; got != 1 {
		t.Errorf("GetMenuIssueCounts()[athena] = %d, want 1 — alias %q must canonicalize to \"athena\"", got, "workgroups")
	}
	if got := c.GetMenuIssueCounts()["workgroups"]; got != 0 {
		t.Errorf("GetMenuIssueCounts()[workgroups] = %d, want 0 — alias key must not be stored verbatim alongside the canonical key", got)
	}
}

// A type whose Wave-2 enrichment reports zero issues becomes IssueKnown:
// syncMenuIssueCount's newIssues > curIssues guard never fires on 0-vs-0, so the
// authoritative flag is what sets Known.
func TestApplyEnrichmentState_ZeroIssues_StillBecomesKnown(t *testing.T) {
	c := newEnrichmentMenuBadgeController(t)

	c.ApplyEnrichmentState("s3", 0, false, map[string][]domain.Finding{}, map[string]map[domain.FindingCode]domain.AttentionDetail{})

	if got := c.GetMenuIssueKnown()["s3"]; !got {
		t.Errorf("GetMenuIssueKnown()[s3] = %v, want true — a confirmed Wave-2 zero-issue result must still flip Known", got)
	}
	if got := c.GetMenuIssueCounts()["s3"]; got != 0 {
		t.Errorf("GetMenuIssueCounts()[s3] = %d, want 0", got)
	}
	entry := menuEntryFor(c.Snapshot().Body.Menu, "s3")
	if entry == nil {
		t.Fatal("menu has no entry for s3")
	}
	if entry.IssueBadge.Count != 0 {
		t.Errorf("s3 IssueBadge.Count = %d, want 0 — a confirmed-clean type must show no badge count", entry.IssueBadge.Count)
	}
}

// IssueKnown stays unset when rows with zero findings load through the
// rows-derived lane (syncExactTotalToMenu, reached via ResourcesLoaded): Wave-2
// may not have run for that type. ApplyResourcesLoaded bypasses this
// sync-back, so the test drives Controller.Handle.
func TestSyncExactTotalToMenu_RowsWithoutFindings_DoesNotSetIssueKnown(t *testing.T) {
	c := newEnrichmentMenuBadgeController(t)

	c.Apply(app.Action{Kind: app.ActionCommand, Arg: "s3"})

	rows := make([]resource.Resource, 3)
	for i := range rows {
		rows[i] = resource.Resource{ID: "bucket-" + itoaTest(i), Type: "s3"}
	}
	handlePage(c, messages.ResourcesLoaded{Provenance: messages.FetchProvenanceCanonicalList,
		ResourceType: "s3",
		Resources:    rows,
		Pagination:   &resource.PaginationMeta{IsTruncated: false},
		Gen:          0,
	})

	if got := c.GetMenuIssueKnown()["s3"]; got {
		t.Error("GetMenuIssueKnown()[s3] = true, want false — bare list rows with no Wave-2 findings must not be treated as a confirmed issue result")
	}
}
