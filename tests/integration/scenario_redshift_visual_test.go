//go:build integration

package integration

// scenario_redshift_visual_test.go — Phase 8 render-gate for the redshift resource.
//
// Verifies the rendered TUI output (not fetcher return values) matches the
// universal UI rules and the per-resource §4 contract in docs/resources/redshift.md.
// Authored by the a9s-implement-resource skill runner (not QA), because these
// assertions guard rendering pipeline drift independent of unit-test coverage.
//
// Wave 2 = None for redshift. Universal rules U3 / U4 / U7b / U7c / U7d are
// structurally unreachable (no `~` / `!` findings). This test asserts the
// reachable rules only: U1 (Healthy blank S4), U2 (§4 phrases), U5 (no glyph
// on non-green rows), U6 (menu badge = 0), U7a (multi-W1 suffix), U7e (detail
// enumerates every Wave-1 phrase), U7f (Resource.Issues populated), U8
// (Broken > Warning severity precedence), U9 (related pivots), U10 (no jargon
// columns).

import (
	"strings"
	"testing"

	demofixtures "github.com/k2m30/a9s/v3/internal/demo/fixtures"
	"github.com/k2m30/a9s/v3/internal/resource"
)

func TestScenario_RedshiftVisual(t *testing.T) {
	// Isolate from developer's ~/.a9s/views/redshift.yaml which may be stale.
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
	scenario := fullIntegrationNewDemoScenario(t)

	// Drive the real demo startup: Init → ClientsReadyMsg → demoPrefetchCounts
	// → AvailabilityPrefetchedMsg. Wave 2 = None means the enrichment chain is
	// a NoOp for redshift, but the prefetch still seeds resourceCache.
	runDemoStartup(t, scenario)

	scenario.OpenList("redshift")

	// -----------------------------------------------------------------
	// U10 — no jargon columns anywhere in the frame.
	// -----------------------------------------------------------------
	for _, jargon := range []string{"CIS", " Flags", "NOBKP", "UNENC", "NOPROT", "cis_flags"} {
		scenario.ExpectViewNotContains(jargon)
	}

	// -----------------------------------------------------------------
	// U1 — Healthy rows render blank Status.
	// -----------------------------------------------------------------
	scenario.ExpectRowStatusBlank(demofixtures.AcmeWarehouseID)
	scenario.ExpectRowStatusBlank(demofixtures.AcmeReportingID)
	scenario.ExpectRowStatusBlank(demofixtures.StagingDwhID) // paused is Out of Scope → Healthy silence.

	// -----------------------------------------------------------------
	// U2 — Warning / Broken rows show the exact §4 phrase.
	// -----------------------------------------------------------------
	// ClusterStatus transitional (Warning)
	scenario.ExpectRowStatusEquals(demofixtures.RedshiftResizingID, "resizing")
	scenario.ExpectRowStatusEquals(demofixtures.RedshiftRebootingID, "rebooting")

	// ClusterStatus broken
	scenario.ExpectRowStatusEquals(demofixtures.RedshiftIncompatibleNetworkID, "broken: incompatible-network")
	scenario.ExpectRowStatusEquals(demofixtures.RedshiftHardwareFailureID, "broken: hardware-failure")
	scenario.ExpectRowStatusEquals(demofixtures.RedshiftStorageFullID, "broken: storage-full")

	// ClusterAvailabilityStatus broken
	scenario.ExpectRowStatusEquals(demofixtures.RedshiftAvailUnavailableID, "unavailable")
	scenario.ExpectRowStatusEquals(demofixtures.RedshiftAvailFailedID, "failed")

	// ClusterAvailabilityStatus warning
	scenario.ExpectRowStatusEquals(demofixtures.RedshiftAvailMaintenanceID, "maintenance")
	scenario.ExpectRowStatusEquals(demofixtures.RedshiftAvailModifyingID, "modifying")

	// Config / maintenance warnings
	scenario.ExpectRowStatusEquals(demofixtures.RedshiftPendingChangeID, "pending change queued")
	scenario.ExpectRowStatusEquals(demofixtures.RedshiftMaintenanceDeferredID, "maintenance deferred")
	scenario.ExpectRowStatusEquals(demofixtures.RedshiftPubliclyAccessibleID, "publicly accessible")
	scenario.ExpectRowStatusEquals(demofixtures.RedshiftUnencryptedID, "unencrypted at rest")

	// Expired deferred-maintenance window — must NOT trigger (U2 negative).
	scenario.ExpectRowStatusBlank(demofixtures.RedshiftMaintenanceDeferredExpiredID)

	// U7a — multi-W1: 3 warnings → top + (+2).
	scenario.ExpectRowStatusEquals(demofixtures.WarnRedshiftMultiID, "pending change queued (+2)")
	// Intermediate case: 2 warnings → top + (+1).
	scenario.ExpectRowStatusEquals(demofixtures.WarnRedshiftTwoID, "publicly accessible (+1)")

	// U8 — Broken severity beats Warning. Even when public/unencrypted warnings
	// coexist with a Broken ClusterStatus / ClusterAvailabilityStatus, only the
	// Broken phrase surfaces; no `(+N)` suffix.
	scenario.ExpectRowStatusEquals(demofixtures.RedshiftBrokenWithWarningHiddenID, "broken: storage-full")
	scenario.ExpectRowStatusEquals(demofixtures.RedshiftAvailUnavailableWithWarningHiddenID, "unavailable")

	// -----------------------------------------------------------------
	// U5 — non-green rows never carry a `!` / `~` glyph regardless of finding.
	// (Redundant for redshift since Wave 2 = None, but the assertion locks the
	// invariant so future Wave-2 additions can't regress row-color/glyph.)
	// -----------------------------------------------------------------
	for _, id := range []string{
		demofixtures.RedshiftResizingID,
		demofixtures.RedshiftRebootingID,
		demofixtures.RedshiftIncompatibleNetworkID,
		demofixtures.RedshiftHardwareFailureID,
		demofixtures.RedshiftStorageFullID,
		demofixtures.RedshiftAvailUnavailableID,
		demofixtures.RedshiftAvailFailedID,
		demofixtures.RedshiftAvailMaintenanceID,
		demofixtures.RedshiftAvailModifyingID,
		demofixtures.RedshiftPendingChangeID,
		demofixtures.RedshiftMaintenanceDeferredID,
		demofixtures.RedshiftPubliclyAccessibleID,
		demofixtures.RedshiftUnencryptedID,
		demofixtures.WarnRedshiftMultiID,
		demofixtures.WarnRedshiftTwoID,
		demofixtures.RedshiftBrokenWithWarningHiddenID,
		demofixtures.RedshiftAvailUnavailableWithWarningHiddenID,
		// Plain Healthy rows with no finding also have no glyph.
		demofixtures.AcmeWarehouseID,
		demofixtures.AcmeReportingID,
		demofixtures.StagingDwhID,
		demofixtures.RedshiftMaintenanceDeferredExpiredID,
	} {
		scenario.ExpectRowNoGlyphPrefix(id)
	}

	// -----------------------------------------------------------------
	// U6 — menu badge. Wave 2 = None → zero `!` findings → badge absent / 0.
	// -----------------------------------------------------------------
	scenario.ExpectMenuIssueCount("redshift", 0)

	// -----------------------------------------------------------------
	// U9 — related pivots on graph-root #1 (CloudWatch-logging variant).
	//
	// acme-warehouse covers every `count shown: yes` pivot EXCEPT s3 (AWS logging
	// destinations are mutually exclusive — see docs/resources/redshift-impl-plan.md §5.1).
	// Graph-root #2 (acme-reporting, S3-logging) covers s3. Together they cover
	// 11/11 `count shown: yes` pivots.
	// -----------------------------------------------------------------
	warehouse := selectRedshiftByID(t, scenario, demofixtures.AcmeWarehouseID)
	scenario.OpenDetailResource("redshift", warehouse)
	scenario.ExpectNoAPIError()
	for _, displayName := range []string{
		"CW Alarms", "Security Groups", "VPC", "IAM Role", "KMS Key",
		"CloudFormation", "Secrets Manager", "Log Groups", "Subnets",
	} {
		scenario.ExpectRelatedRowCountAtLeast(displayName, 1)
	}
	// S3 Buckets is 0 on the CloudWatch-logging graph-root (mutual exclusion).

	// -----------------------------------------------------------------
	// U9 — related pivots on graph-root #2 (S3-logging variant).
	// Covers s3 pivot; logs is 0 (mutual exclusion).
	// -----------------------------------------------------------------
	scenario.Back()
	reporting := selectRedshiftByID(t, scenario, demofixtures.AcmeReportingID)
	scenario.OpenDetailResource("redshift", reporting)
	scenario.ExpectNoAPIError()
	for _, displayName := range []string{
		"CW Alarms", "Security Groups", "VPC", "IAM Role", "KMS Key",
		"CloudFormation", "Secrets Manager", "S3 Buckets", "Subnets",
	} {
		scenario.ExpectRelatedRowCountAtLeast(displayName, 1)
	}

	// -----------------------------------------------------------------
	// U7e — detail view enumerates every Wave-1 phrase on the multi-W1 row.
	// -----------------------------------------------------------------
	scenario.Back()
	multi := selectRedshiftByID(t, scenario, demofixtures.WarnRedshiftMultiID)
	scenario.OpenDetailResource("redshift", multi)
	scenario.ExpectNoAPIError()
	// Attention section capitalizes the first rune at render time; data stays lowercase.
	expectAttentionSection(t, scenario.currentView(), []string{
		"Pending change queued",
		"Publicly accessible",
		"Unencrypted at rest",
	})
}

// TestScenario_RedshiftVisual_DetailSurfacesAllIssues asserts spec rule 7
// ("every finding individually visible across S2–S5") for the detail view (S5).
// Paste the rendered detail frame into the test log so the reader can see
// exactly what an `./a9s --demo` user would see on each row.
func TestScenario_RedshiftVisual_DetailSurfacesAllIssues(t *testing.T) {
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
	scenario := fullIntegrationNewDemoScenario(t)
	runDemoStartup(t, scenario)
	scenario.OpenList("redshift")

	type issueCase struct {
		id     string
		issues []string // nil = no Attention section at all.
	}
	cases := []issueCase{
		// Healthy rows with no finding → Attention section must be absent.
		{demofixtures.AcmeWarehouseID, nil},
		{demofixtures.AcmeReportingID, nil},
		{demofixtures.StagingDwhID, nil},
		{demofixtures.RedshiftMaintenanceDeferredExpiredID, nil},
		// Wave-1 single-phrase rows.
		{demofixtures.RedshiftResizingID, []string{"Resizing"}},
		{demofixtures.RedshiftRebootingID, []string{"Rebooting"}},
		{demofixtures.RedshiftIncompatibleNetworkID, []string{"Broken: incompatible-network"}},
		{demofixtures.RedshiftHardwareFailureID, []string{"Broken: hardware-failure"}},
		{demofixtures.RedshiftStorageFullID, []string{"Broken: storage-full"}},
		{demofixtures.RedshiftAvailUnavailableID, []string{"Unavailable"}},
		{demofixtures.RedshiftAvailFailedID, []string{"Failed"}},
		{demofixtures.RedshiftAvailMaintenanceID, []string{"Maintenance"}},
		{demofixtures.RedshiftAvailModifyingID, []string{"Modifying"}},
		{demofixtures.RedshiftPendingChangeID, []string{"Pending change queued"}},
		{demofixtures.RedshiftMaintenanceDeferredID, []string{"Maintenance deferred"}},
		{demofixtures.RedshiftPubliclyAccessibleID, []string{"Publicly accessible"}},
		{demofixtures.RedshiftUnencryptedID, []string{"Unencrypted at rest"}},
		// Multi-W1: every phrase appears under Attention in §4 precedence order.
		{demofixtures.WarnRedshiftMultiID, []string{"Pending change queued", "Publicly accessible", "Unencrypted at rest"}},
		{demofixtures.WarnRedshiftTwoID, []string{"Publicly accessible", "Unencrypted at rest"}},
		// U8 — Broken suppresses the Warnings, so only the Broken phrase appears.
		{demofixtures.RedshiftBrokenWithWarningHiddenID, []string{"Broken: storage-full"}},
		{demofixtures.RedshiftAvailUnavailableWithWarningHiddenID, []string{"Unavailable"}},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.id, func(t *testing.T) {
			res := selectRedshiftByID(t, scenario, tc.id)
			scenario.OpenDetailResource("redshift", res)
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

// TestScenario_RedshiftVisual_HealthyRowsHaveNoIssuesPhrases is a dedicated
// regression pin for Healthy silence (spec §4 rule): Healthy rows must not
// render any Wave-1 phrase in the detail view.
func TestScenario_RedshiftVisual_HealthyRowsHaveNoIssuesPhrases(t *testing.T) {
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
	scenario := fullIntegrationNewDemoScenario(t)
	runDemoStartup(t, scenario)
	scenario.OpenList("redshift")

	wave1Phrases := []string{
		"resizing", "rebooting",
		"broken: incompatible-", "broken: hardware-failure", "broken: storage-full",
		"unavailable", "failed", "maintenance", "modifying",
		"pending change queued", "maintenance deferred",
		"publicly accessible", "unencrypted at rest",
	}

	for _, id := range []string{
		demofixtures.AcmeWarehouseID,
		demofixtures.AcmeReportingID,
		demofixtures.StagingDwhID,
		demofixtures.RedshiftMaintenanceDeferredExpiredID,
	} {
		id := id
		t.Run(id, func(t *testing.T) {
			res := selectRedshiftByID(t, scenario, id)
			scenario.OpenDetailResource("redshift", res)
			scenario.ExpectNoAPIError()
			view := scenario.currentView()
			t.Log("\n" + view)

			expectNoAttentionSection(t, view)
			for _, phrase := range wave1Phrases {
				for _, line := range strings.Split(view, "\n") {
					// Skip RELATED panel rows ("CloudFormation", etc.) which may
					// legitimately contain substrings like "modifying" inside a
					// stack-status column. Attention is section-header-gated by the
					// preceding assertion; we're specifically guarding against the
					// phrase leaking into any non-header context on a Healthy row.
					if strings.Contains(line, phrase) {
						t.Errorf("Healthy row %q unexpectedly contains Wave-1 phrase %q in line: %q\nfull view:\n%s",
							id, phrase, line, view)
					}
				}
			}
			scenario.Back()
		})
	}
}

// selectRedshiftByID looks up a concrete redshift resource from the demo clients.
func selectRedshiftByID(t *testing.T, s *fullIntegrationScenario, id string) resource.Resource {
	t.Helper()
	return fullIntegrationMustFindResourceByID(t, s.clients, "redshift", id)
}
