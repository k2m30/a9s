package unit

// qa_policy_color_test.go — Wave 1 Color tests for IAM Policies.
//
// Contract:
//   - attachment_count>0 with is_attachable=true → ColorHealthy (in use).
//   - attachment_count=0 with is_attachable=true → ColorWarning (orphan managed policy).
//   - attachment_count=0 with is_attachable=false (inline/service policy) → ColorHealthy.
//   - No fields present → ColorHealthy.

import (
	"testing"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func TestPolicyColor(t *testing.T) {
	td := resource.FindResourceType("policy")
	if td == nil {
		t.Fatal("policy resource type not registered")
	}

	cases := []struct {
		name   string
		fields map[string]string
		want   resource.Color
	}{
		{
			name: "attached",
			fields: map[string]string{
				"attachment_count": "2",
				"is_attachable":    "true",
			},
			want: resource.ColorHealthy,
		},
		{
			name: "orphan_attachable",
			fields: map[string]string{
				"attachment_count": "0",
				"is_attachable":    "true",
			},
			want: resource.ColorWarning,
		},
		{
			name: "orphan_not_attachable",
			fields: map[string]string{
				"attachment_count": "0",
				"is_attachable":    "false",
			},
			want: resource.ColorHealthy,
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
			if got != tc.want {
				t.Errorf("Color(fields=%v) = %v, want %v", tc.fields, got, tc.want)
			}
		})
	}
}
