package unit

import (
	"testing"

	"github.com/k2m30/a9s/v3/internal/resource"
)

// TestDbcColor tests the Color function for DB Clusters (dbc).
//
// The production Color func reads "status" and delegates to rdsInstanceColor.
// It also honors has_writer, deletion_protection, storage_encrypted, and
// backup_retention_period — all tested below.
func TestDbcColor(t *testing.T) {
	td := resource.FindResourceType("dbc")
	if td == nil {
		t.Fatal("dbc not registered")
	}

	cases := []struct {
		name   string
		fields map[string]string
		want   resource.Color
	}{
		// --- ColorHealthy ---
		{
			name:   "available",
			fields: map[string]string{"status": "available"},
			want:   resource.ColorHealthy,
		},
		{
			name:   "empty_status",
			fields: map[string]string{"status": ""},
			want:   resource.ColorHealthy,
		},
		{
			name:   "nil_fields",
			fields: nil,
			want:   resource.ColorHealthy,
		},

		// --- ColorBroken ---
		{
			name:   "failed",
			fields: map[string]string{"status": "failed"},
			want:   resource.ColorBroken,
		},
		{
			name:   "inaccessible_encryption_credentials",
			fields: map[string]string{"status": "inaccessible-encryption-credentials"},
			want:   resource.ColorBroken,
		},
		{
			name:   "incompatible_parameters",
			fields: map[string]string{"status": "incompatible-parameters"},
			want:   resource.ColorBroken,
		},
		{
			name:   "storage_full",
			fields: map[string]string{"status": "storage-full"},
			want:   resource.ColorBroken,
		},
		{
			name:   "restore_error",
			fields: map[string]string{"status": "restore-error"},
			want:   resource.ColorBroken,
		},
		{
			name:   "incompatible_network",
			fields: map[string]string{"status": "incompatible-network"},
			want:   resource.ColorBroken,
		},

		// --- ColorWarning ---
		{
			name:   "creating",
			fields: map[string]string{"status": "creating"},
			want:   resource.ColorWarning,
		},
		{
			name:   "modifying",
			fields: map[string]string{"status": "modifying"},
			want:   resource.ColorWarning,
		},
		{
			name:   "backing_up",
			fields: map[string]string{"status": "backing-up"},
			want:   resource.ColorWarning,
		},
		{
			name:   "rebooting",
			fields: map[string]string{"status": "rebooting"},
			want:   resource.ColorWarning,
		},
		{
			name:   "upgrading",
			fields: map[string]string{"status": "upgrading"},
			want:   resource.ColorWarning,
		},
		{
			name:   "stopping",
			fields: map[string]string{"status": "stopping"},
			want:   resource.ColorWarning,
		},
		{
			name:   "starting",
			fields: map[string]string{"status": "starting"},
			want:   resource.ColorWarning,
		},
		{
			name:   "deleting",
			fields: map[string]string{"status": "deleting"},
			want:   resource.ColorWarning,
		},

		// --- field-honoring: has_writer=false → Broken ---
		{
			name:   "no_writer",
			fields: map[string]string{"status": "available", "has_writer": "false"},
			want:   resource.ColorBroken,
		},
		{
			// has_writer=false takes precedence over deletion_protection warning
			name:   "no_writer_overrides_warnings",
			fields: map[string]string{"status": "available", "has_writer": "false", "deletion_protection": "false"},
			want:   resource.ColorBroken,
		},

		// --- field-honoring: Warning (has_writer=true, one bad field) ---
		{
			name:   "has_writer_unencrypted",
			fields: map[string]string{"status": "available", "has_writer": "true", "storage_encrypted": "false"},
			want:   resource.ColorWarning,
		},
		{
			name:   "has_writer_no_deletion_protection",
			fields: map[string]string{"status": "available", "has_writer": "true", "deletion_protection": "false"},
			want:   resource.ColorWarning,
		},
		{
			name:   "has_writer_no_backups",
			fields: map[string]string{"status": "available", "has_writer": "true", "backup_retention_period": "0"},
			want:   resource.ColorWarning,
		},

		// --- all good fields → Healthy ---
		{
			name: "healthy_full",
			fields: map[string]string{
				"status":                  "available",
				"has_writer":              "true",
				"deletion_protection":     "true",
				"storage_encrypted":       "true",
				"backup_retention_period": "7",
			},
			want: resource.ColorHealthy,
		},

		// --- has_writer field absent (legacy/unset) → treated as unknown, not penalised ---
		{
			name:   "has_writer_unset_treated_unknown",
			fields: map[string]string{"status": "available"},
			want:   resource.ColorHealthy,
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
