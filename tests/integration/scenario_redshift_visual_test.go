//go:build integration

package integration

// scenario_redshift_visual_test.go checks the rendered TUI output (not
// fetcher return values) for redshift against the universal UI rules and
// docs/resources/redshift.md. redshift has no Wave-2 signals.

import (
	"strings"
	"testing"

	demofixtures "github.com/k2m30/a9s/v3/core/demo/fixtures"
	"github.com/k2m30/a9s/v3/core/resource"
)

func TestScenario_RedshiftVisual(t *testing.T) {
	// A user-dir ~/.a9s/views/redshift.yaml overlay wins the merge.
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
	scenario := fullIntegrationNewDemoScenario(t)

	// Drive the real demo startup: Init → ClientsReadyMsg → demoPrefetchCounts
	// → AvailabilityPrefetchedMsg. The enrichment chain is a no-op for
	// redshift, but the prefetch still seeds resourceCache.
	runDemoStartup(t, scenario)

	scenario.OpenList("redshift")

	for _, jargon := range []string{"CIS", " Flags", "NOBKP", "UNENC", "NOPROT", "cis_flags"} {
		scenario.ExpectViewNotContains(jargon)
	}

	scenario.ExpectRowStatusBlank(demofixtures.AcmeWarehouseID)
	scenario.ExpectRowStatusBlank(demofixtures.AcmeReportingID)
	scenario.ExpectRowStatusBlank(demofixtures.StagingDwhID) // paused carries no finding

	// ClusterStatus transitional (Warning)
	scenario.ExpectRowStatusEquals(demofixtures.RedshiftResizingID, "resizing")
	scenario.ExpectRowStatusEquals(demofixtures.RedshiftRebootingID, "rebooting")

	// ClusterStatus broken
	scenario.ExpectRowStatusEquals(demofixtures.RedshiftIncompatibleNetworkID, "subnet group cannot host the cluster")
	scenario.ExpectRowStatusEquals(demofixtures.RedshiftHardwareFailureID, "node hardware failed")
	scenario.ExpectRowStatusEquals(demofixtures.RedshiftStorageFullID, "out of storage")

	// ClusterAvailabilityStatus broken
	scenario.ExpectRowStatusEquals(demofixtures.RedshiftAvailUnavailableID, "unavailable")
	scenario.ExpectRowStatusEquals(demofixtures.RedshiftAvailFailedID, "failed")

	// ClusterAvailabilityStatus warning
	scenario.ExpectRowStatusEquals(demofixtures.RedshiftAvailMaintenanceID, "maintenance")
	// The cluster status and the availability status both read the bare word
	// "modifying", so the availability code carries its own phrase.
	scenario.ExpectRowStatusEquals(demofixtures.RedshiftAvailModifyingID, "modifying — availability affected")

	// Config / maintenance warnings
	scenario.ExpectRowStatusEquals(demofixtures.RedshiftPendingChangeID, "pending change queued")
	scenario.ExpectRowStatusEquals(demofixtures.RedshiftMaintenanceDeferredID, "maintenance deferred")
	scenario.ExpectRowStatusEquals(demofixtures.RedshiftPubliclyAccessibleID, "public endpoint")
	scenario.ExpectRowStatusEquals(demofixtures.RedshiftUnencryptedID, "unencrypted at rest")

	// An expired deferred-maintenance window carries no finding.
	scenario.ExpectRowStatusBlank(demofixtures.RedshiftDeferralLapsedID)

	scenario.ExpectRowStatusEquals(demofixtures.WarnRedshiftMultiID, "pending change queued (+2)")
	scenario.ExpectRowStatusEquals(demofixtures.WarnRedshiftTwoID, "public endpoint (+1)")

	// Broken severity beats Warning. Even when public/unencrypted warnings
	// coexist with a Broken ClusterStatus / ClusterAvailabilityStatus, only the
	// Broken phrase surfaces; no `(+N)` suffix.
	scenario.ExpectRowStatusEquals(demofixtures.RedshiftBrokenWithWarningHiddenID, "out of storage")
	scenario.ExpectRowStatusEquals(demofixtures.RedshiftAvailUnavailableWithWarningHiddenID, "unavailable")

	// Non-green rows never carry a `!` / `~` glyph regardless of finding.
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
		demofixtures.AcmeWarehouseID,
		demofixtures.AcmeReportingID,
		demofixtures.StagingDwhID,
		demofixtures.RedshiftDeferralLapsedID,
	} {
		scenario.ExpectRowNoGlyphPrefix(id)
	}

	// redshift has no Wave-2 `!` findings, so the badge is 0.
	scenario.ExpectMenuIssueCount("redshift", 0)

	// Redshift logging destinations are mutually exclusive: acme-warehouse logs
	// to CloudWatch and acme-reporting to S3, so each covers one of the two
	// pivots.
	warehouse := selectRedshiftByID(t, scenario, demofixtures.AcmeWarehouseID)
	scenario.OpenDetailResource("redshift", warehouse)
	scenario.ExpectNoAPIError()
	for _, displayName := range []string{
		"CW Alarms", "Security Groups", "VPC", "IAM Role", "KMS Key",
		"CloudFormation", "Secrets Manager", "Log Groups", "Subnets",
	} {
		scenario.ExpectRelatedRowCountAtLeast(displayName, 1)
	}

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

	scenario.Back()
	multi := selectRedshiftByID(t, scenario, demofixtures.WarnRedshiftMultiID)
	scenario.OpenDetailResource("redshift", multi)
	scenario.ExpectNoAPIError()
	// Attention section capitalizes the first rune at render time; data stays lowercase.
	expectAttentionSection(t, scenario.currentView(), []string{
		"Pending change queued",
		"Public endpoint",
		"Unencrypted at rest",
	})
}

// TestScenario_RedshiftVisual_DetailSurfacesAllIssues asserts every finding
// is individually visible in the detail view.
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
		{demofixtures.AcmeWarehouseID, nil},
		{demofixtures.AcmeReportingID, nil},
		{demofixtures.StagingDwhID, nil},
		{demofixtures.RedshiftDeferralLapsedID, nil},
		{demofixtures.RedshiftResizingID, []string{"Resizing"}},
		{demofixtures.RedshiftRebootingID, []string{"Rebooting"}},
		{demofixtures.RedshiftIncompatibleNetworkID, []string{"Subnet group cannot host the cluster"}},
		{demofixtures.RedshiftHardwareFailureID, []string{"Node hardware failed"}},
		{demofixtures.RedshiftStorageFullID, []string{"Out of storage"}},
		{demofixtures.RedshiftAvailUnavailableID, []string{"Unavailable"}},
		{demofixtures.RedshiftAvailFailedID, []string{"Failed"}},
		{demofixtures.RedshiftAvailMaintenanceID, []string{"Maintenance"}},
		{demofixtures.RedshiftAvailModifyingID, []string{"Modifying"}},
		{demofixtures.RedshiftPendingChangeID, []string{"Pending change queued"}},
		{demofixtures.RedshiftMaintenanceDeferredID, []string{"Maintenance deferred"}},
		{demofixtures.RedshiftPubliclyAccessibleID, []string{"Public endpoint"}},
		{demofixtures.RedshiftUnencryptedID, []string{"Unencrypted at rest"}},
		{demofixtures.WarnRedshiftMultiID, []string{"Pending change queued", "Public endpoint", "Unencrypted at rest"}},
		{demofixtures.WarnRedshiftTwoID, []string{"Public endpoint", "Unencrypted at rest"}},
		// Broken suppresses the Warnings.
		{demofixtures.RedshiftBrokenWithWarningHiddenID, []string{"Out of storage"}},
		{demofixtures.RedshiftAvailUnavailableWithWarningHiddenID, []string{"Unavailable"}},
	}

	for _, tc := range cases {
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

// TestScenario_RedshiftVisual_HealthyRowsHaveNoIssuesPhrases pins Healthy
// silence: Healthy rows render no Wave-1 phrase in the detail view.
func TestScenario_RedshiftVisual_HealthyRowsHaveNoIssuesPhrases(t *testing.T) {
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
	scenario := fullIntegrationNewDemoScenario(t)
	runDemoStartup(t, scenario)
	scenario.OpenList("redshift")

	wave1Phrases := []string{
		"resizing", "rebooting",
		"parameter group rejected", "restore did not complete",
		"encryption key store unreachable", "subnet group cannot host the cluster",
		"node hardware failed", "out of storage",
		"unavailable", "failed", "maintenance", "modifying",
		"pending change queued", "maintenance deferred",
		"public endpoint", "unencrypted at rest",
	}

	for _, id := range []string{
		demofixtures.AcmeWarehouseID,
		demofixtures.AcmeReportingID,
		demofixtures.StagingDwhID,
		demofixtures.RedshiftDeferralLapsedID,
	} {
		t.Run(id, func(t *testing.T) {
			res := selectRedshiftByID(t, scenario, id)
			scenario.OpenDetailResource("redshift", res)
			scenario.ExpectNoAPIError()
			view := scenario.currentView()
			t.Log("\n" + view)

			expectNoAttentionSection(t, view)
			// The per-line phrase scan catches a phrase leaking into a
			// non-Attention context on a Healthy row. It skips the RELATED
			// panel, which legitimately
			// carries substrings like "modifying" inside stack-status columns
			// (e.g. CloudFormation StackStatus=UPDATE_IN_PROGRESS) or
			// "maintenance" in display names (e.g. "Maintenance deferred"
			// pivot label on a parent that happens to be Healthy).
			detailBody := view
			if idx := strings.Index(detailBody, "RELATED"); idx >= 0 {
				detailBody = detailBody[:idx]
			}
			for _, phrase := range wave1Phrases {
				for _, line := range strings.Split(detailBody, "\n") {
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
