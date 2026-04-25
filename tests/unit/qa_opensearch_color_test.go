package unit

// qa_opensearch_color_test.go — Color-function tests for the opensearch resource type.
//
// Tests construct a minimal resource.Resource with documented Fields and assert
// resource.FindResourceType("opensearch").ResolveColor(r) returns the expected
// resource.Color. Per impl-plan §1.2: the Color func must strip (+N) suffix before
// matching, and background signals (! and ~) must NOT flip row color.

import (
	"testing"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func TestOpenSearchColor(t *testing.T) {
	td := resource.FindResourceType("opensearch")
	if td == nil {
		t.Fatal("opensearch not registered")
	}

	cases := []struct {
		name   string
		status string
		fields map[string]string
		want   resource.Color
	}{
		// --- ColorHealthy ---
		{
			// healthy: blank status, no flags — green silence
			name:   "healthy_blank",
			status: "",
			fields: map[string]string{"status": ""},
			want:   resource.ColorHealthy,
		},
		{
			// update_available_alone: ! finding does NOT flip color — glyph does
			name:   "update_available_alone",
			status: "software update forced soon",
			fields: map[string]string{
				"status":                            "software update forced soon",
				"service_software_update_available": "true",
				"deleted":                           "false",
				"processing":                        "false",
				"upgrade_processing":                "false",
				"domain_processing_status":          "Active",
			},
			want: resource.ColorHealthy,
		},
		{
			// encryption_off_alone: ~ finding stays green
			name:   "encryption_off_alone",
			status: "encryption at rest off",
			fields: map[string]string{
				"status":                   "encryption at rest off",
				"encryption_at_rest_enabled": "false",
				"deleted":                  "false",
				"processing":               "false",
				"upgrade_processing":       "false",
				"domain_processing_status": "Active",
			},
			want: resource.ColorHealthy,
		},

		// --- ColorBroken ---
		{
			// isolated: strip (+N) suffix; isolated wins
			name:   "isolated",
			status: "isolated: quarantined by AWS",
			fields: map[string]string{
				"status":                   "isolated: quarantined by AWS",
				"domain_processing_status": "Isolated",
				"deleted":                  "false",
				"processing":               "false",
				"upgrade_processing":       "false",
			},
			want: resource.ColorBroken,
		},
		{
			// isolated_plus_update_available: (+1) suffix stripped; isolated wins
			name:   "isolated_plus_update_available",
			status: "isolated: quarantined by AWS (+1)",
			fields: map[string]string{
				"status":                            "isolated: quarantined by AWS (+1)",
				"domain_processing_status":          "Isolated",
				"service_software_update_available": "true",
				"deleted":                           "false",
				"processing":                        "false",
				"upgrade_processing":                "false",
			},
			want: resource.ColorBroken,
		},

		// --- ColorWarning ---
		{
			// processing: Processing=true
			name:   "processing",
			status: "processing: config change in flight",
			fields: map[string]string{
				"status":                   "processing: config change in flight",
				"processing":               "true",
				"deleted":                  "false",
				"upgrade_processing":       "false",
				"domain_processing_status": "Modifying",
			},
			want: resource.ColorWarning,
		},
		{
			// upgrade_processing: UpgradeProcessing=true
			name:   "upgrade_processing",
			status: "processing: config change in flight",
			fields: map[string]string{
				"status":                   "processing: config change in flight",
				"upgrade_processing":       "true",
				"processing":               "false",
				"deleted":                  "false",
				"domain_processing_status": "Upgrading",
			},
			want: resource.ColorWarning,
		},
		{
			// processing_plus_encryption_off: warning wins
			name:   "processing_plus_encryption_off",
			status: "processing: config change in flight (+1)",
			fields: map[string]string{
				"status":                     "processing: config change in flight (+1)",
				"processing":                 "true",
				"encryption_at_rest_enabled": "false",
				"deleted":                    "false",
				"upgrade_processing":         "false",
				"domain_processing_status":   "Modifying",
			},
			want: resource.ColorWarning,
		},

		// --- ColorDim ---
		{
			// deleted=true → dim row
			name:   "deleted",
			status: "deleting: removal in progress",
			fields: map[string]string{
				"status":  "deleting: removal in progress",
				"deleted": "true",
			},
			want: resource.ColorDim,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := resource.Resource{
				ID:     "test-domain",
				Name:   "test-domain",
				Status: tc.status,
				Fields: tc.fields,
			}
			got := td.ResolveColor(r)
			if got != tc.want {
				t.Errorf("ResolveColor(%q) = %v, want %v", tc.status, got, tc.want)
			}
		})
	}
}
