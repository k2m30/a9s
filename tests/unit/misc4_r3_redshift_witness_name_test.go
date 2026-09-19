package unit

// A demo row named for a condition is read as evidence of it.
//
// hasActiveDeferredMaintenanceWindow (core/aws/redshift.go) reports a
// deferral only while `now` falls inside [DeferMaintenanceStartTime,
// DeferMaintenanceEndTime]. A window whose DeferMaintenanceEndTime has passed
// is a lapsed deferral: the normal state, not a posture item. The row is the
// negative control for redshift.maintenance-deferred.

import (
	"strings"
	"testing"

	"github.com/k2m30/a9s/v3/core/demo/fixtures"
)

// TestRedshiftLapsedDeferralWitnessIsNamedForWhatItShows pins that the row
// carries no finding and its ID does not promise one.
func TestRedshiftLapsedDeferralWitnessIsNamedForWhatItShows(t *testing.T) {
	id := fixtures.RedshiftDeferralLapsedID
	if strings.Contains(id, "expired-window") {
		t.Errorf("fixture ID %q still names a condition no finding reports", id)
	}
	r := fetchSingleCluster(t, redshiftFixtureByID(t, id))
	if len(r.Findings) != 0 {
		t.Errorf("%s carries %d finding(s); it is the negative control for a "+
			"lapsed deferral and must carry none: %+v", id, len(r.Findings), r.Findings)
	}
}
