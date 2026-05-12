package runtime

// handlers_availability_test.go — locks the two NEEDS-CHANGES invariants from
// PR #344 Stage 5 review:
//
//  1. Truncation precedence in handleEnrichmentChecked: Wave-1 ProbeTruncated
//     is authoritative — it must override the zero-issues clear, so a
//     truncated availability scan keeps the badge truncated even when the
//     visible subset shows no issues.
//
//  2. PatchDetail.EnrichmentFindings nil = clear contract: when Wave-2 returns
//     no findings, the emitted PatchDetail intent must carry nil
//     EnrichmentFindings so the adapter clears stale detail markers.

import (
	"testing"

	"github.com/k2m30/a9s/v3/internal/catalog"
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/session"
	"github.com/k2m30/a9s/v3/internal/runtime/messages"
)

// findPatchMenu returns the first PatchMenu intent in xs whose ResourceType
// matches rt, or nil if none.
func findPatchMenu(xs []UIIntent, rt string) *PatchMenu {
	for _, x := range xs {
		if pm, ok := x.(PatchMenu); ok && pm.ResourceType == rt {
			return &pm
		}
	}
	return nil
}

// findPatchDetail returns the first PatchDetail intent in xs whose ResourceType
// matches rt, or nil if none.
func findPatchDetail(xs []UIIntent, rt string) *PatchDetail {
	for _, x := range xs {
		if pd, ok := x.(PatchDetail); ok && pd.ResourceType == rt {
			return &pd
		}
	}
	return nil
}

// pickKnownShortName returns the first registered ShortName from the static
// catalog, so the tests don't hard-code a value that could be removed later.
func pickKnownShortName(t *testing.T) string {
	t.Helper()
	if len(catalog.ResourceTypes) == 0 {
		t.Fatal("catalog.ResourceTypes is empty — cannot run handler test")
	}
	return catalog.ResourceTypes[0].ShortName
}

// TestHandleEnrichmentChecked_TruncationPrecedence_Wave1Wins covers the
// regression case Architect/CodexReviewer flagged: Wave-1 probe truncated +
// Wave-2 sees zero issues + zero findings → badge must stay Truncated=true.
//
// The deleted handler applied the override after the zero-clear step; the
// runtime port must preserve that ordering.
func TestHandleEnrichmentChecked_TruncationPrecedence_Wave1Wins(t *testing.T) {
	rt := pickKnownShortName(t)

	sess := session.New()
	sess.ProbeTruncated = map[string]bool{rt: true}

	c := New(sess, catalog.ResourceTypes)

	intents, _ := c.handleEnrichmentChecked(messages.EnrichmentChecked{
		ResourceType: rt,
		Issues:       0,
		Truncated:    false,
		Findings:     nil,
	})

	pm := findPatchMenu(intents, rt)
	if pm == nil {
		t.Fatalf("no PatchMenu intent emitted for %q", rt)
	}
	if !pm.Truncated {
		t.Fatalf("expected Truncated=true (Wave-1 ProbeTruncated must override zero-issues clear); got false. PatchMenu=%+v", pm)
	}
}

// TestHandleEnrichmentChecked_TruncationPrecedence_NoWave1NoIssues_ClearsToFalse
// covers the symmetric case: no Wave-1 truncation + zero issues + zero
// findings → badge goes back to Truncated=false.
func TestHandleEnrichmentChecked_TruncationPrecedence_NoWave1NoIssues_ClearsToFalse(t *testing.T) {
	rt := pickKnownShortName(t)

	sess := session.New()
	c := New(sess, catalog.ResourceTypes)

	intents, _ := c.handleEnrichmentChecked(messages.EnrichmentChecked{
		ResourceType: rt,
		Issues:       0,
		Truncated:    true, // Wave-2 thinks it's truncated, but no issues observed
		Findings:     nil,
	})

	pm := findPatchMenu(intents, rt)
	if pm == nil {
		t.Fatalf("no PatchMenu intent emitted for %q", rt)
	}
	if pm.Truncated {
		t.Fatalf("expected Truncated=false (no Wave-1 trunc, no issues observed); got true. PatchMenu=%+v", pm)
	}
}

// TestHandleEnrichmentChecked_TruncationPrecedence_Wave2WithFindings_StaysTruthy
// covers the case where Wave-2 is genuinely truncated and reports findings —
// the badge stays truncated regardless of Wave-1.
func TestHandleEnrichmentChecked_TruncationPrecedence_Wave2WithFindings_StaysTruthy(t *testing.T) {
	rt := pickKnownShortName(t)

	sess := session.New()
	c := New(sess, catalog.ResourceTypes)

	intents, _ := c.handleEnrichmentChecked(messages.EnrichmentChecked{
		ResourceType: rt,
		Issues:       3,
		Truncated:    true,
		Findings: map[string]resource.EnrichmentFinding{
			"id-1": {Severity: "!", Summary: "broken"},
		},
	})

	pm := findPatchMenu(intents, rt)
	if pm == nil {
		t.Fatalf("no PatchMenu intent emitted for %q", rt)
	}
	if !pm.Truncated {
		t.Fatalf("expected Truncated=true (Wave-2 reports truncation + findings); got false. PatchMenu=%+v", pm)
	}
}

// TestHandleEnrichmentChecked_PatchDetail_NilFindings_ClearsContract verifies
// the runtime emits PatchDetail with EnrichmentFindings=nil when Wave-2
// returned no findings, so the adapter (intent.go contract: nil = clear) wipes
// stale detail-view markers.
func TestHandleEnrichmentChecked_PatchDetail_NilFindings_ClearsContract(t *testing.T) {
	rt := pickKnownShortName(t)

	sess := session.New()
	c := New(sess, catalog.ResourceTypes)

	intents, _ := c.handleEnrichmentChecked(messages.EnrichmentChecked{
		ResourceType: rt,
		Findings:     nil,
	})

	pd := findPatchDetail(intents, rt)
	if pd == nil {
		t.Fatalf("no PatchDetail intent emitted for %q", rt)
	}
	if pd.EnrichmentFindings != nil {
		t.Fatalf("expected EnrichmentFindings=nil (clear contract); got %+v", pd.EnrichmentFindings)
	}
}

// TestHandleEnrichmentChecked_PatchDetail_NonNilFindings_PassesThrough verifies
// the symmetric case: when Wave-2 returns a populated findings map, the
// PatchDetail intent forwards it unchanged for the adapter to apply.
func TestHandleEnrichmentChecked_PatchDetail_NonNilFindings_PassesThrough(t *testing.T) {
	rt := pickKnownShortName(t)

	sess := session.New()
	c := New(sess, catalog.ResourceTypes)

	findings := map[string]resource.EnrichmentFinding{
		"id-1": {Severity: "!", Summary: "broken"},
		"id-2": {Severity: "~", Summary: "warn"},
	}
	intents, _ := c.handleEnrichmentChecked(messages.EnrichmentChecked{
		ResourceType: rt,
		Findings:     findings,
	})

	pd := findPatchDetail(intents, rt)
	if pd == nil {
		t.Fatalf("no PatchDetail intent emitted for %q", rt)
	}
	if len(pd.EnrichmentFindings) != len(findings) {
		t.Fatalf("expected %d EnrichmentFindings, got %d", len(findings), len(pd.EnrichmentFindings))
	}
	for k, v := range findings {
		got, ok := pd.EnrichmentFindings[k]
		if !ok {
			t.Fatalf("missing finding for %q", k)
		}
		if got.Severity != v.Severity || got.Summary != v.Summary {
			t.Fatalf("finding %q mismatch: want %+v, got %+v", k, v, got)
		}
	}
}
