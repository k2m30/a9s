package unit

import (
	"testing"
	"time"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func TestEbsColor(t *testing.T) {
	td := resource.FindResourceType("ebs")
	if td == nil {
		t.Fatal("ebs not registered")
	}

	tenDaysAgo := time.Now().AddDate(0, 0, -10).Format("2006-01-02 15:04")
	now := time.Now().Format("2006-01-02 15:04")

	cases := []struct {
		name   string
		fields map[string]string
		want   resource.Color
	}{
		{
			name:   "in_use",
			fields: map[string]string{"state": "in-use", "encrypted": "true"},
			want:   resource.ColorHealthy,
		},
		{
			name:   "in_use_unencrypted",
			fields: map[string]string{"state": "in-use", "encrypted": "false"},
			want:   resource.ColorWarning,
		},
		{
			name:   "available_attached_recent",
			fields: map[string]string{"state": "available", "attached_to": "", "created": now},
			want:   resource.ColorHealthy,
		},
		{
			name:   "available_orphan_old",
			fields: map[string]string{"state": "available", "attached_to": "", "created": tenDaysAgo},
			want:   resource.ColorWarning,
		},
		{
			name:   "available_attached",
			fields: map[string]string{"state": "available", "attached_to": "i-abc", "created": tenDaysAgo},
			want:   resource.ColorHealthy,
		},
		{
			name:   "creating",
			fields: map[string]string{"state": "creating"},
			want:   resource.ColorWarning,
		},
		{
			name:   "deleting",
			fields: map[string]string{"state": "deleting"},
			want:   resource.ColorWarning,
		},
		{
			name:   "error",
			fields: map[string]string{"state": "error"},
			want:   resource.ColorBroken,
		},
		{
			name:   "broken_overrides_orphan",
			fields: map[string]string{"state": "error", "attached_to": "", "created": tenDaysAgo},
			want:   resource.ColorBroken,
		},
		{
			name:   "empty",
			fields: map[string]string{"state": ""},
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
