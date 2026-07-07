// rowstore_fetch_origin_enrich_pin_test.go — regression pin for the
// fetch-origin blind-spot in Core.ProbeResources (internal/runtime/
// accessors.go). Before the fix, ProbeResources gated its RowStore read on
// tr.Origin being OriginProbe or OriginDisk, so a type whose ONLY retained
// entry carried OriginFetch (the exact shape produced by a `-c <type>`
// startup list-open, or any top-level list fetch, BEFORE the Wave-2 sweep's
// enrich task ran for that type) fed ProbeEnrichment an empty resources
// slice. The real Wave-2 enricher then ran against zero rows: zero findings,
// zero glyphs, and the eventual completion save carried no findings for that
// type — a silent, total loss of Wave-2 signal for whichever type happened
// to be open on screen when the sweep reached it.
//
// This file drives the REAL execution path end to end:
//  1. Seed a RowStore entry via Observe(..., session.OriginFetch, ...) — the
//     open-list-fetch shape, not a hand-built ObserveRows/OriginProbe seed.
//  2. Register a test Wave-2 enricher (SetWave2EnricherForTest) that records
//     the rows it was actually invoked with.
//  3. Call the real Core.ProbeEnrichment execution lane (TaskKindProbeEnrich's
//     production path) and assert the enricher saw the NON-EMPTY fetch-origin
//     rows.
//  4. Drive the completion flow via Core.Handle(messages.EnrichmentChecked)
//     and assert the dispatched TaskKindSaveCache payload's rows carry the
//     findings the enricher emitted.
//  5. Companion negative: a Partial-only entry must still be excluded from
//     the enricher's input (mirrors ProbeResources' remaining tr.Partial
//     gate, which this fix must not weaken).
package unit_test

import (
	"context"
	"testing"

	"github.com/k2m30/a9s/v3/internal/app"
	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/domain"
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/runtime"
	"github.com/k2m30/a9s/v3/internal/runtime/messages"
	"github.com/k2m30/a9s/v3/internal/session"
)

// newFetchOriginPinSession builds a minimal session+Core+Controller triple,
// mirroring newStage2PinTestController's shape but without the on-disk store
// isolation (this file never touches the disk cache), so ProbeEnrichment's
// nil-clients guard is satisfied by a non-nil, zero-value ServiceClients.
// The Controller is needed because Handle (the completion-flow entry point
// this file drives) lives on *app.Controller, not *runtime.Core directly.
func newFetchOriginPinSession(t *testing.T) (*session.Session, *runtime.Core, *app.Controller) {
	t.Helper()
	s := session.New()
	s.Profile = "demo"
	s.Region = "us-east-1"
	s.Clients = &awsclient.ServiceClients{}
	core := runtime.New(s, nil)
	c := app.New(core)
	t.Cleanup(c.Close)
	return s, core, c
}

// TestProbeEnrichment_FetchOriginRows_ReachesRealEnricher is the primary
// pin: the exact `-c <type>` startup shape — a top-level list fetch stamps
// OriginFetch on the type BEFORE the Wave-2 sweep's enrich task runs for it.
//
// Pre-fix (accessors.go @ 61c33e96): ProbeResources gates on
// tr.Origin == OriginProbe || tr.Origin == OriginDisk, so an OriginFetch-only
// entry never satisfies the gate; ProbeResources returns (nil, false) and
// ProbeEnrichment's `resources, _ := c.ProbeResources(shortName)` feeds the
// registered enricher a nil slice. This test's captureFn records len(rows),
// so it FAILS (wantLen=1, got 0) against the pre-fix accessor.
//
// Post-fix: ProbeResources accepts any full-population origin (Disk, Probe,
// or Fetch) and only excludes tr.Partial, so the fetch-origin row reaches
// the enricher intact.
func TestProbeEnrichment_FetchOriginRows_ReachesRealEnricher(t *testing.T) {
	const sentinelType = "dbi-fetch-origin-enrich-pin"

	s, core, _ := newFetchOriginPinSession(t)

	fetchRow := resource.Resource{
		ID:   "rds:fetch-origin-open-list-1",
		Name: "fetch-origin-open-list-1",
		Type: sentinelType,
		Fields: map[string]string{
			"engine": "postgres",
		},
	}
	// The open-screen shape: a top-level list fetch stamps OriginFetch, not
	// OriginProbe — mirrors how applyResourcesLoaded/ObserveRows records a
	// `-c <type>` startup list open, BEFORE any Wave-2 sweep task for this
	// type has run.
	s.RowStore.Observe(sentinelType, []resource.Resource{fetchRow}, nil, session.OriginFetch, false)

	var seenRows []resource.Resource
	captureFn := func(_ context.Context, _ *awsclient.ServiceClients, rows []resource.Resource, _ resource.ResourceCache) (awsclient.IssueEnricherResult, error) {
		seenRows = rows
		return awsclient.IssueEnricherResult{
			IssueCount: 1,
			Findings: map[string][]domain.Finding{
				fetchRow.ID: {{
					Code:     "fetch-origin-pin-finding",
					Phrase:   "fetch-origin pin finding",
					Severity: domain.SevWarn,
					Source:   "wave2:" + sentinelType,
				}},
			},
		}, nil
	}
	awsclient.SetWave2EnricherForTest(t, sentinelType, awsclient.IssueEnricher{Fn: captureFn, Priority: 100})

	result := core.ProbeEnrichment(context.Background(), s.Clients, sentinelType)
	if result.Err != nil {
		t.Fatalf("ProbeEnrichment returned unexpected error: %v", result.Err)
	}

	if len(seenRows) != 1 {
		t.Fatalf("enricher saw %d rows, want 1 — a FETCH-origin RowStore entry must reach the real Wave-2 enricher's input just like Probe/Disk origin does (fetch-origin blind spot)", len(seenRows))
	}
	if seenRows[0].ID != fetchRow.ID {
		t.Errorf("enricher saw row ID %q, want %q", seenRows[0].ID, fetchRow.ID)
	}
	if seenRows[0].Fields["engine"] != "postgres" {
		t.Errorf("enricher saw row Fields[engine] = %q, want %q — fetch-origin row content must pass through unchanged", seenRows[0].Fields["engine"], "postgres")
	}

	if result.Issues != 1 {
		t.Errorf("ProbeEnrichmentResult.Issues = %d, want 1 — the enricher DID run and DID emit a finding once it received the fetch-origin row", result.Issues)
	}
	findings, ok := result.Findings[fetchRow.ID]
	if !ok {
		t.Fatalf("ProbeEnrichmentResult.Findings missing entry for %q", fetchRow.ID)
	}
	finding := findings[0]
	if finding.Code != "fetch-origin-pin-finding" {
		t.Errorf("Findings[%q].Code = %q, want %q", fetchRow.ID, finding.Code, "fetch-origin-pin-finding")
	}
}

// TestEnrichmentChecked_FetchOriginFindings_SurviveToCompletionSavePayload
// extends the primary pin through the completion flow: once the real
// enricher (driven via ProbeEnrichment above) has produced findings for a
// fetch-origin type, the sweep-completion save dispatched by
// Core.Handle(messages.EnrichmentChecked) must carry those findings on the
// type's rows — mirroring the DEF-7 pin's payload-inspection pattern
// (TestStage2Pin_DEF7_SavePayloadFrozenAtDispatch_SurvivesLaterAmend) but for
// a fetch-origin-seeded type rather than a probe-seeded one.
//
// Pre-fix: the sweep never sees any rows for the fetch-origin type in the
// first place (see the primary pin above), so this would trivially "pass"
// with zero findings for the wrong reason; this test's own precondition
// check (EnrichTotal/queue membership) and its independent seenRows
// assertion on the enricher closure make sure the failure mode is visible
// end-to-end, not just muted by an empty completion payload.
func TestEnrichmentChecked_FetchOriginFindings_SurviveToCompletionSavePayload(t *testing.T) {
	const sentinelType = "dbi-fetch-origin-completion-pin"

	s, core, ctrl := newFetchOriginPinSession(t)

	fetchRow := resource.Resource{
		ID:   "rds:fetch-origin-completion-1",
		Name: "fetch-origin-completion-1",
		Type: sentinelType,
	}
	s.RowStore.Observe(sentinelType, []resource.Resource{fetchRow}, nil, session.OriginFetch, false)

	var seenRows []resource.Resource
	captureFn := func(_ context.Context, _ *awsclient.ServiceClients, rows []resource.Resource, _ resource.ResourceCache) (awsclient.IssueEnricherResult, error) {
		seenRows = rows
		return awsclient.IssueEnricherResult{
			IssueCount: 1,
			Findings: map[string][]domain.Finding{
				fetchRow.ID: {{
					Code:     "fetch-origin-completion-finding",
					Phrase:   "fetch-origin completion pin finding",
					Severity: domain.SevWarn,
					Source:   "wave2:" + sentinelType,
				}},
			},
		}, nil
	}
	awsclient.SetWave2EnricherForTest(t, sentinelType, awsclient.IssueEnricher{Fn: captureFn, Priority: 100})

	result := core.ProbeEnrichment(context.Background(), s.Clients, sentinelType)
	if len(seenRows) != 1 {
		t.Fatalf("precondition: enricher saw %d rows, want 1 (fetch-origin row must reach the enricher before the completion flow can be meaningfully tested)", len(seenRows))
	}
	if result.Err != nil {
		t.Fatalf("precondition: ProbeEnrichment returned unexpected error: %v", result.Err)
	}

	// Manually enqueue this type as the sole entry in the enrichment queue —
	// startEnrichment's own BuildEnrichQueue membership rule is exercised
	// elsewhere (enrich_queue_test.go); this test's concern is what happens
	// to the ProbeEnrichment RESULT once it lands via EnrichmentChecked, not
	// queue construction.
	s.EnrichmentGen++
	s.EnrichChecked = 0
	s.EnrichTotal = 1
	if s.EnrichmentTypeGen == nil {
		s.EnrichmentTypeGen = map[string]domain.Gen{}
	}
	s.EnrichmentTypeGen[sentinelType]++

	_, tasks := ctrl.Handle(messages.EnrichmentChecked{
		ResourceType:     sentinelType,
		Issues:           result.Issues,
		Findings:         runtime.WorstFindingPerID(result.Findings),
		AllFindings:      result.Findings,
		AttentionDetails: result.AttentionDetails,
		FieldUpdates:     result.FieldUpdates,
		TruncatedIDs:     result.TruncatedIDs,
		Gen:              s.EnrichmentGen,
		TypeGen:          s.EnrichmentTypeGen[sentinelType],
	})

	var payload *runtime.SaveCachePayload
	for _, tr := range tasks {
		if tr.Key.Kind == runtime.TaskKindSaveCache {
			p, ok := tr.Payload.(*runtime.SaveCachePayload)
			if !ok {
				t.Fatalf("TaskKindSaveCache payload type = %T, want *runtime.SaveCachePayload", tr.Payload)
			}
			payload = p
		}
	}
	if payload == nil {
		t.Fatal("no TaskKindSaveCache task dispatched by the sweep-completion EnrichmentChecked — precondition for the completion-save assertion failed")
	}

	savedRows, ok := payload.Resources[sentinelType]
	if !ok || len(savedRows) != 1 {
		t.Fatalf("completion-save payload.Resources[%q] = %+v, want exactly 1 row", sentinelType, savedRows)
	}
	if len(savedRows[0].Findings) == 0 {
		t.Fatalf("completion-save payload row Findings is empty, want the fetch-origin enricher's finding to have been folded onto the saved row")
	}
	var finding domain.Finding
	var found bool
	for _, f := range savedRows[0].Findings {
		if f.Code == domain.FindingCode("fetch-origin-completion-finding") {
			finding = f
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("completion-save payload row Findings = %+v, missing code %q", savedRows[0].Findings, "fetch-origin-completion-finding")
	}
	if finding.Phrase != "fetch-origin completion pin finding" {
		t.Errorf("completion-save payload finding Phrase = %q, want %q", finding.Phrase, "fetch-origin completion pin finding")
	}
}

// TestProbeEnrichment_PartialOnlyRows_ExcludedFromEnricherInput is the
// companion negative the dispatch calls for: a Partial-only entry (sparse
// FetchByIDs lazy-add, never a canonical Observe/ObserveFetch/disk seed)
// must remain excluded from the enricher's input — the fix widening the
// origin gate to include OriginFetch must not also drop the pre-existing
// tr.Partial exclusion.
func TestProbeEnrichment_PartialOnlyRows_ExcludedFromEnricherInput(t *testing.T) {
	const sentinelType = "dbi-partial-only-enrich-pin"

	s, core, _ := newFetchOriginPinSession(t)

	partialRow := resource.Resource{
		ID:   "rds:partial-only-1",
		Name: "partial-only-1",
		Type: sentinelType,
	}
	s.RowStore.ObservePartial(sentinelType, []resource.Resource{partialRow})

	var (
		invoked  bool
		seenRows []resource.Resource
	)
	captureFn := func(_ context.Context, _ *awsclient.ServiceClients, rows []resource.Resource, _ resource.ResourceCache) (awsclient.IssueEnricherResult, error) {
		invoked = true
		seenRows = rows
		return awsclient.IssueEnricherResult{}, nil
	}
	awsclient.SetWave2EnricherForTest(t, sentinelType, awsclient.IssueEnricher{Fn: captureFn, Priority: 100})

	result := core.ProbeEnrichment(context.Background(), s.Clients, sentinelType)
	if result.Err != nil {
		t.Fatalf("ProbeEnrichment returned unexpected error: %v", result.Err)
	}

	if !invoked {
		t.Fatal("enricher was never invoked — ProbeEnrichment should still call the registered enricher even with an excluded (nil/empty) resources argument, matching production behavior for a never-observed type")
	}
	if len(seenRows) != 0 {
		t.Errorf("enricher saw %d rows for a Partial-only entry, want 0 — a sparse lazy-add (ObservePartial) must remain excluded from the enricher's input even after widening the origin gate to include OriginFetch", len(seenRows))
	}
}
