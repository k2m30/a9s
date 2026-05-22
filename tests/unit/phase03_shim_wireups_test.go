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
//	   internal m.res field, not m.Core().Session().ResourceCache. That field is unexported and
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
//	   handleNavigate does m.Core().Session().ResourceCache[msg.ResourceType] — alias misses canonical
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
	"github.com/k2m30/a9s/v3/internal/session"
	"github.com/k2m30/a9s/v3/internal/tui"
	"github.com/k2m30/a9s/v3/internal/runtime/messages"
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
	m = shimApplyMsg(m, messages.ClientsReady{Clients: nil})
	return m
}

// TestShim_ProbeResourcesPopulatesFindings verifies that when
// AvailabilityCheckedMsg is handled the retained resources in m.Core().Session().ProbeResources
// have their Findings field populated by DeriveFindings.
//
// Wire-up site: app_handlers_availability.go (~line 215), the
// m.Core().Session().ProbeResources[msg.ResourceType] = msg.Resources assignment.
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

			m = shimApplyMsg(m, messages.AvailabilityChecked{
				ResourceType: effective,
				HasResources: true,
				Count:        1,
				Truncated:    false,
				// session.New seeds AvailabilityGen=1 (AS-659) — stamp the live
				// value so the AvailabilityChecked stale guard accepts it.
				Gen:       m.Core().Session().AvailabilityGen,
				Issues:    1,
				Resources: []resource.Resource{res},
			})

			// The cache lookup must use the canonical short name, NOT the alias.
			probeSlice, ok := m.Core().Session().ProbeResources[tc.canonShort]
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
// The test pre-seeds m.Core().Session().ResourceCache[tc.canonShort] with a resource that has
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
			m.Core().Session().ResourceCache[tc.canonShort] = &session.ResourceCacheEntry{
				Resources: []resource.Resource{res},
			}

			// Send EnrichmentCheckedMsg using the alias (or canon) as ResourceType.
			m = shimApplyMsg(m, messages.EnrichmentChecked{
				ResourceType: effective,
				Issues:       0,
				Truncated:    false,
				Findings:     map[string]resource.EnrichmentFinding{},
				Gen:          0,
				TypeGen:      0,
			})

			entry, ok := m.Core().Session().ResourceCache[tc.canonShort]
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
// carries a CachedPages entry, the resources written to m.Core().Session().ResourceCache have
// their Findings field populated by DeriveFindings.
//
// Wire-up site: app.go (~line 489), the m.Core().Session().ResourceCache[shortName] = ... assignment
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

			m = shimApplyMsg(m, messages.RelatedCheckResult{
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
			entry, ok := m.Core().Session().ResourceCache[tc.canonShort]
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
// carries a LazyAddedResources entry, the resources merged into m.Core().Session().LazyResourceCache
// have their Findings field populated by DeriveFindings.
//
// Wire-up site: app.go (~line 517), the m.Core().Session().LazyResourceCache[shortName] = existing
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

			m = shimApplyMsg(m, messages.RelatedCheckResult{
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
			lazySlice, ok := m.Core().Session().LazyResourceCache[tc.canonShort]
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

// TestShim_DeriveHelpersResolveAlias verifies that after applyEnrichment is called
// via EnrichmentCheckedMsg, ProbeResources rows for a canonically-keyed type carry
// both wave1 (from AvailabilityCheckedMsg) and wave2 (from EnrichmentCheckedMsg)
// findings.
//
// After PR-03a-fold: deriveFindingsForType no longer reads m.EnrichmentFindings.
// Instead, wave2 data flows through EnrichmentCheckedMsg → applyEnrichment which
// calls DeriveFindings with the full findings map on all cached rows. The alias
// resolution bug ("rds" → "dbi") is exercised through the EnrichmentCheckedMsg
// alias-normalization path in handleEnrichmentChecked.
//
// Setup: send AvailabilityCheckedMsg{ResourceType: "rds"} to populate
// ProbeResources["dbi"] with wave1. Then send EnrichmentCheckedMsg{ResourceType: "dbi"}
// with a wave2 finding. applyEnrichment must mutate ProbeResources["dbi"][0] to
// hold 2 findings (wave1 + wave2).
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

	res := resource.Resource{
		ID:     rid,
		Name:   "test-dbi-alias",
		Status: "impaired",
		Issues: []string{"impaired"},
	}

	// Step 1: send AvailabilityCheckedMsg with alias "rds" → ProbeResources["dbi"]
	// is populated with wave1 findings (deriveFindingsForType called by handler).
	m = shimApplyMsg(m, messages.AvailabilityChecked{
		ResourceType: "rds", // alias — handler normalizes to canonical "dbi"
		HasResources: true,
		Count:        1,
		Truncated:    false,
		// session.New seeds AvailabilityGen=1 (AS-659) — stamp the live
		// value so the AvailabilityChecked stale guard accepts it.
		Gen:       m.Core().Session().AvailabilityGen,
		Issues:    1,
		Resources: []resource.Resource{res},
	})

	// Step 2: send EnrichmentCheckedMsg with canonical "dbi" carrying wave2 finding.
	// handleEnrichmentChecked stores findings then calls applyEnrichment which
	// calls DeriveFindings (wave1+wave2) on ProbeResources["dbi"] rows in-place.
	//
	// Set EnrichTotal > 1 so the "all enrichment done" branch (EnrichChecked >= EnrichTotal)
	// does not fire after processing this single message, which would nil out ProbeResources
	// before we can inspect it.
	m.Core().Session().EnrichTotal = 2
	m = shimApplyMsg(m, messages.EnrichmentChecked{
		ResourceType: "dbi",
		Findings: map[string]resource.EnrichmentFinding{
			rid: wave2Finding,
		},
		Gen:     0,
		TypeGen: 0,
	})

	probeSlice, ok := m.Core().Session().ProbeResources["dbi"]
	if !ok || len(probeSlice) == 0 {
		t.Fatal("ProbeResources[\"dbi\"] is empty after AvailabilityCheckedMsg with alias \"rds\"")
	}

	got := probeSlice[0]
	if len(got.Findings) < 2 {
		t.Errorf(
			"ProbeResources[\"dbi\"][0].Findings: got %d finding(s), want 2 (wave1 + wave2) — "+
				"applyEnrichment must have merged wave2 findings from EnrichmentCheckedMsg into ProbeResources rows",
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

// TestShim_DeriveHelpersResolveAlias_SingleResource verifies that after
// applyEnrichment is called via EnrichmentCheckedMsg (using the alias "rds"),
// ProbeResources["dbi"] rows carry both wave1 and wave2 findings.
//
// After PR-03a-fold: deriveFindingsForType no longer reads m.EnrichmentFindings.
// Wave2 data flows through EnrichmentCheckedMsg → handleEnrichmentChecked which
// normalizes the alias ("rds" → "dbi") then calls applyEnrichment, which calls
// DeriveFindings with the full findings map on all cached rows.
//
// Setup: send AvailabilityCheckedMsg{ResourceType: "rds"} to seed ProbeResources
// with wave1. Then send EnrichmentCheckedMsg{ResourceType: "rds"} (alias) with
// a wave2 finding. handleEnrichmentChecked normalizes "rds" → "dbi" and
// applyEnrichment merges wave2 into ProbeResources["dbi"][0].
//
// Expected: 2 findings (wave1 + wave2).
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

	res := resource.Resource{
		ID:     rid,
		Name:   "test-dbi-single-alias",
		Status: "impaired",
		Issues: []string{"impaired"},
	}

	// Step 1: AvailabilityCheckedMsg with alias "rds" → ProbeResources["dbi"] with wave1.
	m = shimApplyMsg(m, messages.AvailabilityChecked{
		ResourceType: "rds", // alias — handler normalizes to canonical "dbi"
		HasResources: true,
		Count:        1,
		Truncated:    false,
		// session.New seeds AvailabilityGen=1 (AS-659) — stamp the live
		// value so the AvailabilityChecked stale guard accepts it.
		Gen:       m.Core().Session().AvailabilityGen,
		Issues:    1,
		Resources: []resource.Resource{res},
	})

	// Step 2: EnrichmentCheckedMsg with alias "rds" (mirrors real production path
	// where the enricher might be dispatched with the alias). handleEnrichmentChecked
	// normalizes "rds" → "dbi", then applyEnrichment merges wave2 into
	// ProbeResources["dbi"] rows.
	//
	// Set EnrichTotal > 1 so the "all enrichment done" branch (EnrichChecked >= EnrichTotal)
	// does not fire after processing this single message, which would nil out ProbeResources
	// before we can inspect it.
	m.Core().Session().EnrichTotal = 2
	m = shimApplyMsg(m, messages.EnrichmentChecked{
		ResourceType: "rds", // alias — exercises alias normalization in handleEnrichmentChecked
		Findings: map[string]resource.EnrichmentFinding{
			rid: wave2Finding,
		},
		Gen:     0,
		TypeGen: 0,
	})

	probeSlice, ok := m.Core().Session().ProbeResources["dbi"]
	if !ok || len(probeSlice) == 0 {
		t.Fatal("ProbeResources[\"dbi\"] is empty after AvailabilityCheckedMsg with alias \"rds\"")
	}

	got := probeSlice[0]
	if len(got.Findings) < 2 {
		t.Errorf(
			"ProbeResources[\"dbi\"][0].Findings: got %d finding(s), want 2 (wave1 + wave2) — "+
				"applyEnrichment via EnrichmentCheckedMsg alias \"rds\" must merge wave2 into ProbeResources[\"dbi\"] rows",
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

// ── Wire-up sites added in this append block ────────────────────────────────
//
// Site 1  (app.go:342)  — ResourcesLoadedMsg → non-list cache write-through.
// Site 6  — child-view navigate cache-miss: already covered by the combination
//            of TestShim_NavigateAliasHitsCanonicalCache (cache-hit path at
//            app_handlers_navigate.go:56) and TestShim_ResourcesLoadedPopulatesFindings
//            (cache-miss path returns a fetchResources command whose eventual
//            ResourcesLoadedMsg result is handled by the Site 1 handler). No
//            redundant test written.
// Site 7  (app.go:446)  — EnrichDetailResultMsg → deriveFindingsForResource.
//            Requires tui.Model.ActiveDetailResource() accessor (see note below).
//
// ── Note on Site 7 accessor requirement ────────────────────────────────────
//
//   views.DetailModel already has a public SourceResource() accessor. However,
//   tui.Model.stack is unexported and there is no public ActiveView() method,
//   so the test cannot retrieve the active DetailModel from a tui.Model after
//   Update(). Two approaches are available:
//     (a) Add a public accessor to tui.Model — e.g.,
//            func (m Model) ActiveDetailResource() (resource.Resource, bool)
//         This is the approach chosen here. The test calls m.ActiveDetailResource()
//         so it fails to COMPILE until the coder adds the accessor. This
//         satisfies the TDD red-light requirement without modifying production
//         logic.
//     (b) Inspect via View() string rendering (brittle, not used).
//   Choice: option (a). The accessor is a read-only helper with zero risk of
//   changing production behavior; detail-model accessors are already present
//   in the views layer.
//
//   Coder action required: add to internal/tui/app.go (or a new accessor file):
//
//       // ActiveDetailResource returns the resource held by the top-of-stack
//       // DetailModel, if any. Used by tests to inspect shim wire-ups.
//       func (m Model) ActiveDetailResource() (resource.Resource, bool) {
//           if d, ok := m.activeView().(*views.DetailModel); ok {
//               return d.SourceResource(), true
//           }
//           return resource.Resource{}, false
//       }
//
// ──────────────────────────────────────────────────────────────────────────

// TestShim_NavigateAliasHitsCanonicalCache verifies that handleNavigate resolves
// an alias to the canonical ShortName before looking up m.Core().Session().ResourceCache.
//
// Bug: app_handlers_navigate.go:49 does m.Core().Session().ResourceCache[msg.ResourceType] —
// when msg.ResourceType is "rds" (alias) but the cache is keyed under "dbi"
// (canonical), the lookup misses. deriveFindingsForType is never called on the
// cached resources, so Findings remain empty.
//
// Setup: seed m.Core().Session().ResourceCache["dbi"] with one resource (Status="impaired").
// Drive NavigateMsg{Target: TargetResourceList, ResourceType: "rds"}.
// Post-fix: m.Core().Session().ResourceCache["dbi"].Resources[0].Findings is populated.
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
	m.Core().Session().ResourceCache["dbi"] = &session.ResourceCacheEntry{
		Resources: []resource.Resource{res},
	}

	// Navigate using the alias "rds". The handler must resolve to "dbi" and
	// find the cache entry, then call deriveFindingsForType on it.
	m = shimApplyMsg(m, messages.Navigate{
		Target:       messages.TargetResourceList,
		ResourceType: "rds", // alias for "dbi"
	})

	// The cache under "dbi" must now have Findings populated.
	entry, ok := m.Core().Session().ResourceCache["dbi"]
	if !ok || len(entry.Resources) == 0 {
		t.Fatal("ResourceCache[\"dbi\"] is empty after NavigateMsg — unexpected state")
	}

	got := entry.Resources[0]
	if len(got.Findings) == 0 {
		t.Errorf(
			"ResourceCache[\"dbi\"].Resources[0].Findings: got empty — " +
				"handleNavigate not resolving alias \"rds\" to canonical \"dbi\" for cache hit path; " +
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

// ── Site 1: ResourcesLoadedMsg ───────────────────────────────────────────────

// TestShim_ResourcesLoadedPopulatesFindings verifies that when
// ResourcesLoadedMsg is handled and the active view is NOT a ResourceListModel
// (initial model state: main menu), the resources cached via the non-list
// write-through path (app.go:383-391) have their Findings field populated by
// the DeriveFindings shim at app.go:342.
//
// Wire-up site: app.go:342 — (&m).deriveFindingsForType(msg.ResourceType, msg.Resources)
// called before updateActiveView and the write-through cache block.
//
// Red-light expectation: if the shim at line 342 is removed, Findings will be
// nil on the cached resources because the write-through stores msg.Resources
// verbatim (no separate derive call).
func TestShim_ResourcesLoadedPopulatesFindings(t *testing.T) {
	cases := []struct {
		name, canonShort, alias, status, expectedSlug string
	}{
		{"ec2-canonical", "ec2", "", "impaired", "impaired"},
		{"s3-canonical", "s3", "", "bucket public", "bucket.public"},
		{"rds-aliased", "dbi", "rds", "maintenance pending", "maintenance.pending"},
		{"redis-aliased", "redis", "elasticache", "failover pending", "failover.pending"},
		{"sg-canonical", "sg", "", "overly permissive", "overly.permissive"},
		{"kms-canonical", "kms", "", "pending deletion", "pending.deletion"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			m := newShimModel()
			// Active view is the main menu — no ResourceListModel on stack.
			// This drives the non-list write-through path at app.go:383-391.

			effective := tc.canonShort
			if tc.alias != "" {
				effective = tc.alias
			}

			res := resource.Resource{
				ID:     "r-loaded-" + tc.canonShort,
				Name:   "loaded-" + tc.canonShort,
				Status: tc.status,
			}

			m = shimApplyMsg(m, messages.ResourcesLoaded{
				ResourceType: effective,
				Resources:    []resource.Resource{res},
			})

			// The non-list write-through path caches under msg.ResourceType (may be
			// an alias). The canonical test: check both canonical and alias keys to
			// find where the entry landed, then assert Findings.
			var entry *session.ResourceCacheEntry
			if e, ok := m.Core().Session().ResourceCache[tc.canonShort]; ok {
				entry = e
			} else if tc.alias != "" {
				if e, ok := m.Core().Session().ResourceCache[effective]; ok {
					entry = e
				}
			}

			if entry == nil || len(entry.Resources) == 0 {
				t.Fatalf("ResourceCache has no entry for type %q (canonShort=%q, alias=%q) after ResourcesLoadedMsg — write-through path not triggered",
					effective, tc.canonShort, tc.alias)
			}

			got := entry.Resources[0]
			if len(got.Findings) == 0 {
				t.Errorf(
					"ResourceCache entry for %q: Resources[0].Findings is empty — "+
						"shim at app.go:342 not called or findings not propagated to cached slice; "+
						"want wave1 Finding with Code %q",
					effective, tc.canonShort+"."+tc.expectedSlug,
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

// ── Site 7: EnrichDetailResultMsg ───────────────────────────────────────────

// TestShim_EnrichDetailResultPopulatesFindings verifies that when
// EnrichDetailResultMsg is processed, the shim at app.go:446 calls
// deriveFindingsForResource on msg.EnrichedRes before handing it to the
// detail view. After handling, the detail view's resource must have its
// Findings field populated.
//
// Wire-up site: app.go:446 — (&m).deriveFindingsForResource(msg.ResourceType, &msg.EnrichedRes)
//
// This test requires tui.Model.ActiveDetailResource() — a public read-only
// accessor that returns the resource held by the top-of-stack DetailModel.
// See the coder action note at the top of this append block. The test will
// FAIL TO COMPILE until the accessor is added, satisfying the TDD red-light
// requirement.
//
// Red-light expectation: if the shim at line 446 is removed, msg.EnrichedRes
// reaches the detail view without Findings, so ActiveDetailResource().Findings
// will be nil.
func TestShim_EnrichDetailResultPopulatesFindings(t *testing.T) {
	const (
		resID      = "i-enrich-001"
		resType    = "ec2"
		resStatus  = "impaired"
		wantPhrase = "impaired"
		wantCode   = domain.FindingCode("ec2.impaired")
		wantSource = "wave1"
	)

	m := newShimModel()

	// Navigate to detail view so the stack has a DetailModel on top.
	res := resource.Resource{
		ID:     resID,
		Name:   "enrich-test-instance",
		Status: resStatus,
	}
	m = shimApplyMsg(m, messages.Navigate{
		Target:       messages.TargetDetail,
		Resource:     &res,
		ResourceType: resType,
	})

	// Build the enriched resource carrying the same ID so the guard in
	// DetailModel.Update() (msg.ResourceID == m.res.ID) passes.
	enriched := resource.Resource{
		ID:     resID,
		Name:   "enrich-test-instance-enriched",
		Status: resStatus,
	}

	// Send EnrichDetailResultMsg with Generation=0 (always accepted by the
	// stale-check guard at app.go:432-434).
	m = shimApplyMsg(m, messages.EnrichDetailResult{
		ResourceType: resType,
		ResourceID:   resID,
		EnrichedRes:  enriched,
		Err:          nil,
		Generation:   0,
	})

	// Retrieve the resource from the active DetailModel via the public accessor.
	// This line intentionally fails to compile until the coder adds
	//   func (m Model) ActiveDetailResource() (resource.Resource, bool)
	// to internal/tui/app.go (see coder action note above).
	got, ok := m.ActiveDetailResource()
	if !ok {
		t.Fatal("ActiveDetailResource: no DetailModel on view stack — NavigateMsg to TargetDetail did not push a detail view")
	}

	if len(got.Findings) == 0 {
		t.Errorf(
			"ActiveDetailResource().Findings: got empty — "+
				"shim at app.go:446 not called or findings not reaching detail view; "+
				"want wave1 Finding with Phrase=%q Code=%q Source=%q",
			wantPhrase, wantCode, wantSource,
		)
		return
	}

	f := got.Findings[0]
	if f.Code != wantCode {
		t.Errorf("Findings[0].Code: got %q, want %q", f.Code, wantCode)
	}
	if f.Phrase != wantPhrase {
		t.Errorf("Findings[0].Phrase: got %q, want %q", f.Phrase, wantPhrase)
	}
	if f.Source != wantSource {
		t.Errorf("Findings[0].Source: got %q, want %q", f.Source, wantSource)
	}
}
