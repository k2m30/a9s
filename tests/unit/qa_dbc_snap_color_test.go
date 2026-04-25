package unit

import (
	"testing"
	"time"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func TestDbcSnapColor(t *testing.T) {
	td := resource.FindResourceType("dbc-snap")
	if td == nil {
		t.Fatal("dbc-snap not registered")
	}

	twoYearsAgo := time.Now().AddDate(-2, 0, 0).Format("2006-01-02 15:04")
	recentTime := time.Now().Format("2006-01-02 15:04")

	cases := []struct {
		name   string
		fields map[string]string
		want   resource.Color
	}{
		{
			name:   "available",
			fields: map[string]string{"status": "available", "storage_encrypted": "true"},
			want:   resource.ColorHealthy,
		},
		{
			name:   "creating",
			fields: map[string]string{"status": "creating", "storage_encrypted": "true"},
			want:   resource.ColorWarning,
		},
		{
			name:   "failed",
			fields: map[string]string{"status": "failed"},
			want:   resource.ColorBroken,
		},
		{
			name:   "unencrypted",
			fields: map[string]string{"status": "available", "storage_encrypted": "false"},
			want:   resource.ColorWarning,
		},
		{
			name: "manual_old",
			fields: map[string]string{
				"status":               "available",
				"snapshot_type":        "manual",
				"snapshot_create_time": twoYearsAgo,
			},
			want: resource.ColorWarning,
		},
		{
			name: "automated_old",
			fields: map[string]string{
				"status":               "available",
				"snapshot_type":        "automated",
				"snapshot_create_time": twoYearsAgo,
			},
			want: resource.ColorHealthy,
		},
		{
			name: "manual_recent",
			fields: map[string]string{
				"status":               "available",
				"snapshot_type":        "manual",
				"snapshot_create_time": recentTime,
			},
			want: resource.ColorHealthy,
		},
		{
			name:   "broken_overrides_unencrypted",
			fields: map[string]string{"status": "failed", "storage_encrypted": "false"},
			want:   resource.ColorBroken,
		},
		{
			name: "invalid_date_ignored",
			fields: map[string]string{
				"status":               "available",
				"snapshot_type":        "manual",
				"snapshot_create_time": "garbage",
			},
			want: resource.ColorHealthy,
		},
		{
			name:   "empty",
			fields: map[string]string{"status": ""},
			want:   resource.ColorHealthy,
		},
		// --- Regression pins for Issue 2: dbc-snap Color blind to enriched phrases ---
		// Bug: dbc-snap Color only matches exact "failed" and "creating".
		// Cross-ref enricher writes "failed (+1)" (multi-finding suffix) and AWS DocDB
		// writes "incompatible-restore" / "incompatible-parameters" — all slip through
		// as ColorHealthy. These FAIL today; they pass once the fix applies
		// StripFindingSuffix + strings.HasPrefix("incompatible-") to dbc-snap.Color.
		{
			// "failed (+1)" has the (+1) suffix appended by BumpFindingSuffix when a
			// second finding is added. StripFindingSuffix must strip it so the base
			// phrase "failed" still maps to ColorBroken.
			// FAILS today: switch only matches exact "failed", returns ColorHealthy.
			// DBC-SNAP-COLOR-BLIND BUG: enriched "failed (+1)" slips through as green
			name:   "failed_with_finding_suffix",
			fields: map[string]string{"status": "failed (+1)"},
			want:   resource.ColorBroken,
		},
		{
			// AWS status "incompatible-restore" is a hard failure for DocDB cluster snapshots.
			// dbi-snap Color handles this via strings.HasPrefix(phrase, "incompatible-");
			// dbc-snap Color does not — it returns ColorHealthy.
			// FAILS today: returns ColorHealthy. DBC-SNAP-COLOR-BLIND BUG.
			name:   "incompatible_restore",
			fields: map[string]string{"status": "incompatible-restore"},
			want:   resource.ColorBroken,
		},
		{
			// FAILS today: returns ColorHealthy. DBC-SNAP-COLOR-BLIND BUG.
			name:   "incompatible_parameters",
			fields: map[string]string{"status": "incompatible-parameters"},
			want:   resource.ColorBroken,
		},
		{
			// "creating: 47%" is a progress phrase that does NOT match the switch's
			// exact "creating", so it falls through to the non-broken, non-empty path.
			// With the fix this should map to ColorWarning (non-empty, non-broken phrase).
			// FAILS today: "creating: 47%" does not match exact "creating", falls to
			// default path — but the default in the current switch is no match, so the
			// function reaches the unencrypted/date checks and returns ColorHealthy.
			// DBC-SNAP-COLOR-BLIND BUG: "creating: 47%" treated as healthy.
			name:   "creating_with_percent_suffix",
			fields: map[string]string{"status": "creating: 47%"},
			want:   resource.ColorWarning,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := td.Color(resource.Resource{Fields: tc.fields})
			if got != tc.want {
				t.Errorf("Color(%v) = %v, want %v", tc.fields, got, tc.want)
			}
		})
	}
}
