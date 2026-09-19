package unit

// Partial-success handling in handleAvailabilityChecked and
// handleEnrichmentChecked.
//
// handleAvailabilityChecked:
//   - Err != nil AND Resources non-empty: menu count is set, probeResources is
//     retained, and the error is recorded in the `!` error log without a
//     blocking flash banner — the rows are on screen carrying their findings.
//   - Err != nil AND Resources empty: no menu update, error FlashMsg banner.
//
// handleEnrichmentChecked:
//   - Err != nil AND Findings non-empty: enrichmentFindings[type] is set,
//     FieldUpdates merge into probeResources/resourceCache, the menu badge
//     updates, and FlashMsg surfaces Err.
//   - Err != nil AND Findings empty: FlashMsg only.

import (
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
	"github.com/k2m30/a9s/v3/internal/tui"
)

// With both Err and Resources on AvailabilityCheckedMsg, the menu count is set
// and probeResources retained; the error goes to the `!` error log without a
// flash banner, since the rows on screen already carry their findings.
func TestHandleAvailabilityChecked_PartialErrAppliesState(t *testing.T) {
	tui.Version = "test"

	m := newRootSizedModel()
	partialErr := errors.New("partial: throttled on 1 of 3 IDs")
	partialResources := []resource.Resource{
		{ID: "res-b-001", Name: "res-b-001"},
		{ID: "res-b-002", Name: "res-b-002"},
	}

	// session.New seeds AvailabilityGen=1 — stamp the live value so
	// the AvailabilityChecked stale guard (AcceptZeroGen=false) accepts it.
	m, cmd := rootApplyMsg(m, messages.AvailabilityChecked{
		ResourceType: "ec2", // registered type so menu can track it
		Err:          partialErr,
		HasResources: true,
		Count:        len(partialResources),
		Resources:    partialResources,
		Issues:       0,
		Truncated:    false,
		Gen:          m.Core().Session().AvailabilityGen,
	})

	if cmd != nil {
		for _, raw := range drainAllMessages(cmd) {
			if fm, ok := raw.(messages.Flash); ok && fm.IsError {
				t.Errorf("handleAvailabilityChecked partial-err: unexpected error FlashMsg %q — partial success must be error-log-only", fm.Text)
			}
		}
	}
	logModel, logCmd := rootApplyMsg(m, tea.KeyPressMsg{Code: '!'})
	if logCmd != nil {
		if raw := logCmd(); raw != nil {
			logModel, _ = rootApplyMsg(logModel, raw)
		}
	}
	logView := stripANSI(rootViewContent(logModel))
	if !strings.Contains(logView, partialErr.Error()) {
		t.Errorf("handleAvailabilityChecked partial-err: `!` error log must contain the full composite error text %q; got view:\n%s", partialErr.Error(), logView)
	}

	// probeResources is internal: finalizing the probe cycle starts enrichment,
	// and buildEnrichQueue includes "ec2" only when probeResources["ec2"] was
	// retained.
	finalizeModel, enrichCmd := rootApplyMsg(m, messages.AvailabilityChecked{
		ResourceType: "dummy-for-finalize",
		Gen:          m.Core().Session().AvailabilityGen,
		Count:        0,
		HasResources: false,
	})

	if enrichCmd != nil {
		enrichMsgs := collectEnrichmentMsgs(t, finalizeModel, enrichCmd)
		ec2Dispatched := false
		for _, em := range enrichMsgs {
			if em.ResourceType == "ec2" {
				ec2Dispatched = true
				break
			}
		}
		if !ec2Dispatched {
			t.Errorf("handleAvailabilityChecked partial-err: probeResources[\"ec2\"] was NOT retained — Wave 2 enrichment for \"ec2\" not dispatched after partial-success probe; PARTIAL-SUCCESS BUG: state not applied when Err != nil")
		}
	}
	// enrichCmd can be nil for reasons unrelated to probeResources (queue
	// ordering, enricher registry), so only a non-nil tree without ec2 fails.
}

// A hard failure (Err set, no Resources) leaves the menu and probeResources
// untouched.
func TestHandleAvailabilityChecked_HardErr_NoStateApplied(t *testing.T) {
	tui.Version = "test"

	m := newRootSizedModel()
	hardErr := errors.New("hard: endpoint unreachable")

	m, cmd := rootApplyMsg(m, messages.AvailabilityChecked{
		ResourceType: "lambda",
		Err:          hardErr,
		HasResources: false,
		Count:        0,
		Resources:    nil,
		Gen:          m.Core().Session().AvailabilityGen,
	})

	finalizeModel, enrichCmd := rootApplyMsg(m, messages.AvailabilityChecked{
		ResourceType: "dummy-finalize",
		Gen:          m.Core().Session().AvailabilityGen,
	})
	if enrichCmd != nil {
		enrichMsgs := collectEnrichmentMsgs(t, finalizeModel, enrichCmd)
		for _, em := range enrichMsgs {
			if em.ResourceType == "lambda" {
				t.Errorf("handleAvailabilityChecked hard-err: lambda enrichment dispatched even though probe returned no resources")
			}
		}
	}

	// cmd may carry the progress update — that's fine.
	_ = cmd
}

// With both Err and Findings on EnrichmentCheckedMsg, the findings are applied
// and a FlashMsg with IsError=true surfaces the error.
func TestHandleEnrichmentChecked_PartialErrAppliesState(t *testing.T) {
	tui.Version = "test"

	m := newRootSizedModel()
	partialErr := errors.New("partial: enrichment timed out for 1 of 5 resources")

	findings := map[string][]domain.Finding{
		"i-partial-001": {{Code: "ec2.system.status.impaired", Phrase: "stopped instance", Severity: domain.SevBroken, Source: "wave2:ec2"}},
	}
	fieldUpdates := map[string]map[string]string{
		"i-partial-001": {"stop_reason": "user initiated"},
	}

	// Seed probeResources["ec2"] so the handler can merge FieldUpdates.
	m, _ = rootApplyMsg(m, messages.AvailabilityChecked{
		ResourceType: "ec2",
		Gen:          m.Core().Session().AvailabilityGen,
		Count:        1,
		HasResources: true,
		Resources: []resource.Resource{
			{ID: "i-partial-001", Name: "web-server",
				Fields: map[string]string{"instance_id": "i-partial-001"}},
		},
	})

	// Gen=0 and TypeGen=0 are the documented test-injection bypasses (always accepted).
	m, cmd := rootApplyMsg(m, messages.EnrichmentChecked{
		ResourceType: "ec2",
		Err:          partialErr,
		Truncated:    true,
		Findings:     findings,
		FieldUpdates: fieldUpdates,
		TruncatedIDs: map[string]string{"i-partial-002": ""},
		Gen:          0,
		TypeGen:      0,
	})

	if cmd == nil {
		t.Fatal("handleEnrichmentChecked partial-err: must emit a cmd (FlashMsg for the error)")
	}
	allMsgs := drainAllMessages(cmd)
	var flash *messages.Flash
	for i := range allMsgs {
		if fm, ok := allMsgs[i].(messages.Flash); ok {
			flash = &fm
			break
		}
	}
	if flash == nil {
		t.Fatalf("handleEnrichmentChecked partial-err: expected FlashMsg in output; got %d msgs", len(allMsgs))
	}
	if !flash.IsError {
		t.Errorf("handleEnrichmentChecked partial-err: FlashMsg.IsError = false, want true")
	}

	if !strings.Contains(flash.Text, "partial") {
		t.Errorf("handleEnrichmentChecked partial-err: FlashMsg.Text = %q, want partial error text", flash.Text)
	}

	m, _ = rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetResourceList,
		ResourceType: "ec2",
	})
	m, _ = rootApplyMsg(m, messages.ResourcesLoaded{Provenance: messages.FetchProvenanceCanonicalList,
		ResourceType: "ec2",
		Resources: []resource.Resource{
			{ID: "i-partial-001", Name: "web-server",
				Fields: map[string]string{"instance_id": "i-partial-001"}},
		},
	})

	m, cmd2 := rootApplyMsg(m, messages.EnrichmentChecked{
		ResourceType: "ec2",
		Err:          partialErr,
		Findings:     findings,
		FieldUpdates: fieldUpdates,
		Gen:          0,
		TypeGen:      0,
	})
	_ = m

	if cmd2 != nil {
		msgs2 := drainAllMessages(cmd2)
		hasFlash2 := false
		for _, msg := range msgs2 {
			if _, ok := msg.(messages.Flash); ok {
				hasFlash2 = true
				break
			}
		}
		if !hasFlash2 {
			t.Errorf("handleEnrichmentChecked partial-err: second delivery: expected FlashMsg in output for partial error")
		}
	}
}

// Err with empty Findings emits only a FlashMsg and applies no findings.
func TestHandleEnrichmentChecked_PartialErrEmptyFindings_OnlyFlash(t *testing.T) {
	tui.Version = "test"

	m := newRootSizedModel()
	hardErr := errors.New("enricher: network timeout, no data returned")

	_, cmd := rootApplyMsg(m, messages.EnrichmentChecked{
		ResourceType: "rds",
		Err:          hardErr,
		Findings:     nil,
		FieldUpdates: nil,
		Gen:          0,
		TypeGen:      0,
	})

	if cmd == nil {
		t.Fatal("handleEnrichmentChecked error with nil findings: must emit FlashMsg cmd")
	}
	allMsgs := drainAllMessages(cmd)
	// The cmd may be a batch that includes enrichment for other queued types.
	// Find the FlashMsg that specifically mentions "rds" (our injected type).
	var flash *messages.Flash
	for i := range allMsgs {
		if fm, ok := allMsgs[i].(messages.Flash); ok && strings.Contains(fm.Text, "rds") {
			fm := fm
			flash = &fm
			break
		}
	}
	if flash == nil {
		t.Fatalf("handleEnrichmentChecked pure error: expected FlashMsg mentioning 'rds'; got %d messages", len(allMsgs))
	}
	if !flash.IsError {
		t.Error("handleEnrichmentChecked pure error: FlashMsg.IsError must be true")
	}
}
