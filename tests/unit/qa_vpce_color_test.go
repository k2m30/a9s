package unit

import (
	"testing"

	"github.com/k2m30/a9s/v3/core/resource"
)

func TestVpceColor(t *testing.T) {
	td := resource.FindResourceType("vpce")
	if td == nil {
		t.Fatal("vpce not registered")
	}

	// DescribeVpcEndpoints sends the state lower-camel; the SDK constants'
	// capitalised spelling never arrives on the wire.
	cases := []struct {
		name  string
		state string
		want  resource.Color
	}{
		{name: "available", state: "available", want: resource.ColorHealthy},
		{name: "pendingAcceptance", state: "pendingAcceptance", want: resource.ColorWarning},
		{name: "pending", state: "pending", want: resource.ColorWarning},
		{name: "deleting", state: "deleting", want: resource.ColorWarning},
		{name: "failed", state: "failed", want: resource.ColorBroken},
		{name: "rejected", state: "rejected", want: resource.ColorBroken},
		{name: "expired", state: "expired", want: resource.ColorBroken},
		{name: "partial", state: "partial", want: resource.ColorBroken},
		{name: "deleted", state: "deleted", want: resource.ColorDim},
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
