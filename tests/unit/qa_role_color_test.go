package unit

// qa_role_color_test.go — Wave 1 Color tests for IAM Roles.
//
// Contract:
//   - No assume_role_policy_document field → ColorHealthy.
//   - Principal is a service (not a wildcard) → ColorHealthy.
//   - Principal is "*" (star) → ColorBroken (overly permissive trust policy).
//   - Principal is "*" with extra whitespace → ColorBroken.

import (
	"testing"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func TestRoleColor(t *testing.T) {
	td := resource.FindResourceType("role")
	if td == nil {
		t.Fatal("role resource type not registered")
	}

	cases := []struct {
		name   string
		fields map[string]string
		want   resource.Color
	}{
		{
			name:   "no_doc",
			fields: map[string]string{},
			want:   resource.ColorHealthy,
		},
		{
			name: "safe_principal",
			fields: map[string]string{
				"assume_role_policy_document": `{"Statement":[{"Principal":{"Service":"ec2.amazonaws.com"}}]}`,
			},
			want: resource.ColorHealthy,
		},
		{
			name: "star_principal",
			fields: map[string]string{
				"assume_role_policy_document": `{"Statement":[{"Principal":"*"}]}`,
			},
			want: resource.ColorBroken,
		},
		{
			name: "star_principal_spaced",
			fields: map[string]string{
				"assume_role_policy_document": `{"Statement":[{"Principal": "*"}]}`,
			},
			want: resource.ColorBroken,
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
