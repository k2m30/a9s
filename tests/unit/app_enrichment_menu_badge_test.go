// app_enrichment_menu_badge_test.go — RED pins for the missing menu-badge
// sync-back on Controller.ApplyEnrichmentState (core/app/list_filter.go).
//
// Defect: visiting the s3 list in the WEB session runs Wave-2 enrichment and
// flags rows, but after Escape back to the menu the s3 row shows no issue
// badge. Root cause: applyEnrichmentState (core/app/list_filter.go)
// receives issueCount/truncated and stores per-resource findings for the
// list, but discards issueCount (`_ = issueCount`) and never touches
// MenuState.IssueCounts/IssueKnown/IssueTruncated. The web lane runs no
// background availability/issue sweep (unlike the TUI's ResourceListModel,
// which separately syncs via the sweep lane), so ApplyEnrichmentState is the
// ONLY chance the menu badge has to learn the in-session Wave-2 result.
//
// The menu-sync semantics pinned here mirror the monotonic guard already
// pinned for the sweep lane in syncExactTotalToMenu (core/app/handle.go):
// only raise the count, set Known once any count is observed, and clear a
// stale truncated flag once an equal-count exact (untruncated) observation
// lands.
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

// newEnrichmentMenuBadgeController builds a Controller + its backing Core,
// mirroring newSeededTestController in app_cache_first_seeding_test.go — a
// small variant duplicated here per that file's own stated precedent (no
// cross-file coupling to another test file's helper lifetime).
func newEnrichmentMenuBadgeController(t *testing.T) *app.Controller {
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

// TestApplyEnrichmentState_SyncsMenuIssueBadge_S3 pins the primary defect:
// a fresh controller with no prior issue info for "s3" must show the
// wave-2-derived issue badge on the s3 menu entry after ApplyEnrichmentState,
// even though no background sweep ever ran (the web/headless lane).
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

// INVERTED (cachegen row 7, the authoritative issue observation assigns): a
// Wave-2 result IS the type's issue count as of now, so it lowers the badge
// as readily as it raises it. The old expectation — 7 surviving a fresh
// Wave-2 answer of 5 — was the defect: five cached issues healed and
// re-verified kept showing, and kept being persisted, forever. Monotonicity
// still applies to the rows-derived lane, which cannot prove an issue gone;
// that half is pinned by
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

// TestApplyEnrichmentState_MenuBadge_RowsDerivedResultNeverLowersCount is the
// other half of cachegen row 7: a NON-authoritative observation — one whose
// number comes from bare list rows, where Wave-2 may not have run — still
// only raises. It cannot prove an issue is gone, so it must not clear a
// badge a real Wave-2 result set.
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

// TestApplyEnrichmentState_MenuBadge_ClearsTruncationAtEqualCount pins the
// truncation-clear rule: a menu issue count seeded as truncated at 5 becomes
// exact once an equal-count, untruncated ApplyEnrichmentState result lands.
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

// TestApplyEnrichmentState_MenuBadge_CanonicalizesAlias pins type-name
// canonicalization: production always calls ApplyEnrichmentState with the
// type's canonical ShortName (ResourceListModel passes m.typeDef.ShortName —
// internal/tui/views/resourcelist.go), but the menu-sync chokepoint must
// still resolve an alias variant to the canonical key, mirroring
// handleResourcesLoadedEvent's "Resolve canonical short name (handles
// aliases like ...)" step (core/app/handle.go) and menuActiveKey's own
// alias-resolution contract (core/app/menu.go). "workgroups" is a real,
// registered alias of the "athena" resource type (core/aws/catalog_data.go).
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

// TestApplyEnrichmentState_ZeroIssues_StillBecomesKnown pins the
// authoritative-zero case: a type whose Wave-2 enrichment reports ZERO
// issues must still flip IssueKnown to true (a genuinely clean type must not
// stay stuck "unknown" forever just because its confirmed result happens to
// be zero). Before the authoritative-flag fix, syncMenuIssueCount's guard
// only fires on newIssues > curIssues, so a fresh 0-vs-0 comparison never
// sets IssueKnown — this is RED at HEAD (Known stays false).
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

// TestSyncExactTotalToMenu_RowsWithoutFindings_DoesNotSetIssueKnown pins the
// other half of the authoritative distinction: the rows-derived lane
// (syncExactTotalToMenu, reached via ResourcesLoaded handling) must NOT set
// IssueKnown just because a list of rows loaded with zero enrichment
// findings — Wave-2 may simply not have run yet for that type. Driven via
// Controller.Handle(messages.ResourcesLoaded{Gen: 0}) (the
// ApplyResourcesLoaded test seam bypasses this sync-back entirely).
func TestSyncExactTotalToMenu_RowsWithoutFindings_DoesNotSetIssueKnown(t *testing.T) {
	c := newEnrichmentMenuBadgeController(t)

	c.Apply(app.Action{Kind: app.ActionCommand, Arg: "s3"})

	rows := make([]resource.Resource, 3)
	for i := range rows {
		rows[i] = resource.Resource{ID: "bucket-" + itoaTest(i), Type: "s3"}
	}
	c.Handle(messages.ResourcesLoaded{Provenance: messages.FetchProvenanceCanonicalList,
		ResourceType: "s3",
		Resources:    rows,
		Pagination:   &resource.PaginationMeta{IsTruncated: false},
		Gen:          0,
	})

	if got := c.GetMenuIssueKnown()["s3"]; got {
		t.Error("GetMenuIssueKnown()[s3] = true, want false — bare list rows with no Wave-2 findings must not be treated as a confirmed issue result")
	}
}
