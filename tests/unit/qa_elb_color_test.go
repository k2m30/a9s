package unit

// qa_elb_color_test.go — Regression tests for Load Balancer Color mapping.
//
// ELB Color reads the "state" field. Tests pin each branch so regressions are caught.

import (
	"testing"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func TestElbColor(t *testing.T) {
	td := resource.FindResourceType("elb")
	if td == nil {
		t.Fatal("elb not registered")
	}

	cases := []struct {
		name  string
		state string
		want  resource.Color
	}{
		{name: "active", state: "active", want: resource.ColorHealthy},
		{name: "provisioning", state: "provisioning", want: resource.ColorWarning},
		{name: "active_impaired", state: "active_impaired", want: resource.ColorWarning},
		{name: "failed", state: "failed", want: resource.ColorBroken},
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
