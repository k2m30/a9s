// phase03_shim_wireups_test.go — TDD red-light tests for PR-03a-shim wire-up sites.
//
// These tests drive the root TUI model through each message-handler entry point
// that the spec (docs/refactor/03-finding-model.md lines 141-147) identifies as
// a site where attention.DeriveFindings must be called. They assert that after
// the handler processes the trigger message, the resources stored in the relevant
// in-memory cache have their Findings field populated.
//
// ALL tests MUST FAIL before the shim is wired in (red light) and PASS after
// (green light). Run with:
//
//	go test ./tests/unit/ -count=1 -run TestShim_ -v
//
// ── Wire-up sites covered by this file ─────────────────────────────────────────
//
//	#2 app_handlers_availability.go (~line 215) — AvailabilityCheckedMsg → ProbeResources
//	#3 app_handlers_availability.go (~line 340) — EnrichmentCheckedMsg → ResourceCache walk
//	#4 app.go (~line 489)                       — RelatedCheckResultMsg.CachedPages → ResourceCache
//	#5 app.go (~line 517)                       — RelatedCheckResultMsg.LazyAddedResources → LazyResourceCache
//
// ── Wire-up sites NOT covered by this file ─────────────────────────────────────
//
//	#1 app_fetchers.go  — ResourcesLoadedMsg → ResourceCache write-through.
//	   Reason: the write-through path runs inside updateActiveView which requires
//	   a live ResourceListModel on the view stack populated by a real fetch round-trip.
//	   Wiring this as a TDD test requires driving the full fetcher stack; the
//	   coder will verify via the grep-audit exit criterion (exactly 7 call sites).
//
//	#6 app_handlers_navigate.go — child-view fetcher path (EnterChildViewMsg → fetchChildResources).
//	   Reason: the child fetcher is dispatched asynchronously and returns a command;
//	   simulating the full round-trip requires an AWS client mock with non-trivial
//	   scaffolding. Covered by grep-audit.
//
//	#7 app_enrich.go — EnrichDetailResultMsg updates only the active DetailModel's
//	   internal m.res field, not m.ResourceCache. That field is unexported and
//	   inaccessible from this external test package. The wire-up will be verified
//	   by the coder's grep-audit (exactly 7 sites).
//
// ── Note on the EnrichmentChecked test (site #3) ───────────────────────────────
//
//	The critical Wave-2 bridge property is: the shim must be called AFTER
//	m.EnrichmentFindings[type] is updated (not before), so the second call to
//	DeriveFindings sees the wave2 enrichment map and appends the wave2 Finding
//	to the wave1 Findings already on the cached resource. The test seeds the
//	cache with a resource that has Status "impaired" (→ wave1 Finding pre-seeded
//	via an earlier DeriveFindings call), then sends EnrichmentCheckedMsg carrying
//	a wave2 finding for the same resource ID. After the handler the resource must
//	have 2 Findings (wave1 + wave2).
package unit_test

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/internal/domain"
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/semantics/attention"
	"github.com/k2m30/a9s/v3/internal/session"
	"github.com/k2m30/a9s/v3/internal/tui"
	"github.com/k2m30/a9s/v3/internal/tui/messages"
)

// shimApplyMsg applies a message to the TUI model and returns the updated model.
// Mirrors the applyMsg helper defined in TestHandleRefresh_SESDetailViewInvalidatesRuleSetCache.
func shimApplyMsg(m tui.Model, msg tea.Msg) tui.Model {
	newM, _ := m.Update(msg)
	return newM.(tui.Model)
}

// newShimModel builds a minimal root model suitable for shim wire-up tests.
// It applies a WindowSizeMsg and a ClientsReadyMsg with nil clients, which is
// enough to advance the model past the initial state without triggering live AWS calls.
func newShimModel() tui.Model {
	m := tui.New("test-profile", "us-east-1",
		tui.WithNoCache(true),
	)
	m = shimApplyMsg(m, tea.WindowSizeMsg{Width: 120, Height: 40})
	// ClientsReadyMsg with nil clients advances m.clients; no AWS calls are made
	// because WithNoCache(true) suppresses background availability probes.
	m = shimApplyMsg(m, messages.ClientsReadyMsg{Clients: nil})
	return m
}

// TestShim_ProbeResourcesPopulatesFindings verifies that when
// AvailabilityCheckedMsg is handled the retained resources in m.ProbeResources
// have their Findings field populated by DeriveFindings.
//
// Wire-up site: app_handlers_availability.go (~line 215), the
// m.ProbeResources[msg.ResourceType] = msg.Resources assignment.
//
// Red-light expectation: the handler currently stores resources verbatim;
// DeriveFindings is not called, so r.Findings remains nil.
func TestShim_ProbeResourcesPopulatesFindings(t *testing.T) {
	m := newShimModel()

	// Build a resource with a non-healthy Status that DeriveFindings will translate
	// into a wave1 Finding with Phrase "impaired".
	res := resource.Resource{
		ID:     "i-probe001",
		Name:   "test-instance",
		Status: "impaired",
	}

	// Send an AvailabilityCheckedMsg with Gen=0 (bypasses the gen guard because
	// m.AvailabilityGen is 0 in a freshly constructed model). Resources carries
	// the impaired instance.
	m = shimApplyMsg(m, messages.AvailabilityCheckedMsg{
		ResourceType: "ec2",
		HasResources: true,
		Count:        1,
		Truncated:    false,
		Gen:          0, // test-injection bypass
		Issues:       1,
		Resources:    []resource.Resource{res},
	})

	// After handling, m.ProbeResources["ec2"] should carry the retained resource
	// with Findings populated by the shim.
	probeSlice, ok := m.ProbeResources["ec2"]
	if !ok || len(probeSlice) == 0 {
		t.Fatal("ProbeResources[\"ec2\"] is empty — handler did not retain resources")
	}

	got := probeSlice[0]
	if len(got.Findings) == 0 {
		t.Errorf(
			"ProbeResources[\"ec2\"][0].Findings: got empty — "+
				"expected wave1 Finding with Phrase %q from Status %q; "+
				"shim not yet wired into AvailabilityCheckedMsg handler",
			"impaired", "impaired",
		)
		return
	}

	// Validate the Finding content once the shim is wired.
	f := got.Findings[0]
	if f.Phrase != "impaired" {
		t.Errorf("Findings[0].Phrase: got %q, want %q", f.Phrase, "impaired")
	}
	if f.Source != "wave1" {
		t.Errorf("Findings[0].Source: got %q, want %q", f.Source, "wave1")
	}
	if f.Code != "ec2.impaired" {
		t.Errorf("Findings[0].Code: got %q, want %q", f.Code, "ec2.impaired")
	}
}

// TestShim_EnrichmentCheckedBridgesWave2Findings is the critical Wave-2 bridge test.
//
// The test pre-seeds m.ResourceCache["ec2"] with a resource that already has
// wave1 Findings (derived from Status "impaired"), then sends EnrichmentCheckedMsg
// carrying a wave2 finding for the same resource ID.
//
// After the handler, the resource in cache must have 2 Findings (wave1 + wave2),
// proving that the shim re-derives AFTER m.EnrichmentFindings is written — the
// deterministic/no-early-return property is load-bearing here.
//
// Wire-up site: app_handlers_availability.go (~line 440), the ResourceCache walk
// that runs after m.EnrichmentFindings[msg.ResourceType] = msg.Findings.
//
// Red-light expectation: the handler currently does not walk the cache for shim
// re-derivation; the resource's Findings stay at their initial value (wave1 only,
// or empty if the cache was seeded without calling DeriveFindings first).
func TestShim_EnrichmentCheckedBridgesWave2Findings(t *testing.T) {
	m := newShimModel()

	// Build the ec2 resource with wave1 findings pre-derived.
	res := resource.Resource{
		ID:     "i-wave2001",
		Name:   "wave2-test-instance",
		Status: "impaired",
	}
	// Pre-derive wave1 findings so the cache entry starts with 1 Finding.
	td := resource.ResourceTypeDef{ShortName: "ec2"}
	attention.DeriveFindings(&res, td, nil)
	if len(res.Findings) != 1 {
		t.Fatalf("test setup: DeriveFindings produced %d findings, want 1", len(res.Findings))
	}

	// Seed m.ResourceCache["ec2"] with the pre-derived resource.
	m.ResourceCache["ec2"] = &session.ResourceCacheEntry{
		Resources: []resource.Resource{res},
	}

	// Build a wave2 enrichment finding for the same resource.
	wave2Finding := resource.EnrichmentFinding{
		Severity: "!",
		Summary:  "pending maintenance",
		Rows: []resource.FindingRow{
			{Label: "Action", Value: "reboot"},
		},
	}

	// Send EnrichmentCheckedMsg with Gen=0 and TypeGen=0 (both bypass gen guards).
	m = shimApplyMsg(m, messages.EnrichmentCheckedMsg{
		ResourceType: "ec2",
		Issues:       1,
		Truncated:    false,
		Findings: map[string]resource.EnrichmentFinding{
			"i-wave2001": wave2Finding,
		},
		Gen:     0, // session-wide gen bypass
		TypeGen: 0, // per-type gen bypass
	})

	// Inspect the cached resource.
	entry, ok := m.ResourceCache["ec2"]
	if !ok || len(entry.Resources) == 0 {
		t.Fatal("ResourceCache[\"ec2\"] is empty after EnrichmentCheckedMsg")
	}

	got := entry.Resources[0]
	if len(got.Findings) < 2 {
		t.Errorf(
			"ResourceCache[\"ec2\"].Resources[0].Findings: got %d finding(s), want 2 (wave1 + wave2) — "+
				"shim not yet wired to re-derive findings after EnrichmentFindings update",
			len(got.Findings),
		)
		return
	}

	// Validate wave1 Finding is still present at index 0.
	wave1 := got.Findings[0]
	if wave1.Source != "wave1" {
		t.Errorf("Findings[0].Source: got %q, want %q", wave1.Source, "wave1")
	}
	if wave1.Phrase != "impaired" {
		t.Errorf("Findings[0].Phrase: got %q, want %q", wave1.Phrase, "impaired")
	}

	// Validate wave2 Finding appears at index 1.
	wave2 := got.Findings[1]
	if wave2.Source != "wave2:ec2" {
		t.Errorf("Findings[1].Source: got %q, want %q", wave2.Source, "wave2:ec2")
	}
	if wave2.Phrase != "pending maintenance" {
		t.Errorf("Findings[1].Phrase: got %q, want %q", wave2.Phrase, "pending maintenance")
	}
	if wave2.Severity != domain.SevBroken {
		t.Errorf("Findings[1].Severity: got %v, want SevBroken", wave2.Severity)
	}
}

// TestShim_CachedPagesPopulatesFindings verifies that when RelatedCheckResultMsg
// carries a CachedPages entry, the resources written to m.ResourceCache have
// their Findings field populated by DeriveFindings.
//
// Wire-up site: app.go (~line 489), the m.ResourceCache[shortName] = ... assignment
// inside the CachedPages loop.
//
// Red-light expectation: the handler currently writes resources verbatim;
// DeriveFindings is not called, so Findings remains nil.
func TestShim_CachedPagesPopulatesFindings(t *testing.T) {
	m := newShimModel()

	// Build a resource with a non-healthy status.
	res := resource.Resource{
		ID:     "i-cached001",
		Name:   "cached-test-instance",
		Status: "impaired",
	}

	// Construct a RelatedCheckResultMsg that carries the resource as a CachedPages entry.
	// Generation=0 bypasses the relatedGen guard.
	m = shimApplyMsg(m, messages.RelatedCheckResultMsg{
		ResourceType:     "ec2",
		SourceResourceID: "",
		DefDisplayName:   "EC2 Instances",
		Result:           resource.RelatedCheckResult{TargetType: "ec2", Count: 1},
		Generation:       0, // test-injection bypass
		CachedPages: map[string]resource.ResourceCacheEntry{
			"ec2": {
				Resources:   []resource.Resource{res},
				IsTruncated: false,
			},
		},
	})

	// After handling, m.ResourceCache["ec2"] must exist with Findings populated.
	entry, ok := m.ResourceCache["ec2"]
	if !ok || len(entry.Resources) == 0 {
		t.Fatal("ResourceCache[\"ec2\"] is empty — CachedPages was not written")
	}

	got := entry.Resources[0]
	if len(got.Findings) == 0 {
		t.Errorf(
			"ResourceCache[\"ec2\"].Resources[0].Findings: got empty — "+
				"expected wave1 Finding with Phrase %q; "+
				"shim not yet wired into CachedPages write path",
			"impaired",
		)
		return
	}

	f := got.Findings[0]
	if f.Phrase != "impaired" {
		t.Errorf("Findings[0].Phrase: got %q, want %q", f.Phrase, "impaired")
	}
	if f.Source != "wave1" {
		t.Errorf("Findings[0].Source: got %q, want %q", f.Source, "wave1")
	}
	if f.Code != "ec2.impaired" {
		t.Errorf("Findings[0].Code: got %q, want %q", f.Code, "ec2.impaired")
	}
}

// TestShim_LazyAddedPopulatesFindings verifies that when RelatedCheckResultMsg
// carries a LazyAddedResources entry, the resources merged into m.LazyResourceCache
// have their Findings field populated by DeriveFindings.
//
// Wire-up site: app.go (~line 517), the m.LazyResourceCache[shortName] = existing
// assignment inside the LazyAddedResources loop.
//
// Red-light expectation: the handler currently merges resources verbatim;
// DeriveFindings is not called, so Findings remains nil.
func TestShim_LazyAddedPopulatesFindings(t *testing.T) {
	m := newShimModel()

	// Build a resource with an issue status.
	res := resource.Resource{
		ID:     "kms-lazy001",
		Name:   "customer-managed-key",
		Status: "pending deletion",
	}

	// Construct a RelatedCheckResultMsg that carries the resource in LazyAddedResources.
	// Generation=0 bypasses the relatedGen guard.
	m = shimApplyMsg(m, messages.RelatedCheckResultMsg{
		ResourceType:     "kms",
		SourceResourceID: "",
		DefDisplayName:   "KMS Keys",
		Result:           resource.RelatedCheckResult{TargetType: "kms", Count: 1},
		Generation:       0, // test-injection bypass
		LazyAddedResources: map[string][]resource.Resource{
			"kms": {res},
		},
	})

	// After handling, m.LazyResourceCache["kms"] must exist with Findings populated.
	lazySlice, ok := m.LazyResourceCache["kms"]
	if !ok || len(lazySlice) == 0 {
		t.Fatal("LazyResourceCache[\"kms\"] is empty — LazyAddedResources was not written")
	}

	got := lazySlice[0]
	if len(got.Findings) == 0 {
		t.Errorf(
			"LazyResourceCache[\"kms\"][0].Findings: got empty — "+
				"expected wave1 Finding with Phrase %q; "+
				"shim not yet wired into LazyAddedResources write path",
			"pending deletion",
		)
		return
	}

	f := got.Findings[0]
	if f.Phrase != "pending deletion" {
		t.Errorf("Findings[0].Phrase: got %q, want %q", f.Phrase, "pending deletion")
	}
	if f.Source != "wave1" {
		t.Errorf("Findings[0].Source: got %q, want %q", f.Source, "wave1")
	}
	// slug("pending deletion") → "pending.deletion"; type short name is "kms"
	if f.Code != "kms.pending.deletion" {
		t.Errorf("Findings[0].Code: got %q, want %q", f.Code, "kms.pending.deletion")
	}
}
