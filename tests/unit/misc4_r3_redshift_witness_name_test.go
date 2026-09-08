package unit

// misc4_r3_redshift_witness_name_test.go — misc4 row 3.
//
// A demo row named for a condition is read as a witness for it. This one was
// named "redshift-expired-window" while carrying no finding at all, so the
// demo list showed a row promising a signal beside a blank Status cell.
//
// The AWS field settles it: hasActiveDeferredMaintenanceWindow
// (core/aws/redshift.go) reports a deferral only while `now` falls inside
// [DeferMaintenanceStartTime, DeferMaintenanceEndTime]. A window whose
// DeferMaintenanceEndTime has passed is a deferral that has lapsed — the
// cluster's maintenance is no longer being held off, which is the normal
// state and not a posture item. There is no wave-1 check to add. The row is
// the negative control for redshift.maintenance-deferred, and its name now
// says so.

import (
	"strings"
	"testing"

	"github.com/k2m30/a9s/v3/core/demo/fixtures"
)

// TestRedshiftLapsedDeferralWitnessIsNamedForWhatItShows pins the rename and
// the behaviour it describes: the row carries no finding, and its ID does not
// promise one.
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
