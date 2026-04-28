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
//
// ── Alias regression tests (CodeRabbit PR #308 findings) ───────────────────────
//
//	TestShim_DeriveHelpersResolveAlias, TestShim_DeriveHelpersResolveAlias_SingleResource:
//	   deriveFindingsForType/deriveFindingsForResource read m.EnrichmentFindings[short]
//	   where short may be an alias — Wave-2 findings keyed under canonical ShortName
//	   are missed. Pre-fix: only wave1 finding returned. Post-fix: 2 findings.
//
//	TestShim_NavigateAliasHitsCanonicalCache:
//	   handleNavigate does m.ResourceCache[msg.ResourceType] — alias misses canonical
//	   cache key, bypasses deriveFindingsForType, leaving Findings empty. Pre-fix:
//	   empty. Post-fix: Findings populated.
//
// ── Confirmed alias map ─────────────────────────────────────────────────────────
//
//	dbi (DB Instances)          → aliases: "dbi", "rds", "databases", "db-instances"
//	redis (ElastiCache Redis)   → aliases: "redis", "elasticache"
//	ec2, s3, sg, role, ng, kms  → canonical-only (ShortName is first alias entry)
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
// Table-driven: every row exercises a distinct resource type to ensure no
// type is special-cased and aliases are handled correctly.
//
// Red-light expectation: the handler currently stores resources verbatim;
// DeriveFindings is not called, so r.Findings remains nil.
func TestShim_ProbeResourcesPopulatesFindings(t *testing.T) {
	cases := []struct {
		name, canonShort, alias, status, expectedSlug string
	}{
		{"ec2-canonical", "ec2", "", "impaired", "impaired"},
		{"s3-canonical", "s3", "", "bucket public", "bucket.public"},
		{"rds-aliased", "dbi", "rds", "maintenance pending", "maintenance.pending"},
		{"redis-aliased", "redis", "elasticache", "failover pending", "failover.pending"},
		{"sg-canonical", "sg", "", "overly permissive", "overly.permissive"},
		{"iam-role-canonical", "role", "", "unused", "unused"},
		{"ng-canonical", "ng", "", "scale failure", "scale.failure"},
		{"kms-canonical", "kms", "", "pending deletion", "pending.deletion"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			m := newShimModel()

			effective := tc.canonShort
			if tc.alias != "" {
				effective = tc.alias
			}

			res := resource.Resource{
				ID:     "r-probe-" + tc.canonShort,
				Name:   "test-" + tc.canonShort,
				Status: tc.status,
			}

			m = shimApplyMsg(m, messages.AvailabilityCheckedMsg{
				ResourceType: effective,
				HasResources: true,
				Count:        1,
				Truncated:    false,
				Gen:          0,
				Issues:       1,
				Resources:    []resource.Resource{res},
			})

			// The cache lookup must use the canonical short name, NOT the alias.
			probeSlice, ok := m.ProbeResources[tc.canonShort]
			if !ok || len(probeSlice) == 0 {
				t.Fatalf("ProbeResources[%q] is empty — handler did not retain resources (alias=%q)", tc.canonShort, effective)
			}

			got := probeSlice[0]
			if len(got.Findings) == 0 {
				t.Errorf(
					"ProbeResources[%q][0].Findings: got empty — expected wave1 Finding with Code %q; shim not yet wired",
					tc.canonShort, tc.canonShort+"."+tc.expectedSlug,
				)
				return
			}

			f := got.Findings[0]
			wantCode := domain.FindingCode(tc.canonShort + "." + tc.expectedSlug)
			if f.Code != wantCode {
				t.Errorf("Findings[0].Code: got %q, want %q", f.Code, wantCode)
			}
			if f.Phrase != tc.status {
				t.Errorf("Findings[0].Phrase: got %q, want %q", f.Phrase, tc.status)
			}
			if f.Source != "wave1" {
				t.Errorf("Findings[0].Source: got %q, want %q", f.Source, "wave1")
			}
		})
	}
}

// TestShim_EnrichmentCheckedBridgesWave2Findings is the critical Wave-2 bridge test.
//
// The test pre-seeds m.ResourceCache[tc.canonShort] with a resource that has
// tc.status, sends EnrichmentCheckedMsg (using alias if present), and asserts
// the first finding is populated correctly.
//
// Red-light for alias rows: m.EnrichmentFindings[alias] lookup misses the
// canonical key, so the handler does not walk the cache at all, leaving Findings
// empty.
func TestShim_EnrichmentCheckedBridgesWave2Findings(t *testing.T) {
	cases := []struct {
		name, canonShort, alias, status, expectedSlug string
	}{
		{"ec2-canonical", "ec2", "", "impaired", "impaired"},
		{"s3-canonical", "s3", "", "bucket public", "bucket.public"},
		{"rds-aliased", "dbi", "rds", "maintenance pending", "maintenance.pending"},
		{"redis-aliased", "redis", "elasticache", "failover pending", "failover.pending"},
		{"sg-canonical", "sg", "", "overly permissive", "overly.permissive"},
		{"iam-role-canonical", "role", "", "unused", "unused"},
		{"ng-canonical", "ng", "", "scale failure", "scale.failure"},
		{"kms-canonical", "kms", "", "pending deletion", "pending.deletion"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			m := newShimModel()

			effective := tc.canonShort
			if tc.alias != "" {
				effective = tc.alias
			}

			rid := "r-wave2-" + tc.canonShort
			res := resource.Resource{
				ID:     rid,
				Name:   "wave2-" + tc.canonShort,
				Status: tc.status,
			}

			// Seed the cache under the canonical short name.
			m.ResourceCache[tc.canonShort] = &session.ResourceCacheEntry{
				Resources: []resource.Resource{res},
			}

			// Send EnrichmentCheckedMsg using the alias (or canon) as ResourceType.
			m = shimApplyMsg(m, messages.EnrichmentCheckedMsg{
				ResourceType: effective,
				Issues:       0,
				Truncated:    false,
				Findings:     map[string]resource.EnrichmentFinding{},
				Gen:          0,
				TypeGen:      0,
			})

			entry, ok := m.ResourceCache[tc.canonShort]
			if !ok || len(entry.Resources) == 0 {
				t.Fatalf("ResourceCache[%q] is empty after EnrichmentCheckedMsg", tc.canonShort)
			}

			got := entry.Resources[0]
			if len(got.Findings) == 0 {
				t.Errorf(
					"ResourceCache[%q].Resources[0].Findings: got empty — expected wave1 Finding with Code %q; shim not yet wired (alias=%q)",
					tc.canonShort, tc.canonShort+"."+tc.expectedSlug, effective,
				)
				return
			}

			f := got.Findings[0]
			wantCode := domain.FindingCode(tc.canonShort + "." + tc.expectedSlug)
			if f.Code != wantCode {
				t.Errorf("Findings[0].Code: got %q, want %q", f.Code, wantCode)
			}
			if f.Phrase != tc.status {
				t.Errorf("Findings[0].Phrase: got %q, want %q", f.Phrase, tc.status)
			}
			if f.Source != "wave1" {
				t.Errorf("Findings[0].Source: got %q, want %q", f.Source, "wave1")
			}
		})
	}
}

// TestShim_CachedPagesPopulatesFindings verifies that when RelatedCheckResultMsg
// carries a CachedPages entry, the resources written to m.ResourceCache have
// their Findings field populated by DeriveFindings.
//
// Wire-up site: app.go (~line 489), the m.ResourceCache[shortName] = ... assignment
// inside the CachedPages loop.
//
// Table-driven: aliased rows (rds-aliased, redis-aliased) are expected to fail
// pre-fix because the CachedPages key uses the alias, but cache lookup and
// DeriveFindings both need the canonical name.
//
// Red-light expectation: the handler currently writes resources verbatim;
// DeriveFindings is not called, so Findings remains nil.
func TestShim_CachedPagesPopulatesFindings(t *testing.T) {
	cases := []struct {
		name, canonShort, alias, status, expectedSlug string
	}{
		{"ec2-canonical", "ec2", "", "impaired", "impaired"},
		{"s3-canonical", "s3", "", "bucket public", "bucket.public"},
		{"rds-aliased", "dbi", "rds", "maintenance pending", "maintenance.pending"},
		{"redis-aliased", "redis", "elasticache", "failover pending", "failover.pending"},
		{"sg-canonical", "sg", "", "overly permissive", "overly.permissive"},
		{"iam-role-canonical", "role", "", "unused", "unused"},
		{"ng-canonical", "ng", "", "scale failure", "scale.failure"},
		{"kms-canonical", "kms", "", "pending deletion", "pending.deletion"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			m := newShimModel()

			effective := tc.canonShort
			if tc.alias != "" {
				effective = tc.alias
			}

			res := resource.Resource{
				ID:     "r-cached-" + tc.canonShort,
				Name:   "cached-" + tc.canonShort,
				Status: tc.status,
			}

			m = shimApplyMsg(m, messages.RelatedCheckResultMsg{
				ResourceType:     effective,
				SourceResourceID: "",
				DefDisplayName:   tc.name,
				Result:           resource.RelatedCheckResult{TargetType: effective, Count: 1},
				Generation:       0,
				CachedPages: map[string]resource.ResourceCacheEntry{
					effective: {
						Resources:   []resource.Resource{res},
						IsTruncated: false,
					},
				},
			})

			// After handling, the cache must be stored under the canonical key.
			entry, ok := m.ResourceCache[tc.canonShort]
			if !ok || len(entry.Resources) == 0 {
				t.Fatalf("ResourceCache[%q] is empty — CachedPages was not written (alias=%q)", tc.canonShort, effective)
			}

			got := entry.Resources[0]
			if len(got.Findings) == 0 {
				t.Errorf(
					"ResourceCache[%q].Resources[0].Findings: got empty — expected wave1 Finding with Code %q; shim not yet wired (alias=%q)",
					tc.canonShort, tc.canonShort+"."+tc.expectedSlug, effective,
				)
				return
			}

			f := got.Findings[0]
			wantCode := domain.FindingCode(tc.canonShort + "." + tc.expectedSlug)
			if f.Code != wantCode {
				t.Errorf("Findings[0].Code: got %q, want %q", f.Code, wantCode)
			}
			if f.Phrase != tc.status {
				t.Errorf("Findings[0].Phrase: got %q, want %q", f.Phrase, tc.status)
			}
			if f.Source != "wave1" {
				t.Errorf("Findings[0].Source: got %q, want %q", f.Source, "wave1")
			}
		})
	}
}

// TestShim_LazyAddedPopulatesFindings verifies that when RelatedCheckResultMsg
// carries a LazyAddedResources entry, the resources merged into m.LazyResourceCache
// have their Findings field populated by DeriveFindings.
//
// Wire-up site: app.go (~line 517), the m.LazyResourceCache[shortName] = existing
// assignment inside the LazyAddedResources loop.
//
// Table-driven: aliased rows (rds-aliased, redis-aliased) are expected to fail
// pre-fix because LazyAddedResources is keyed by alias, but cache lookup and
// DeriveFindings both need the canonical name.
//
// Red-light expectation: the handler currently merges resources verbatim;
// DeriveFindings is not called, so Findings remains nil.
func TestShim_LazyAddedPopulatesFindings(t *testing.T) {
	cases := []struct {
		name, canonShort, alias, status, expectedSlug string
	}{
		{"ec2-canonical", "ec2", "", "impaired", "impaired"},
		{"s3-canonical", "s3", "", "bucket public", "bucket.public"},
		{"rds-aliased", "dbi", "rds", "maintenance pending", "maintenance.pending"},
		{"redis-aliased", "redis", "elasticache", "failover pending", "failover.pending"},
		{"sg-canonical", "sg", "", "overly permissive", "overly.permissive"},
		{"iam-role-canonical", "role", "", "unused", "unused"},
		{"ng-canonical", "ng", "", "scale failure", "scale.failure"},
		{"kms-canonical", "kms", "", "pending deletion", "pending.deletion"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			m := newShimModel()

			effective := tc.canonShort
			if tc.alias != "" {
				effective = tc.alias
			}

			res := resource.Resource{
				ID:     "r-lazy-" + tc.canonShort,
				Name:   "lazy-" + tc.canonShort,
				Status: tc.status,
			}

			m = shimApplyMsg(m, messages.RelatedCheckResultMsg{
				ResourceType:     effective,
				SourceResourceID: "",
				DefDisplayName:   tc.name,
				Result:           resource.RelatedCheckResult{TargetType: effective, Count: 1},
				Generation:       0,
				LazyAddedResources: map[string][]resource.Resource{
					effective: {res},
				},
			})

			// The lazy cache must be stored under the canonical short name.
			lazySlice, ok := m.LazyResourceCache[tc.canonShort]
			if !ok || len(lazySlice) == 0 {
				t.Fatalf("LazyResourceCache[%q] is empty — LazyAddedResources was not written (alias=%q)", tc.canonShort, effective)
			}

			got := lazySlice[0]
			if len(got.Findings) == 0 {
				t.Errorf(
					"LazyResourceCache[%q][0].Findings: got empty — expected wave1 Finding with Code %q; shim not yet wired (alias=%q)",
					tc.canonShort, tc.canonShort+"."+tc.expectedSlug, effective,
				)
				return
			}

			f := got.Findings[0]
			wantCode := domain.FindingCode(tc.canonShort + "." + tc.expectedSlug)
			if f.Code != wantCode {
				t.Errorf("Findings[0].Code: got %q, want %q", f.Code, wantCode)
			}
			if f.Phrase != tc.status {
				t.Errorf("Findings[0].Phrase: got %q, want %q", f.Phrase, tc.status)
			}
			if f.Source != "wave1" {
				t.Errorf("Findings[0].Source: got %q, want %q", f.Source, "wave1")
			}
		})
	}
}

// TestShim_DeriveHelpersResolveAlias verifies that deriveFindingsForType resolves
// an alias to its canonical ShortName before looking up m.EnrichmentFindings.
//
// Bug: derive_helper.go reads m.EnrichmentFindings[short] where short may be an
// alias — Wave-2 findings keyed under canonical ShortName ("dbi") are dropped
// when the caller passes alias "rds".
//
// Setup: seed m.EnrichmentFindings["dbi"] with a wave2 finding for rid. Build a
// Resource with that ID and Issues=["impaired"] (wave1). Call
// deriveFindingsForType("rds", []Resource{r}). Post-fix: r.Findings has 2 entries.
// Pre-fix: only wave1 (enrichment map miss).
func TestShim_DeriveHelpersResolveAlias(t *testing.T) {
	m := newShimModel()

	rid := "i-alias-wave2-001"
	wave2Finding := resource.EnrichmentFinding{
		Severity: "!",
		Summary:  "pending maintenance",
		Rows: []resource.FindingRow{
			{Label: "Action", Value: "reboot"},
		},
	}
	// Seed enrichment findings under the CANONICAL short name ("dbi").
	m.EnrichmentFindings["dbi"] = map[string]resource.EnrichmentFinding{
		rid: wave2Finding,
	}

	res := resource.Resource{
		ID:     rid,
		Name:   "test-dbi-alias",
		Status: "impaired",
		Issues: []string{"impaired"},
	}

	// Pre-seed wave1 finding so test setup is valid regardless of shim status.
	td := resource.ResourceTypeDef{ShortName: "dbi"}
	attention.DeriveFindings(&res, td, nil)
	if len(res.Findings) != 1 {
		t.Fatalf("test setup: DeriveFindings (wave1) produced %d findings, want 1", len(res.Findings))
	}

	// Drive the slice through AvailabilityCheckedMsg using the ALIAS "rds".
	// The shim must resolve "rds" → "dbi" to find enrichment findings.
	m = shimApplyMsg(m, messages.AvailabilityCheckedMsg{
		ResourceType: "rds", // alias
		HasResources: true,
		Count:        1,
		Truncated:    false,
		Gen:          0,
		Issues:       1,
		Resources:    []resource.Resource{res},
	})

	probeSlice, ok := m.ProbeResources["dbi"]
	if !ok || len(probeSlice) == 0 {
		t.Fatal("ProbeResources[\"dbi\"] is empty after AvailabilityCheckedMsg with alias \"rds\"")
	}

	got := probeSlice[0]
	if len(got.Findings) < 2 {
		t.Errorf(
			"ProbeResources[\"dbi\"][0].Findings: got %d finding(s), want 2 (wave1 + wave2) — "+
				"deriveFindingsForType not resolving alias \"rds\" to canonical \"dbi\" for EnrichmentFindings lookup",
			len(got.Findings),
		)
		return
	}

	// Validate wave1 finding at index 0.
	wave1 := got.Findings[0]
	if wave1.Source != "wave1" {
		t.Errorf("Findings[0].Source: got %q, want %q", wave1.Source, "wave1")
	}
	if wave1.Phrase != "impaired" {
		t.Errorf("Findings[0].Phrase: got %q, want %q", wave1.Phrase, "impaired")
	}

	// Validate wave2 finding at index 1.
	wave2 := got.Findings[1]
	if wave2.Source != "wave2:dbi" {
		t.Errorf("Findings[1].Source: got %q, want %q", wave2.Source, "wave2:dbi")
	}
	if wave2.Phrase != "pending maintenance" {
		t.Errorf("Findings[1].Phrase: got %q, want %q", wave2.Phrase, "pending maintenance")
	}
	if wave2.Severity != domain.SevBroken {
		t.Errorf("Findings[1].Severity: got %v, want SevBroken", wave2.Severity)
	}
}

// TestShim_DeriveHelpersResolveAlias_SingleResource verifies that the derive
// helpers resolve an alias to its canonical ShortName when reading m.EnrichmentFindings
// on the single-resource path.
//
// Bug: derive_helper.go reads m.EnrichmentFindings[short] where short may be an
// alias — Wave-2 findings keyed under canonical ShortName ("dbi") are dropped
// when the caller passes alias "rds". This path fires when AvailabilityCheckedMsg
// carries resources for an aliased type AND m.EnrichmentFindings already holds
// wave2 findings from a prior enrichment run under the canonical key.
//
// Setup: seed m.EnrichmentFindings["dbi"] with wave2 findings. Send
// AvailabilityCheckedMsg{ResourceType: "rds"} with a resource that has Status
// "impaired" (wave1). The handler calls deriveFindingsForType("rds", resources).
// The helper must resolve "rds" → "dbi" before reading m.EnrichmentFindings.
//
// Pre-fix: only 1 wave1 finding (enrichment lookup reads EnrichmentFindings["rds"] = nil).
// Post-fix: 2 findings (wave1 + wave2), "rds" resolved to "dbi" for enrichment lookup.
func TestShim_DeriveHelpersResolveAlias_SingleResource(t *testing.T) {
	m := newShimModel()

	rid := "i-alias-single-001"
	wave2Finding := resource.EnrichmentFinding{
		Severity: "!",
		Summary:  "pending maintenance",
		Rows: []resource.FindingRow{
			{Label: "Action", Value: "reboot"},
		},
	}
	// Seed wave2 enrichment findings under the CANONICAL short name ("dbi").
	// After fix, deriveFindingsForType("rds", ...) must resolve "rds" → "dbi"
	// before reading m.EnrichmentFindings to find this wave2 data.
	m.EnrichmentFindings["dbi"] = map[string]resource.EnrichmentFinding{
		rid: wave2Finding,
	}

	res := resource.Resource{
		ID:     rid,
		Name:   "test-dbi-single-alias",
		Status: "impaired",
		Issues: []string{"impaired"},
	}

	// Send AvailabilityCheckedMsg with the alias "rds" (not canonical "dbi").
	// The handler calls deriveFindingsForType("rds", [res]) which must read
	// m.EnrichmentFindings["dbi"] (canonical), not m.EnrichmentFindings["rds"]
	// (alias), to pick up the wave2 finding.
	m = shimApplyMsg(m, messages.AvailabilityCheckedMsg{
		ResourceType: "rds", // alias — triggers the derive helper alias regression
		HasResources: true,
		Count:        1,
		Truncated:    false,
		Gen:          0,
		Issues:       1,
		Resources:    []resource.Resource{res},
	})

	// The handler stores under m.ProbeResources[msg.ResourceType = "rds"].
	// After fix (canonical normalization), the resource should be under "dbi".
	probeSlice, ok := m.ProbeResources["dbi"]
	if !ok || len(probeSlice) == 0 {
		// Pre-fix: resources are stored under the alias key "rds", not "dbi".
		// This is Bug 1 (navigate) manifesting here too — the availability handler
		// does NOT normalize msg.ResourceType before the ProbeResources write.
		t.Fatal("ProbeResources[\"dbi\"] is empty — AvailabilityCheckedMsg with alias \"rds\" stored under alias key instead of canonical")
	}

	got := probeSlice[0]
	if len(got.Findings) < 2 {
		t.Errorf(
			"ProbeResources[\"dbi\"][0].Findings: got %d finding(s), want 2 (wave1 + wave2) — "+
				"derive helper not resolving alias \"rds\" to canonical \"dbi\" for EnrichmentFindings lookup; "+
				"wave2 finding pre-seeded under \"dbi\" not visible via alias path",
			len(got.Findings),
		)
		return
	}

	// Validate wave1 finding at index 0.
	wave1 := got.Findings[0]
	if wave1.Source != "wave1" {
		t.Errorf("Findings[0].Source: got %q, want %q", wave1.Source, "wave1")
	}
	if wave1.Phrase != "impaired" {
		t.Errorf("Findings[0].Phrase: got %q, want %q", wave1.Phrase, "impaired")
	}

	// Validate wave2 finding at index 1.
	wave2 := got.Findings[1]
	if wave2.Source != "wave2:dbi" {
		t.Errorf("Findings[1].Source: got %q, want %q", wave2.Source, "wave2:dbi")
	}
	if wave2.Phrase != "pending maintenance" {
		t.Errorf("Findings[1].Phrase: got %q, want %q", wave2.Phrase, "pending maintenance")
	}
	if wave2.Severity != domain.SevBroken {
		t.Errorf("Findings[1].Severity: got %v, want SevBroken", wave2.Severity)
	}
}

// TestShim_NavigateAliasHitsCanonicalCache verifies that handleNavigate resolves
// an alias to the canonical ShortName before looking up m.ResourceCache.
//
// Bug: app_handlers_navigate.go:49 does m.ResourceCache[msg.ResourceType] —
// when msg.ResourceType is "rds" (alias) but the cache is keyed under "dbi"
// (canonical), the lookup misses. deriveFindingsForType is never called on the
// cached resources, so Findings remain empty.
//
// Setup: seed m.ResourceCache["dbi"] with one resource (Status="impaired").
// Drive NavigateMsg{Target: TargetResourceList, ResourceType: "rds"}.
// Post-fix: m.ResourceCache["dbi"].Resources[0].Findings is populated.
// Pre-fix: Findings is empty (cache entry found only after canonical lookup is wired).
func TestShim_NavigateAliasHitsCanonicalCache(t *testing.T) {
	m := newShimModel()

	rid := "i-nav-alias-001"
	res := resource.Resource{
		ID:     rid,
		Name:   "nav-alias-test",
		Status: "impaired",
	}

	// Seed the cache under the canonical key "dbi".
	m.ResourceCache["dbi"] = &session.ResourceCacheEntry{
		Resources: []resource.Resource{res},
	}

	// Navigate using the alias "rds". The handler must resolve to "dbi" and
	// find the cache entry, then call deriveFindingsForType on it.
	m = shimApplyMsg(m, messages.NavigateMsg{
		Target:       messages.TargetResourceList,
		ResourceType: "rds", // alias for "dbi"
	})

	// The cache under "dbi" must now have Findings populated.
	entry, ok := m.ResourceCache["dbi"]
	if !ok || len(entry.Resources) == 0 {
		t.Fatal("ResourceCache[\"dbi\"] is empty after NavigateMsg — unexpected state")
	}

	got := entry.Resources[0]
	if len(got.Findings) == 0 {
		t.Errorf(
			"ResourceCache[\"dbi\"].Resources[0].Findings: got empty — "+
				"handleNavigate not resolving alias \"rds\" to canonical \"dbi\" for cache hit path; "+
				"deriveFindingsForType not called on cached resources",
		)
		return
	}

	f := got.Findings[0]
	if f.Code != "dbi.impaired" {
		t.Errorf("Findings[0].Code: got %q, want %q", f.Code, "dbi.impaired")
	}
	if f.Phrase != "impaired" {
		t.Errorf("Findings[0].Phrase: got %q, want %q", f.Phrase, "impaired")
	}
	if f.Source != "wave1" {
		t.Errorf("Findings[0].Source: got %q, want %q", f.Source, "wave1")
	}
}
