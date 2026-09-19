package unit

// The ACM Certificates Color function reads findings
// only.
//
// Colour derives from findings, so a resource carrying no findings is
// Healthy whatever its fields say; the state each row names is reported by
// the finding the fetcher emits for it (see prowler_w6a_*_test.go). The
// table enumerates the states that must not colour a row on their own, so a
// raw-field branch is what it catches.

import (
	"testing"

	"github.com/k2m30/a9s/v3/core/resource"
)

func TestACMColor(t *testing.T) {
	td := resource.FindResourceType("acm")
	if td == nil {
		t.Fatal("acm type not registered")
	}

	cases := []struct {
		name   string
		fields map[string]string
		want   resource.Color
	}{
		{
			name:   "issued_in_use_long_expiry",
			fields: map[string]string{"status": "ISSUED", "days_left": "90 days", "in_use": "true"},
			want:   resource.ColorHealthy,
		},
		{
			name:   "issued_expiring_within_30d",
			fields: map[string]string{"status": "ISSUED", "days_left": "20 days", "in_use": "true"},
			want:   resource.ColorWarning,
		},
		{
			name:   "issued_expiring_within_7d",
			fields: map[string]string{"status": "ISSUED", "days_left": "5 days", "in_use": "true"},
			want:   resource.ColorBroken,
		},
		{
			// Fetcher writes "expired" when the cert has already passed its expiry.
			name:   "issued_expired_literal",
			fields: map[string]string{"status": "ISSUED", "days_left": "expired", "in_use": "true"},
			want:   resource.ColorBroken,
		},
		{
			name:   "issued_orphan",
			fields: map[string]string{"status": "ISSUED", "days_left": "90 days", "in_use": "false"},
			want:   resource.ColorWarning,
		},
		{
			name:   "issued_expiring_and_orphan",
			fields: map[string]string{"status": "ISSUED", "days_left": "20 days", "in_use": "false"},
			want:   resource.ColorWarning,
		},
		{
			name:   "issued_broken_beats_orphan",
			fields: map[string]string{"status": "ISSUED", "days_left": "5 days", "in_use": "false"},
			want:   resource.ColorBroken,
		},
		{
			name:   "pending_validation_still_warning",
			fields: map[string]string{"status": "PENDING_VALIDATION"},
			want:   resource.ColorWarning,
		},
		{
			name:   "expired_status_broken",
			fields: map[string]string{"status": "EXPIRED"},
			want:   resource.ColorBroken,
		},
		{
			name:   "inactive_status_dim",
			fields: map[string]string{"status": "INACTIVE"},
			want:   resource.ColorDim,
		},
		{
			name:   "empty",
			fields: map[string]string{},
			want:   resource.ColorHealthy,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := td.Color(resource.Resource{Fields: tc.fields})
			if got != resource.ColorHealthy {
				t.Errorf("Color(fields=%v) = %v, want ColorHealthy; the raw-field branch that returned %v is gone", tc.fields, got, tc.want)
			}
		})
	}
}
