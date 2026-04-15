package unit

import (
	"testing"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func TestIgwColor(t *testing.T) {
	td := resource.FindResourceType("igw")
	if td == nil {
		t.Fatal("igw not registered")
	}

	cases := []struct {
		name   string
		fields map[string]string
		want   resource.Color
	}{
		{
			name:   "attached",
			fields: map[string]string{"attachments_count": "1", "state": "attached"},
			want:   resource.ColorHealthy,
		},
		{
			name:   "orphan",
			fields: map[string]string{"attachments_count": "0", "state": "detached"},
			want:   resource.ColorWarning,
		},
		{
			name:   "attaching",
			fields: map[string]string{"attachments_count": "1", "state": "attaching"},
			want:   resource.ColorWarning,
		},
		{
			name:   "detaching",
			fields: map[string]string{"attachments_count": "1", "state": "detaching"},
			want:   resource.ColorWarning,
		},
		{
			name:   "zero_attachments_overrides_state",
			fields: map[string]string{"attachments_count": "0", "state": "attached"},
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
