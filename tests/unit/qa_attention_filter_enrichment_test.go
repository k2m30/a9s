package unit

// qa_attention_filter_enrichment_test.go — Regression guard for the
// attention-filter (ctrl+z) inconsistency after Wave 2 enrichment.
//
// Scenario:
//   1. User opens a resource list.
//   2. User enables the attention filter (ctrl+z) BEFORE Wave 2 completes.
//      At this moment findingsByID is empty, so only rows with an issue
//      color survive the filter.
//   3. Wave 2 enrichment completes, SetEnrichmentState fires with a
//      non-empty findings map.
//   4. The filtered list must now include rows flagged by the new findings
//      WITHOUT requiring the user to toggle the filter or edit filter text.
//
// Pre-fix: SetEnrichmentState only updated findingsByID and did not re-run
// applySortAndFilter, so the filtered list stayed stale and showed fewer
// rows than the issue badge claimed. Post-fix: SetEnrichmentState re-runs
// applySortAndFilter.

import (
	"strings"
	"testing"

	"github.com/k2m30/a9s/v3/internal/config"
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui/keys"
	"github.com/k2m30/a9s/v3/internal/runtime/messages"
	"github.com/k2m30/a9s/v3/internal/tui/views"
)

func TestAttentionFilter_SetEnrichmentState_ReappliesFilter(t *testing.T) {
	td := resource.FindResourceType("s3")
	if td == nil {
		t.Fatal("s3 resource type not registered")
	}

	cfg := config.DefaultConfig()
	k := keys.Default()
	m := views.NewResourceList(*td, cfg, k)
	m.SetSize(200, 30)
	m, _ = m.Init()

	// Three Healthy buckets — none survive attention filtering on Wave-1
	// color alone. We expect Wave 2 to later flag "bucket-alpha" only.
	resources := []resource.Resource{
		{ID: "b-0", Name: "bucket-alpha", Fields: map[string]string{"name": "bucket-alpha", "region": "us-east-1"}},
		{ID: "b-1", Name: "bucket-beta", Fields: map[string]string{"name": "bucket-beta", "region": "us-east-1"}},
		{ID: "b-2", Name: "bucket-gamma", Fields: map[string]string{"name": "bucket-gamma", "region": "us-east-1"}},
	}
	m, _ = m.Update(messages.ResourcesLoaded{ResourceType: "s3", Resources: resources})

	// Enable attention filter BEFORE Wave 2 lands — mimics the user pressing
	// ctrl+z while enrichment is still in flight.
	m.SetEnabled(true)

	// Wave 2 completes and flags bucket-alpha via a finding. With the
	// SetEnrichmentState → applySortAndFilter re-run, the row must become
	// visible immediately — without the user toggling ctrl+z or editing the
	// filter text.
	findings := map[string]resource.EnrichmentFinding{
		"b-0": {Severity: "!", Summary: "public access enabled"},
	}
	m.SetEnrichmentState(1, false, findings)

	rendered := stripANSI(m.View())

	if !strings.Contains(rendered, "bucket-alpha") {
		t.Error(
			"SetEnrichmentState must re-run applySortAndFilter so rows newly flagged " +
				"by Wave 2 findings become visible under an already-enabled attention filter; " +
				"bucket-alpha was hidden after SetEnrichmentState fired.",
		)
	}
	if strings.Contains(rendered, "bucket-beta") {
		t.Error("unrelated healthy row bucket-beta must stay hidden under the attention filter")
	}
	if strings.Contains(rendered, "bucket-gamma") {
		t.Error("unrelated healthy row bucket-gamma must stay hidden under the attention filter")
	}
}
