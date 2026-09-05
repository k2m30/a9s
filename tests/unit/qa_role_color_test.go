package unit

// qa_role_color_test.go — Wave 1 Color tests for IAM Roles.
//
// Contract:
//   - The row color is derived from the resource's Findings alone. The trust
//     verdict is decided once, in the fetcher, and the classifier reports it —
//     it does not re-read assume_role_policy_document and reach its own
//     conclusion.
//   - A wildcard-trust finding → ColorBroken.
//   - No finding → ColorHealthy, whatever the raw document says.
//
// The two "star principal" cases previously asserted ColorBroken from the raw
// document with no finding attached. That expectation encoded a second,
// independent trust evaluator inside the classifier, which is exactly what
// could disagree with the Findings list; it is deliberately inverted here,
// not broken.

import (
	"testing"

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

func TestRoleColor(t *testing.T) {
	td := resource.FindResourceType("role")
	if td == nil {
		t.Fatal("role resource type not registered")
	}

	wildcardTrust := []domain.Finding{{
		Code:     "role.trust.wildcard-principal",
		Phrase:   "anyone can assume this role",
		Severity: domain.SevBroken,
		Source:   "wave1",
	}}

	cases := []struct {
		name     string
		fields   map[string]string
		findings []domain.Finding
		want     resource.Color
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
			name: "star_principal_with_finding",
			fields: map[string]string{
				"assume_role_policy_document": `{"Statement":[{"Principal":"*"}]}`,
			},
			findings: wildcardTrust,
			want:     resource.ColorBroken,
		},
		{
			name: "star_principal_without_finding",
			fields: map[string]string{
				"assume_role_policy_document": `{"Statement":[{"Principal": "*"}]}`,
			},
			want: resource.ColorHealthy,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := td.Color(resource.Resource{Fields: tc.fields, Findings: tc.findings})
			if got != tc.want {
				t.Errorf("Color(fields=%v, findings=%d) = %v, want %v",
					tc.fields, len(tc.findings), got, tc.want)
			}
		})
	}
}
