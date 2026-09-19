//go:build integration

package integration

import (
	"os"
	"strings"
	"testing"
)

// An RDS instance with pending maintenance actions shows the
// "⚠ Background Check" section in its detail view.
//
// Requires a live AWS profile with at least one RDS instance that has pending
// maintenance actions:
//
//	A9S_REPRO_PROFILE=<profile>
//	A9S_REPRO_DBI_ID=<db-instance-identifier>
//	A9S_REPRO_REGION=<region>   (optional — defaults to profile's default region)
func TestEnrichDetailBackgroundCheckVisible(t *testing.T) {
	profile := strings.TrimSpace(os.Getenv("A9S_REPRO_PROFILE"))
	dbiID := strings.TrimSpace(os.Getenv("A9S_REPRO_DBI_ID"))
	region := strings.TrimSpace(os.Getenv("A9S_REPRO_REGION"))
	if profile == "" || dbiID == "" {
		t.Skip("set A9S_REPRO_PROFILE and A9S_REPRO_DBI_ID to run this test")
	}

	scenario := fullIntegrationNewLiveScenario(t, profile, region)
	instance := fullIntegrationMustFindResourceByID(t, scenario.clients, "rds", dbiID)

	scenario.OpenDetailResource("rds", instance)

	scenario.ExpectViewContains("Attention")
}
