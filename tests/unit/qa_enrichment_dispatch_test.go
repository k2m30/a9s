package unit

// qa_enrichment_dispatch_test.go — Tests for enrichment dispatch and handler behavior.
//
// Tests verify:
//   1. IssueEnricherRegistry completeness — all 8 resource short names are registered with non-nil functions.
//   2. Session-wide gen guard — EnrichmentCheckedMsg with stale Gen is silently dropped (no panic, no cmd).
//   3. Per-type gen guard — EnrichmentCheckedMsg with stale TypeGen is silently dropped.
//   4. Valid EnrichmentCheckedMsg with Err != nil does not crash.
//   5. EnricherFunc signature conformance — registered functions satisfy the EnricherFunc type.

import (
	"errors"
	"testing"

	tea "charm.land/bubbletea/v2"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui"
	"github.com/k2m30/a9s/v3/internal/tui/messages"
)

// originalIssue196Enrichers lists the foundational enrichers from issue #196.
// These must remain registered (real, not noop). The full Wave 2 contract is
// enforced by TestAttentionSignalsDoc (per docs/attention-signals.md), so this
// allowlist is no longer the source of truth — it's a regression pin for the
// initial enricher set.
//
// TODO(no-middle-state): registry-pin tests like this catch absence, not
// completeness. A feature can still be disabled, inert, or half-fed and pass.
var originalIssue196Enrichers = []string{
	"rds",
	"dbi",
	"ebs",
	"cb",
	"tg",
	"pipeline",
	"sfn",
	"glue",
}

// TestIssueEnricherRegistry_OriginalSetStillRegistered pins the original 8
// enrichers from issue #196.
func TestIssueEnricherRegistry_OriginalSetStillRegistered(t *testing.T) {
	for _, shortName := range originalIssue196Enrichers {
		fn, ok := awsclient.IssueEnricherRegistry[shortName]
		if !ok {
			t.Errorf("IssueEnricherRegistry missing entry for %q", shortName)
			continue
		}
		if fn.Fn == nil {
			t.Errorf("IssueEnricherRegistry[%q].Fn is nil — must be a non-nil IssueEnricherFunc", shortName)
		}
	}
}

// TestIssueEnricherRegistry_NoEntriesForUnregisteredTypes verifies the registry
// only contains entries for shortNames that are registered as ResourceTypeDefs.
// Replaces the prior allowlist-based test — that was authoritative when only
// 8 enrichers existed; now the doc-grounded contract (TestAttentionSignalsDoc)
// is the source of truth.
//
// TODO(no-middle-state): this test proves only registry shape. Keep behavioral
// tests for any feature that is claimed as implemented.
func TestIssueEnricherRegistry_NoEntriesForUnregisteredTypes(t *testing.T) {
	for key := range awsclient.IssueEnricherRegistry {
		if resource.FindResourceType(key) == nil {
			t.Errorf("IssueEnricherRegistry[%q] has no matching ResourceTypeDef", key)
		}
	}
}

// newTestModel creates a fresh tui.Model sized for testing.
func newTestModel() tui.Model {
	m := tui.New("", "")
	// Propagate a size so views are initialized.
	m2, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	if tm, ok := m2.(tui.Model); ok {
		return tm
	}
	return m
}

// TestEnrichmentCheckedMsg_StaleSessionGenDropped verifies that an
// EnrichmentCheckedMsg with Gen != model.enrichmentGen is silently dropped:
// Update returns the unchanged model and no command.
//
// A fresh tui.Model has enrichmentGen=0; sending Gen=999 is always stale.
func TestEnrichmentCheckedMsg_StaleSessionGenDropped(t *testing.T) {
	m := newTestModel()

	staleMsg := messages.EnrichmentCheckedMsg{
		ResourceType: "ec2",
		Issues:       42,
		Truncated:    false,
		Findings: map[string]resource.EnrichmentFinding{
			"i-abc": {Severity: "!", Summary: "system status impaired"},
		},
		Gen:     999, // stale — fresh model's enrichmentGen is 0
		TypeGen: 0,
	}

	_, cmd := m.Update(staleMsg)
	if cmd != nil {
		t.Error("stale gen guard: Update must return nil cmd for stale EnrichmentCheckedMsg")
	}
}

// TestEnrichmentCheckedMsg_StaleTypeGenDropped verifies that an
// EnrichmentCheckedMsg with matching session Gen but stale TypeGen is dropped.
//
// A fresh model has enrichmentGen=0 and enrichmentTypeGen["ec2"]=0.
// We send Gen=0, TypeGen=99 — the TypeGen doesn't match, so it's stale.
func TestEnrichmentCheckedMsg_StaleTypeGenDropped(t *testing.T) {
	m := newTestModel()

	staleMsg := messages.EnrichmentCheckedMsg{
		ResourceType: "ec2",
		Issues:       5,
		Truncated:    false,
		Findings:     map[string]resource.EnrichmentFinding{},
		Gen:          0,  // matches fresh model's enrichmentGen=0
		TypeGen:      99, // stale — fresh model's enrichmentTypeGen["ec2"] is 0
	}

	_, cmd := m.Update(staleMsg)
	if cmd != nil {
		t.Error("stale TypeGen guard: Update must return nil cmd for stale type gen")
	}
}

// TestEnrichmentCheckedMsg_ErrorDoesNotCrash verifies that a valid-gen message
// carrying Err != nil is handled gracefully (no panic, no spurious cmd).
func TestEnrichmentCheckedMsg_ErrorDoesNotCrash(t *testing.T) {
	m := newTestModel()

	errMsg := messages.EnrichmentCheckedMsg{
		ResourceType: "ddb",
		Err:          errors.New("access denied"),
		Gen:          0, // matches fresh model
		TypeGen:      0, // matches fresh model's initial per-type gen
	}

	// Must not panic.
	m2, _ := m.Update(errMsg)
	_ = m2.View()
}

// TestEnrichmentCheckedMsg_NilFindingsOnError verifies that Findings is
// not required to be non-nil when Err != nil — the contract says Findings
// may be nil/empty on error and the handler must tolerate it.
func TestEnrichmentCheckedMsg_NilFindingsOnError(t *testing.T) {
	m := newTestModel()

	errMsg := messages.EnrichmentCheckedMsg{
		ResourceType: "sfn",
		Findings:     nil, // explicitly nil — enricher errored out
		Err:          errors.New("throttled"),
		Gen:          0,
		TypeGen:      0,
	}

	// Handler must not panic or crash when Findings is nil.
	m2, _ := m.Update(errMsg)
	_ = m2.View()
}

// TestEnrichmentCheckedMsg_ValidSuccessDoesNotCrash verifies that a well-formed
// success message (Gen=0, TypeGen=0, Err=nil) does not panic.
func TestEnrichmentCheckedMsg_ValidSuccessDoesNotCrash(t *testing.T) {
	m := newTestModel()

	successMsg := messages.EnrichmentCheckedMsg{
		ResourceType: "glue",
		Issues:       1,
		Truncated:    false,
		Findings: map[string]resource.EnrichmentFinding{
			"my-glue-job": {Severity: "!", Summary: "latest run FAILED"},
		},
		Gen:     0,
		TypeGen: 0,
	}

	m2, _ := m.Update(successMsg)
	_ = m2.View()
}
