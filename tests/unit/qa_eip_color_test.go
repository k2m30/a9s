package unit

import (
	"testing"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

func TestEipColor(t *testing.T) {
	td := resource.FindResourceType("eip")
	if td == nil {
		t.Fatal("eip not registered")
	}

	// d4 row 23: an idle address is warning because the eip fetcher emits
	// CodeEIPUnassociated for it, not because the classifier re-reads two of
	// the three attachment fields. That second derivation called a NAT
	// gateway's address — attached to an interface, with no association id
	// and no instance — unattached, which is how every real one looks. Do not
	// restore a findings-free "idle" case: the fetcher cannot produce one.
	cases := []struct {
		name          string
		associationID string
		instanceID    string
		findings      []domain.Finding
		want          resource.Color
	}{
		{
			name:          "associated",
			associationID: "eipassoc-1",
			instanceID:    "i-abc",
			want:          resource.ColorHealthy,
		},
		{
			name:          "associated_eni_only",
			associationID: "eipassoc-1",
			instanceID:    "",
			want:          resource.ColorHealthy,
		},
		{
			name:          "idle",
			associationID: "",
			instanceID:    "",
			findings: []domain.Finding{{
				Code: awsclient.CodeEIPUnassociated, Phrase: "unassociated",
				Severity: domain.SevWarn, Source: "wave1",
			}},
			want: resource.ColorWarning,
		},
		{
			name:          "nat_gateway_address",
			associationID: "",
			instanceID:    "",
			want:          resource.ColorHealthy,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := td.Color(resource.Resource{
				Fields: map[string]string{
					"association_id": tc.associationID,
					"instance_id":    tc.instanceID,
				},
				Findings: tc.findings,
			})
			if got != tc.want {
				t.Errorf("Color(association_id=%q, instance_id=%q) = %v, want %v",
					tc.associationID, tc.instanceID, got, tc.want)
			}
		})
	}
}
