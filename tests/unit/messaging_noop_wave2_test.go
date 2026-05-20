package unit

// messaging_noop_wave2_test.go — AS-726 PR-04i — verify that the two NoOp-only
// issue-enrichment files (sns_sub_issue_enrichment.go, kinesis_issue_enrichment.go)
// are gone and the catalog row's Wave2 == nil is the explicit "no Wave 2" signal.
//
// Why this test must verify the NEGATIVE:
//
// Before AS-726, sns-sub and kinesis each had a 6-line init() that called
// `registerIssueEnricher("sns-sub", NoOpIssueEnricher, 100)`. After AS-726,
// those files are deleted (spec §4) and catalog-row Wave2 == nil becomes the
// "no Wave 2" signal. `aws.GetIssueEnricher` must return `(IssueEnricher{}, false)`
// in that case — NOT a NoOp registration cast through catalog.Wave2 or routed
// through a fallback in the legacy IssueEnricherRegistry map.
//
// If the Coder accidentally leaves the NoOp call in place (e.g. moves it from
// a deleted file to a still-existing file), `ok` will be true, masking the
// deletion. The test catches that by asserting ok=false explicitly.

import (
	"testing"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
)

func TestNoOpWave2ReturnsAbsent(t *testing.T) {
	// Per spec §4: the NoOp-only init() blocks are deleted, and catalog
	// Wave2 == nil is the explicit "no Wave 2" signal. GetIssueEnricher
	// must surface that as ok=false.
	cases := []string{"sns-sub", "kinesis"}

	for _, sn := range cases {
		t.Run(sn, func(t *testing.T) {
			e, ok := awsclient.GetIssueEnricher(sn)
			if ok {
				t.Errorf("aws.GetIssueEnricher(%q) returned ok=true; expected false. "+
					"NoOp-only Wave 2 file should be deleted per AS-726 PR-04i §4. "+
					"Returned enricher: Priority=%d, Fn-nil=%v",
					sn, e.Priority, e.Fn == nil)
			}
			if e.Fn != nil {
				t.Errorf("aws.GetIssueEnricher(%q) returned non-nil Fn; expected zero IssueEnricher. "+
					"A stray re-registration is masking the NoOp deletion.", sn)
			}
		})
	}
}
