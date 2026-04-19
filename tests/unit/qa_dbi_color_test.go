package unit

import (
	"testing"

	"github.com/k2m30/a9s/v3/internal/resource"
)

// TestDbiColor tests the Color function for DB Instances (dbi).
//
// The production Color func reads "status" (the canonical fetcher key; the
// legacy "db_instance_status" fallback was removed in #284) and delegates to
// rdsInstanceColor. It also honors backup_retention_period, publicly_accessible,
// storage_encrypted, and deletion_protection — all tested below.
func TestDbiColor(t *testing.T) {
	td := resource.FindResourceType("dbi")
	if td == nil {
		t.Fatal("dbi not registered")
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
			name:   "storage_full",
			fields: map[string]string{"status": "storage-full"},
			want:   resource.ColorBroken,
		},
		{
			name:   "incompatible_parameters",
			fields: map[string]string{"status": "incompatible-parameters"},
			want:   resource.ColorBroken,
		},
		{
			name:   "inaccessible_encryption_credentials",
			fields: map[string]string{"status": "inaccessible-encryption-credentials"},
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

		// --- legacy key is now ignored; only "status" drives Color (#284) ---
		{
			name:   "legacy_db_instance_status_alone_is_ignored",
			fields: map[string]string{"db_instance_status": "failed"},
			want:   resource.ColorHealthy,
		},
		{
			name:   "status_takes_priority_over_legacy_key",
			fields: map[string]string{"status": "available", "db_instance_status": "failed"},
			want:   resource.ColorHealthy,
		},

		// --- broken overrides all field-level warnings ---
		{
			name:   "broken_status_failed",
			fields: map[string]string{"status": "failed"},
			want:   resource.ColorBroken,
		},

		// --- field-honoring: Warning ---
		{
			name:   "no_backups",
			fields: map[string]string{"status": "available", "backup_retention_period": "0"},
			want:   resource.ColorWarning,
		},
		{
			name:   "publicly_accessible",
			fields: map[string]string{"status": "available", "publicly_accessible": "true"},
			want:   resource.ColorWarning,
		},
		{
			name:   "unencrypted",
			fields: map[string]string{"status": "available", "storage_encrypted": "false"},
			want:   resource.ColorWarning,
		},
		{
			name:   "no_deletion_protection",
			fields: map[string]string{"status": "available", "deletion_protection": "false"},
			want:   resource.ColorWarning,
		},

		// --- Broken status overrides field-level Warning ---
		{
			name:   "broken_overrides_warning",
			fields: map[string]string{"status": "failed", "publicly_accessible": "true"},
			want:   resource.ColorBroken,
		},

		// --- all good fields → Healthy ---
		{
			name: "healthy_with_all_fields",
			fields: map[string]string{
				"status":                  "available",
				"backup_retention_period": "7",
				"publicly_accessible":     "false",
				"storage_encrypted":       "true",
				"deletion_protection":     "true",
			},
			want: resource.ColorHealthy,
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
