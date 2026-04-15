package unit

import (
	"testing"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func TestEbRuleColor(t *testing.T) {
	td := resource.FindResourceType("eb-rule")
	if td == nil {
		t.Fatal("eb-rule not registered")
	}

	cases := []struct {
		name  string
		state string
		want  resource.Color
	}{
		{
			name:  "enabled_upper",
			state: "ENABLED",
			want:  resource.ColorHealthy,
		},
		{
			name:  "enabled_lower",
			state: "enabled",
			want:  resource.ColorHealthy,
		},
		{
			name:  "disabled_upper",
			state: "DISABLED",
			want:  resource.ColorDim,
		},
		{
			name:  "disabled_lower",
			state: "disabled",
			want:  resource.ColorDim,
		},
		{
			name:  "empty",
			state: "",
			want:  resource.ColorHealthy,
		},
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
