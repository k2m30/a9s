package unit

import (
	"testing"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func TestEipColor(t *testing.T) {
	td := resource.FindResourceType("eip")
	if td == nil {
		t.Fatal("eip not registered")
	}

	cases := []struct {
		name          string
		associationID string
		instanceID    string
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
			want:          resource.ColorWarning,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := td.Color(resource.Resource{Fields: map[string]string{
				"association_id": tc.associationID,
				"instance_id":    tc.instanceID,
			}})
			if got != tc.want {
				t.Errorf("Color(association_id=%q, instance_id=%q) = %v, want %v",
					tc.associationID, tc.instanceID, got, tc.want)
			}
		})
	}
}
