// snapshot_cross_ref_internal_test.go — internal package tests for the
// unexported computeMergedStatus helper used by EnrichSnapshotCrossRef.
// Lives in internal/aws (not tests/unit) so the test can name the
// unexported symbol directly.
//
// The helper is currently unreachable in its multi-phrase branch from the
// snapshot enrichers themselves (orphan and past-retention are mutually
// exclusive by construction — parent absent XOR parent present). But
// Codex's review flagged the function as a landmine for future cross-ref
// enrichers that emit ≥2 phrases simultaneously. This test pins the
// contract loudly so a future reviewer doesn't trip the off-by-one trap.
package aws

import (
	"testing"

	"github.com/k2m30/a9s/v3/internal/resource"
)

// stripPlusN is a tiny helper that removes any trailing "(+N)" suffix —
// used in expected-value construction to avoid hand-typing the suffix.
//
//nolint:unused // helper kept for symmetry with hand-written cases below
func stripPlusN(s string) string { return resource.StripFindingSuffix(s) }

// TestComputeMergedStatus_MultiPhraseSuffix pins computeMergedStatus's
// (+N) suffix arithmetic across the cells of (existingIssues × newPhrases).
//
// The contract:
//   - existingStatus is the FETCHER-emitted Resource.Status: it already encodes
//     the suffix derived from len(existingIssues). Passed verbatim.
//   - newPhrases are the phrases this enricher contributes (idempotent — the
//     function reads only existingStatus, never a previously-merged value).
//   - return = top phrase + " (+N-1)" where N = len(existingIssues) + len(newPhrases).
//     Top phrase = existingStatus when non-empty, else newPhrases[0].
//
// Codex review B5 specifically called out the case len(newPhrases) ≥ 2 with
// an empty existingStatus — that branch produced "top (+1)" before the fix
// regardless of how many new phrases were appended. Each row below pins the
// CORRECT expected suffix.
func TestComputeMergedStatus_MultiPhraseSuffix(t *testing.T) {
	cases := []struct {
		name           string
		existingStatus string
		existingIssues []string
		newPhrases     []string
		want           string
	}{
		{
			name: "all_empty_returns_blank",
			want: "",
		},
		{
			name:       "single_new_phrase_no_existing",
			newPhrases: []string{"orphan: source DB deleted"},
			want:       "orphan: source DB deleted",
		},
		{
			name:           "fetcher_emits_phrase_keeps_phrase",
			existingStatus: "failed",
			existingIssues: []string{"failed"},
			want:           "failed",
		},
		{
			name:           "single_existing_no_new",
			existingStatus: "unencrypted",
			existingIssues: []string{"unencrypted"},
			want:           "unencrypted",
		},
		{
			name:           "single_existing_plus_single_new",
			existingStatus: "unencrypted",
			existingIssues: []string{"unencrypted"},
			newPhrases:     []string{"orphan: source DB deleted"},
			want:           "unencrypted (+1)",
		},
		// B5 — Codex regression pin. Two new phrases, no fetcher status:
		// must produce (+1), NOT bare top phrase. Pre-fix returned the
		// bare top because BumpFindingSuffix was called once total instead
		// of once per additional phrase.
		{
			name:       "two_new_phrases_no_existing",
			newPhrases: []string{"phrase-a", "phrase-b"},
			want:       "phrase-a (+1)",
		},
		// B5 — three new phrases, no fetcher status: must produce (+2).
		{
			name:       "three_new_phrases_no_existing",
			newPhrases: []string{"phrase-a", "phrase-b", "phrase-c"},
			want:       "phrase-a (+2)",
		},
		// Two existing (suffix already encoded) + one new: existing top wins,
		// suffix bumped by 1 to reflect the new phrase. The fetcher's own
		// (+1) for the second existingIssue is NOT double-counted.
		{
			name:           "two_existing_plus_one_new",
			existingStatus: "unencrypted (+1)",
			existingIssues: []string{"unencrypted", "another-fetcher-phrase"},
			newPhrases:     []string{"orphan: source DB deleted"},
			want:           "unencrypted (+2)",
		},
		// Two existing + two new: total 4, (+3) suffix.
		{
			name:           "two_existing_plus_two_new",
			existingStatus: "unencrypted (+1)",
			existingIssues: []string{"unencrypted", "another-fetcher-phrase"},
			newPhrases:     []string{"orphan: source DB deleted", "another-cross-ref"},
			want:           "unencrypted (+3)",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := computeMergedStatus(tc.existingStatus, tc.existingIssues, tc.newPhrases)
			if got != tc.want {
				t.Errorf("computeMergedStatus(%q, %v, %v)\n  got:  %q\n  want: %q",
					tc.existingStatus, tc.existingIssues, tc.newPhrases, got, tc.want)
			}
		})
	}
}

// TestComputeMergedStatus_FetcherFailedPlusOrphanKeepsFailedTop pins the B1
// regression: a fetcher-emitted Broken phrase ("failed") MUST survive cross-ref
// enrichment. When the enricher adds a single orphan phrase, the output must be
// "failed (+1)" — not "orphan: source cluster deleted" or any other reordering.
//
// This is the key correctness contract for WarnDBCSnapFailedAndManualOldID:
// the fetcher sets Status="failed" (Broken), the enricher adds "orphan:
// source cluster deleted", and the merged status must preserve the Broken
// phrase as the top with the orphan stacked as +1.
func TestComputeMergedStatus_MultiPhraseSuffix_FetcherFailedPlusOrphan(t *testing.T) {
	cases := []struct {
		name           string
		existingStatus string
		existingIssues []string
		newPhrases     []string
		want           string
	}{
		{
			name:           "fetcher_failed_plus_orphan_keeps_failed_top",
			existingStatus: "failed",
			existingIssues: []string{"failed"},
			newPhrases:     []string{"orphan: source cluster deleted"},
			want:           "failed (+1)",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := computeMergedStatus(tc.existingStatus, tc.existingIssues, tc.newPhrases)
			if got != tc.want {
				t.Errorf("computeMergedStatus(%q, %v, %v)\n  got:  %q\n  want: %q",
					tc.existingStatus, tc.existingIssues, tc.newPhrases, got, tc.want)
			}
		})
	}
}

// TestComputeMergedStatus_Idempotent verifies the helper is idempotent: passing
// the same arguments again does not double-suffix. This pin guards against a
// future maintainer "optimizing" the function to read its previous merged
// output as the top phrase — a regression that would re-introduce B1.
func TestComputeMergedStatus_Idempotent(t *testing.T) {
	existingStatus := "unencrypted"
	existingIssues := []string{"unencrypted"}
	newPhrases := []string{"orphan: source DB deleted"}

	first := computeMergedStatus(existingStatus, existingIssues, newPhrases)
	second := computeMergedStatus(existingStatus, existingIssues, newPhrases)

	if first != second {
		t.Errorf("computeMergedStatus is not idempotent — first=%q second=%q", first, second)
	}
	if first != "unencrypted (+1)" {
		t.Errorf("first call returned %q, want %q", first, "unencrypted (+1)")
	}
}
