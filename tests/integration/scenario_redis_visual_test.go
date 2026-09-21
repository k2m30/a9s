//go:build integration

package integration

// scenario_redis_visual_test.go checks the rendered TUI output (not fetcher
// return values) for redis against the universal UI rules and
// docs/resources/redis.md.

import (
	"testing"

	demofixtures "github.com/k2m30/a9s/v3/core/demo/fixtures"
	"github.com/k2m30/a9s/v3/core/resource"
)

func TestScenario_RedisVisual(t *testing.T) {
	scenario := fullIntegrationNewDemoScenario(t)
	runDemoStartup(t, scenario)

	// Menu badge — rows whose Wave-1-only colour IsIssue (redis has no
	// Wave-2 enricher). Over the 16 redis fixtures:
	//   Broken (2):   bad-config-redis (create-failed) and
	//                 broken-redis-no-auth (redis.no-auth)
	//   Warning (11): dev-feature-redis, prod-redis-cache, prod-redis-analytics,
	//                 old-redis-unused, legacy-redis-analytics,
	//                 legacy-redis-billing, multi-shard-modifying-0001,
	//                 multi-shard-2-transitioning, warn-redis-{at-rest-off,
	//                 transit-off,no-backup}. A Wave-1 `~` colours the row
	//                 Warning, which IsIssue.
	// 2 + 11 = 13. The Healthy fixtures and the Valkey fixture
	// (engine-filtered, absent from the redis list) do not bump.
	scenario.ExpectMenuIssueCount("redis", 13)

	scenario.OpenList("redis")

	// DescribeReplicationGroups returns the Valkey fixture too; the engine
	// filter keeps it out of the redis list.
	scenario.ExpectViewNotContains(demofixtures.ValkeyEngineID)
	scenario.ExpectViewNotContains("prod-valkey")

	for _, jargon := range []string{"Failover", "CIS", "NOBKP", "UNENC", "NOPROT", "Flags", "Policy"} {
		scenario.ExpectViewNotContains(jargon)
	}

	// Healthy rows render a blank Status, never "OK" / "available".
	scenario.ExpectRowStatusBlank(demofixtures.ProdRedisID)
	scenario.ExpectRowStatusBlank("staging-redis")                  // single-AZ carries no finding
	scenario.ExpectRowStatusBlank(demofixtures.MultiShardHealthyID) // cluster-mode-enabled, all shards available

	// `—` is a literal em-dash.
	scenario.ExpectRowStatusEquals("dev-feature-redis", "creating — new group")
	scenario.ExpectRowStatusEquals("prod-redis-cache", "modifying — config change")
	scenario.ExpectRowStatusEquals("prod-redis-analytics", "snapshotting — backup running")
	scenario.ExpectRowStatusEquals("old-redis-unused", "deleting — teardown")
	scenario.ExpectRowStatusEquals("bad-config-redis", "create failed — see events")
	scenario.ExpectRowStatusEquals("legacy-redis-analytics", "multi-AZ without auto-failover")

	// Alphabetical order within the Warning bucket places
	// `modifying — config change` before `multi-AZ without auto-failover`.
	scenario.ExpectRowStatusEquals(demofixtures.WarnRedisMultiID, "modifying — config change (+1)")

	// Multi-shard RGs phrase each non-available shard as `shard <ng-id>: <state>`.
	scenario.ExpectRowStatusEquals(demofixtures.MultiShardOneModifyingID, "shard 0001: modifying")
	scenario.ExpectRowStatusEquals(demofixtures.MultiShardTwoTransitioningID, "shard 0001: modifying (+1)")

	// Glyphs appear only on Healthy + Wave-2, and redis has no Wave-2 signals.
	for _, id := range []string{
		demofixtures.ProdRedisID,
		"staging-redis",
		demofixtures.MultiShardHealthyID,
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

	prod := selectRedisByID(t, scenario, demofixtures.ProdRedisID)
	scenario.OpenDetailResource("redis", prod)
	scenario.ExpectNoAPIError()
	// The CloudTrail row carries no count: a row's events are read from
	// CloudTrail with the row's own lookup when the operator opens the row.
	scenario.ExpectRelatedRow("CloudTrail Events")
	for _, displayName := range []string{
		"CW Alarms",
		"CloudFormation",
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

	scenario.Back()
	multi := selectRedisByID(t, scenario, demofixtures.WarnRedisMultiID)
	scenario.OpenDetailResource("redis", multi)
	scenario.ExpectNoAPIError()
	t.Log("\n" + scenario.currentView())
}

// TestScenario_RedisVisual_DetailSurfacesAllIssues asserts that multi-warning
// fixtures enumerate every Resource.Issues
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
		{demofixtures.ProdRedisID, nil},
		{"staging-redis", nil},
		{"dev-feature-redis", []string{"Creating — new group"}},
		{"prod-redis-cache", []string{"Modifying — config change"}},
		{"prod-redis-analytics", []string{"Snapshotting — backup running"}},
		{"old-redis-unused", []string{"Deleting — teardown"}},
		{"bad-config-redis", []string{"Create failed — see events"}},
		{"legacy-redis-analytics", []string{"Multi-AZ without auto-failover"}},
		{demofixtures.WarnRedisMultiID, []string{"Modifying — config change", "Multi-AZ without auto-failover"}},
		// Multi-shard RGs list each transitioning shard as its own Attention entry.
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
