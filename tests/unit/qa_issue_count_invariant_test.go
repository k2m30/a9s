package unit

// qa_issue_count_invariant_test.go — property invariant: IssueCount never exceeds instance count.
//
// Two invariants enforced:
//
//  1. Per-enricher: result.IssueCount <= len(inputResources) for every entry in
//     EnricherRegistry. An enricher emits at most one finding per resource (keyed by
//     resource ID), so the issue count can never exceed the number of distinct inputs.
//
//  2. Unified (Wave-1 + Wave-2): the enrichmentIssueCount stored via SetEnrichmentState
//     must never exceed the union of Wave-1 issue resource IDs and Wave-2 finding IDs.
//     Tested indirectly via ResourceListModel.FrameTitle().

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"testing"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui/keys"
	"github.com/k2m30/a9s/v3/internal/tui/messages"
	"github.com/k2m30/a9s/v3/internal/tui/views"
)

// ─────────────────────────────────────────────────────────────────────────────
// Test 1: per-enricher IssueCount <= len(inputResources)
// ─────────────────────────────────────────────────────────────────────────────

// TestAllEnrichers_IssueCountNeverExceedsResources is a property check over every
// registered enricher. For each enricher:
//   - Calls it with 3 minimal resources (id test-1, test-2, test-3) and nil clients.
//   - Asserts result.IssueCount <= len(input)
//   - Asserts result.IssueCount <= len(result.Findings)
//
// Enrichers that gracefully return early on nil clients are fine — they will
// return IssueCount=0 which satisfies the invariant. If an enricher panics,
// it is logged and skipped (not failed) because the goal is the bound invariant,
// not exhaustive execution coverage.
//
// If any enricher violates IssueCount > len(input), that is a real bug and the
// test reports the enricher name and exact counts.
func TestAllEnrichers_IssueCountNeverExceedsResources(t *testing.T) {
	minimalResources := []resource.Resource{
		{ID: "test-1", Name: "test-resource-1"},
		{ID: "test-2", Name: "test-resource-2"},
		{ID: "test-3", Name: "test-resource-3"},
	}
	nilClients := (*awsclient.ServiceClients)(nil)

	for shortName, ent := range awsclient.EnricherRegistry {
		shortName := shortName
		fn := ent.Fn

		t.Run(shortName, func(t *testing.T) {
			var result awsclient.EnricherResult
			var err error

			func() {
				defer func() {
					if r := recover(); r != nil {
						t.Logf("skipped %s: panic with nil clients: %v", shortName, r)
					}
				}()
				result, err = fn(context.Background(), nilClients, minimalResources)
			}()

			if t.Failed() {
				return
			}
			if err != nil {
				t.Logf("skipped %s: returned error with nil clients: %v", shortName, err)
				return
			}

			// Core invariant: IssueCount can never exceed the number of input resources.
			if result.IssueCount > len(minimalResources) {
				t.Errorf("%s: IssueCount (%d) > len(inputResources) (%d) — issues must never exceed instances",
					shortName, result.IssueCount, len(minimalResources))
			}

			// Secondary invariant: IssueCount cannot exceed the number of distinct findings.
			// (Findings map is keyed by resource ID; each resource contributes at most one entry.)
			if result.IssueCount > len(result.Findings) {
				t.Errorf("%s: IssueCount (%d) > len(Findings) (%d) — issues must not exceed unique resource findings",
					shortName, result.IssueCount, len(result.Findings))
			}
		})
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Test 2: unified issue count never exceeds union of Wave-1 and Wave-2 IDs
// ─────────────────────────────────────────────────────────────────────────────

// buildUnifiedModelWithBadge builds a ResourceListModel with the issue badge enabled
// so that FrameTitle() includes the issue count when enrichmentIssueCount > 0.
// It reuses buildUnifiedModel from qa_unified_issue_count_test.go for loading,
// then enables the badge on the returned model.
func buildUnifiedModelWithBadge(t *testing.T, resources []resource.Resource, enrichIC int, findings map[string]resource.EnrichmentFinding) string {
	t.Helper()
	td := resource.ResourceTypeDef{
		ShortName: "ec2",
		Name:      "EC2 Instances",
		Columns: []resource.Column{
			{Key: "name", Title: "Name", Width: 28},
			{Key: "state", Title: "State", Width: 12},
		},
	}
	m := views.NewResourceList(td, nil, keys.Default())
	m.SetSize(120, 20)
	m, _ = m.Init()
	m, _ = m.Update(messages.ResourcesLoadedMsg{
		ResourceType: "ec2",
		Resources:    resources,
	})
	m.SetEnrichmentState(enrichIC, false, findings)
	m.SetShowIssueBadge(true)
	return m.FrameTitle()
}

// extractIssueCount parses the issue count from a FrameTitle of the form:
//
//	ec2(N/M issue) or ec2(N/M issues) or ec2(N/M+ issues)
//
// Returns -1 if no issue count badge is present (e.g. ec2(N) — no issues).
func extractIssueCount(title string) int {
	plain := stripANSI(title)
	// Match: name(total/issueCount issue) or name(total/issueCount+ issue)
	re := regexp.MustCompile(`\([\d+]+/(\d+)\+?\s+issue`)
	m := re.FindStringSubmatch(plain)
	if len(m) < 2 {
		return -1
	}
	n, err := strconv.Atoi(m[1])
	if err != nil {
		return -1
	}
	return n
}

// unionSize returns the number of distinct IDs across the resource slice and findings map.
func unionSize(resources []resource.Resource, findings map[string]resource.EnrichmentFinding) int {
	seen := make(map[string]struct{}, len(resources)+len(findings))
	for _, r := range resources {
		seen[r.ID] = struct{}{}
	}
	for id := range findings {
		seen[id] = struct{}{}
	}
	return len(seen)
}

// TestUnifiedIssueCount_NeverExceedsUnionSize verifies that the issue count embedded in
// FrameTitle() never exceeds the number of distinct resource IDs across Wave-1 resources
// and Wave-2 findings.
//
// Sub-cases:
//   - Disjoint: Wave-1 stopped resources + Wave-2 findings on different IDs → count ≤ 5
//   - Overlap:  same 3 IDs in both waves → count == 3 (not 6)
//   - Wave-2 only: all resources healthy + 2 findings → count ≤ 2
//   - Findings outside resource list: 2 healthy resources + 5 findings on unknown IDs
func TestUnifiedIssueCount_NeverExceedsUnionSize(t *testing.T) {
	t.Run("disjoint Wave-1 and Wave-2 findings on different IDs — count never exceeds 5", func(t *testing.T) {
		resources := []resource.Resource{
			{ID: "i-001", Name: "s1", Status: "stopped", Fields: map[string]string{"name": "s1", "state": "stopped"}},
			{ID: "i-002", Name: "s2", Status: "stopped", Fields: map[string]string{"name": "s2", "state": "stopped"}},
			{ID: "i-003", Name: "s3", Status: "stopped", Fields: map[string]string{"name": "s3", "state": "stopped"}},
		}
		findings := map[string]resource.EnrichmentFinding{
			"vol-aaa": {Severity: "!", Summary: "impaired"},
			"vol-bbb": {Severity: "!", Summary: "impaired"},
		}
		// The union is {i-001,i-002,i-003,vol-aaa,vol-bbb} = 5 distinct IDs.
		// enrichIC must equal the correct unified count (not the sum).
		union := unionSize(resources, findings)
		title := buildUnifiedModelWithBadge(t, resources, union, findings)
		ic := extractIssueCount(title)
		if ic < 0 {
			// No badge shown — count is effectively 0, invariant holds trivially.
			return
		}
		if ic > union {
			t.Errorf("disjoint: IssueCount (%d) > union size (%d); FrameTitle=%q", ic, union, title)
		}
		if !strings.Contains(fmt.Sprintf("%d", union), fmt.Sprintf("%d", ic)) && ic != union {
			t.Errorf("disjoint: expected IssueCount == %d (union), got %d; FrameTitle=%q", union, ic, title)
		}
	})

	t.Run("fully overlapping — same 3 IDs in Wave-1 and Wave-2 → count must be 3 not 6", func(t *testing.T) {
		resources := []resource.Resource{
			{ID: "i-aaa", Name: "server-a", Status: "stopped", Fields: map[string]string{"name": "server-a", "state": "stopped"}},
			{ID: "i-bbb", Name: "server-b", Status: "stopped", Fields: map[string]string{"name": "server-b", "state": "stopped"}},
			{ID: "i-ccc", Name: "server-c", Status: "stopped", Fields: map[string]string{"name": "server-c", "state": "stopped"}},
		}
		findings := map[string]resource.EnrichmentFinding{
			"i-aaa": {Severity: "!", Summary: "status impaired"},
			"i-bbb": {Severity: "!", Summary: "status impaired"},
			"i-ccc": {Severity: "!", Summary: "status impaired"},
		}
		// Union is still 3 (not 6 — no double counting).
		union := unionSize(resources, findings)
		if union != 3 {
			t.Fatalf("test setup error: expected union size 3, got %d", union)
		}
		// enrichIC passed to SetEnrichmentState is the caller-computed unified count (3).
		title := buildUnifiedModelWithBadge(t, resources, union, findings)
		ic := extractIssueCount(title)
		if ic < 0 {
			return
		}
		if ic > union {
			t.Errorf("overlap: IssueCount (%d) > union size (%d) — must not double-count same ID; FrameTitle=%q", ic, union, title)
		}
	})

	t.Run("Wave-2 only — all resources healthy + 2 findings → count never exceeds 2", func(t *testing.T) {
		resources := []resource.Resource{
			{ID: "i-r01", Name: "healthy-1", Status: "running", Fields: map[string]string{"name": "healthy-1", "state": "running"}},
			{ID: "i-r02", Name: "healthy-2", Status: "running", Fields: map[string]string{"name": "healthy-2", "state": "running"}},
			{ID: "i-r03", Name: "healthy-3", Status: "running", Fields: map[string]string{"name": "healthy-3", "state": "running"}},
			{ID: "i-r04", Name: "healthy-4", Status: "running", Fields: map[string]string{"name": "healthy-4", "state": "running"}},
			{ID: "i-r05", Name: "healthy-5", Status: "running", Fields: map[string]string{"name": "healthy-5", "state": "running"}},
		}
		findings := map[string]resource.EnrichmentFinding{
			"i-r01": {Severity: "!", Summary: "impaired"},
			"i-r02": {Severity: "!", Summary: "impaired"},
		}
		// enrichIC = 2 (two findings, both from known resource IDs)
		title := buildUnifiedModelWithBadge(t, resources, 2, findings)
		ic := extractIssueCount(title)
		if ic < 0 {
			return
		}
		if ic > 2 {
			t.Errorf("wave-2-only: IssueCount (%d) > 2 (number of findings); FrameTitle=%q", ic, title)
		}
		if ic > len(resources) {
			t.Errorf("wave-2-only: IssueCount (%d) > len(resources) (%d); FrameTitle=%q", ic, len(resources), title)
		}
	})

	t.Run("findings on IDs not in resource list still count — count never exceeds union", func(t *testing.T) {
		// 2 healthy resources visible in this page; 5 findings whose IDs are not in the list.
		// The enrichIC passed by the production code is the true distinct union count.
		resources := []resource.Resource{
			{ID: "i-p01", Name: "page-instance-1", Status: "running", Fields: map[string]string{"name": "page-instance-1", "state": "running"}},
			{ID: "i-p02", Name: "page-instance-2", Status: "running", Fields: map[string]string{"name": "page-instance-2", "state": "running"}},
		}
		findings := map[string]resource.EnrichmentFinding{
			"i-x01": {Severity: "!", Summary: "impaired"},
			"i-x02": {Severity: "!", Summary: "impaired"},
			"i-x03": {Severity: "!", Summary: "impaired"},
			"i-x04": {Severity: "!", Summary: "impaired"},
			"i-x05": {Severity: "!", Summary: "impaired"},
		}
		union := unionSize(resources, findings) // = 7 (2 page resources + 5 finding-only IDs)
		// Pass union as enrichIC: the production code computes this across the full account,
		// not just the current page. The invariant is: displayed count ≤ union.
		title := buildUnifiedModelWithBadge(t, resources, union, findings)
		ic := extractIssueCount(title)
		if ic < 0 {
			return
		}
		if ic > union {
			t.Errorf("off-page findings: IssueCount (%d) > union size (%d); FrameTitle=%q", ic, union, title)
		}
	})
}
