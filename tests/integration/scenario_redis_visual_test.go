//go:build integration

package integration

// scenario_redis_visual_test.go — Phase 8 render-gate for the redis resource.
//
// Verifies the rendered TUI output (not fetcher return values) matches the
// universal UI rules and the per-resource §4 contract in docs/resources/redis.md.
// Authored by the a9s-implement-resource skill runner.

import (
	"testing"

	demofixtures "github.com/k2m30/a9s/v3/internal/demo/fixtures"
	"github.com/k2m30/a9s/v3/internal/resource"
)

func TestScenario_RedisVisual(t *testing.T) {
	scenario := fullIntegrationNewDemoScenario(t)
	runDemoStartup(t, scenario)

	// -----------------------------------------------------------------
	// S1 menu badge — count of redis fixtures whose row color is Warning
	// or Broken (app counts ResolveColor(r).IsIssue() rows). Per §3.1:
	//   Warning: creating, modifying, snapshotting, deleting,
	//            multi-AZ-no-failover, multi-W1, multi-shard-modifying-0001,
	//            multi-shard-two-transitioning = 8
	//   Broken:  create-failed = 1
	// → total 9. Healthy fixtures (prod + staging + multi-shard-healthy) and
	// the Valkey fixture (engine-filtered, not in the redis list) do not bump.
	// -----------------------------------------------------------------
	scenario.ExpectMenuIssueCount("redis", 9)

	scenario.OpenList("redis")

	// -----------------------------------------------------------------
	// Engine filter (P2-1 regression pin) — Valkey fixture must NOT appear
	// in the redis list, even though DescribeReplicationGroups returned it.
	// -----------------------------------------------------------------
	scenario.ExpectViewNotContains(demofixtures.ValkeyEngineID)
	scenario.ExpectViewNotContains("prod-valkey")

	// -----------------------------------------------------------------
	// Universal column rules — no jargon columns anywhere in the frame.
	// Includes "Failover" (the stale pre-migration column) and "Version"
	// (removed because ReplicationGroup has no EngineVersion field).
	// -----------------------------------------------------------------
	for _, jargon := range []string{"Failover", "CIS", "NOBKP", "UNENC", "NOPROT", "Flags", "Policy"} {
		scenario.ExpectViewNotContains(jargon)
	}

	// -----------------------------------------------------------------
	// Healthy rows — blank Status (§4 rule: no "OK" / "available").
	// -----------------------------------------------------------------
	scenario.ExpectRowStatusBlank(demofixtures.ProdRedisID)      // graph root
	scenario.ExpectRowStatusBlank("staging-redis")               // single-AZ, no finding per §4 note
	scenario.ExpectRowStatusBlank(demofixtures.MultiShardHealthyID) // cluster-mode-enabled, all shards available

	// -----------------------------------------------------------------
	// Wave 1 §4 phrases per fixture. Exact match — `—` is literal em-dash.
	// -----------------------------------------------------------------
	scenario.ExpectRowStatusEquals("dev-feature-redis", "creating — new group")
	scenario.ExpectRowStatusEquals("prod-redis-cache", "modifying — config change")
	scenario.ExpectRowStatusEquals("prod-redis-analytics", "snapshotting — backup running")
	scenario.ExpectRowStatusEquals("old-redis-unused", "deleting — teardown")
	scenario.ExpectRowStatusEquals("bad-config-redis", "create failed — see events")
	scenario.ExpectRowStatusEquals("legacy-redis-analytics", "multi-AZ without auto-failover")

	// Rule 7 U7a — multi-W1: modifying + multi-AZ-no-failover → top + (+1).
	// §4 precedence: alphabetical within the Warning bucket places
	// `modifying — config change` before `multi-AZ without auto-failover`.
	scenario.ExpectRowStatusEquals(demofixtures.WarnRedisMultiID, "modifying — config change (+1)")

	// -----------------------------------------------------------------
	// Shard-level Wave 1 (spec §3.1 expansion, 2026-04-23).
	// Multi-shard RGs: §4 phrase is `shard <ng-id>: <state>`. Single non-available
	// shard → single phrase. Two non-available shards → top phrase + (+1).
	// -----------------------------------------------------------------
	scenario.ExpectRowStatusEquals(demofixtures.MultiShardOneModifyingID, "shard 0001: modifying")
	scenario.ExpectRowStatusEquals(demofixtures.MultiShardTwoTransitioningID, "shard 0001: modifying (+1)")

	// -----------------------------------------------------------------
	// Glyph rules. Redis has no Wave-2 signals, so no Healthy row ever
	// carries a glyph (§3 rule: glyphs appear only on Healthy + Wave-2).
	// All non-green rows also must NOT carry a glyph.
	// -----------------------------------------------------------------
	for _, id := range []string{
		demofixtures.ProdRedisID,          // Healthy, no finding — no glyph
		"staging-redis",                   // Healthy single-AZ, no finding — no glyph
		demofixtures.MultiShardHealthyID,  // Healthy multi-shard, no finding — no glyph
		"dev-feature-redis",
		"prod-redis-cache",
		"prod-redis-analytics",
		"old-redis-unused",
		"bad-config-redis",
		"legacy-redis-analytics",
		demofixtures.WarnRedisMultiID,
		demofixtures.MultiShardOneModifyingID,
		demofixtures.MultiShardTwoTransitioningID,
	} {
		scenario.ExpectRowNoGlyphPrefix(id)
	}

	// -----------------------------------------------------------------
	// Related panel — every §2 pivot (`count shown: yes`) returns ≥ 1 on
	// the graph-root fixture (prod-redis-sessions). All 10 pivots are
	// required to resolve here per the spec's "count shown: yes" contract.
	// -----------------------------------------------------------------
	prod := selectRedisByID(t, scenario, demofixtures.ProdRedisID)
	scenario.OpenDetailResource("redis", prod)
	scenario.ExpectNoAPIError()
	for _, displayName := range []string{
		"CW Alarms",
		"CloudFormation",
		"CloudTrail Events",
		"KMS Key",
		"Log Groups",
		"Secrets Manager",
		"Security Groups",
		"SNS Topics",
		"Subnets",
		"VPC",
	} {
		scenario.ExpectRelatedRowCountAtLeast(displayName, 1)
	}

	// -----------------------------------------------------------------
	// Phase 8.4 visual-sanity dump — one multi-issue detail view to stderr
	// so a reviewer sees what the ./a9s --demo user actually sees.
	// -----------------------------------------------------------------
	scenario.Back()
	multi := selectRedisByID(t, scenario, demofixtures.WarnRedisMultiID)
	scenario.OpenDetailResource("redis", multi)
	scenario.ExpectNoAPIError()
	t.Log("\n" + scenario.currentView())
}

// TestScenario_RedisVisual_DetailSurfacesAllIssues asserts spec rule 7 for the
// detail view. Multi-warning fixtures must enumerate every Resource.Issues
// entry, not just the top phrase shown in the Status column.
func TestScenario_RedisVisual_DetailSurfacesAllIssues(t *testing.T) {
	scenario := fullIntegrationNewDemoScenario(t)
	runDemoStartup(t, scenario)
	scenario.OpenList("redis")

	type issueCase struct {
		id     string
		issues []string // nil = silence; Attention header must be absent
	}
	// Attention section capitalizes the first letter of each entry.
	cases := []issueCase{
		// Healthy baselines — Attention section must be absent.
		{demofixtures.ProdRedisID, nil},
		{"staging-redis", nil},
		// Single Wave-1 signals (one entry each, capitalized).
		{"dev-feature-redis", []string{"Creating — new group"}},
		{"prod-redis-cache", []string{"Modifying — config change"}},
		{"prod-redis-analytics", []string{"Snapshotting — backup running"}},
		{"old-redis-unused", []string{"Deleting — teardown"}},
		{"bad-config-redis", []string{"Create failed — see events"}},
		{"legacy-redis-analytics", []string{"Multi-AZ without auto-failover"}},
		// U7e — multi Wave-1: every entry of Resource.Issues must appear in
		// detail (capitalized), in §4 precedence order.
		{demofixtures.WarnRedisMultiID, []string{"Modifying — config change", "Multi-AZ without auto-failover"}},
		// Shard-level Wave 1 (2026-04-23): multi-shard RGs enumerate each
		// transitioning shard as a distinct entry in the Attention section.
		{demofixtures.MultiShardHealthyID, nil},
		{demofixtures.MultiShardOneModifyingID, []string{"Shard 0001: modifying"}},
		{demofixtures.MultiShardTwoTransitioningID, []string{"Shard 0001: modifying", "Shard 0002: snapshotting"}},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.id, func(t *testing.T) {
			res := selectRedisByID(t, scenario, tc.id)
			scenario.OpenDetailResource("redis", res)
			scenario.ExpectNoAPIError()
			view := scenario.currentView()
			t.Log("\n" + view)

			if len(tc.issues) == 0 {
				expectNoAttentionSection(t, view)
			} else {
				expectAttentionSection(t, view, tc.issues)
			}
			scenario.Back()
		})
	}
}

// selectRedisByID looks up a concrete redis resource from the demo clients so
// the scenario can call OpenDetailResource with a real resource value.
func selectRedisByID(t *testing.T, s *fullIntegrationScenario, id string) resource.Resource {
	t.Helper()
	return fullIntegrationMustFindResourceByID(t, s.clients, "redis", id)
}
