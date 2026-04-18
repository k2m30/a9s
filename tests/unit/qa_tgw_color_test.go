package unit

import (
	"testing"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func TestTgwColor(t *testing.T) {
	td := resource.FindResourceType("tgw")
	if td == nil {
		t.Fatal("tgw not registered")
	}

	cases := []struct {
		name  string
		state string
		want  resource.Color
	}{
		{name: "available", state: "available", want: resource.ColorHealthy},
		{name: "pending", state: "pending", want: resource.ColorWarning},
		{name: "modifying", state: "modifying", want: resource.ColorWarning},
		{name: "deleting", state: "deleting", want: resource.ColorWarning},
		{name: "failed", state: "failed", want: resource.ColorBroken},
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
