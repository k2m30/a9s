package unit

import (
	"testing"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func TestSubnetColor(t *testing.T) {
	td := resource.FindResourceType("subnet")
	if td == nil {
		t.Fatal("subnet not registered")
	}

	cases := []struct {
		name  string
		state string
		want  resource.Color
	}{
		{name: "available", state: "available", want: resource.ColorHealthy},
		{name: "pending", state: "pending", want: resource.ColorWarning},
		{name: "unavailable", state: "unavailable", want: resource.ColorBroken},
		{name: "failed", state: "failed", want: resource.ColorBroken},
		{name: "failed_insufficient_capacity", state: "failed-insufficient-capacity", want: resource.ColorBroken},
		{name: "unknown", state: "", want: resource.ColorHealthy},
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
