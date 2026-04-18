package unit

import (
	"testing"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func TestVpceColor(t *testing.T) {
	td := resource.FindResourceType("vpce")
	if td == nil {
		t.Fatal("vpce not registered")
	}

	cases := []struct {
		name  string
		state string
		want  resource.Color
	}{
		{name: "Available", state: "Available", want: resource.ColorHealthy},
		{name: "PendingAcceptance", state: "PendingAcceptance", want: resource.ColorWarning},
		{name: "Pending", state: "Pending", want: resource.ColorWarning},
		{name: "Deleting", state: "Deleting", want: resource.ColorWarning},
		{name: "Failed", state: "Failed", want: resource.ColorBroken},
		{name: "Rejected", state: "Rejected", want: resource.ColorBroken},
		{name: "Expired", state: "Expired", want: resource.ColorBroken},
		{name: "Partial", state: "Partial", want: resource.ColorBroken},
		{name: "Deleted", state: "Deleted", want: resource.ColorDim},
		{name: "empty", state: "", want: resource.ColorHealthy},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := td.Color(resource.Resource{Fields: map[string]string{"state": tc.state}})
			if got != tc.want {
				t.Errorf("Color(state=%q) = %v, want %v", tc.state, got, tc.want)
			}
		})
	}
}
