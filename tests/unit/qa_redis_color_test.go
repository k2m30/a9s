package unit

import (
	"testing"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func TestRedisColor(t *testing.T) {
	td := resource.FindResourceType("redis")
	if td == nil {
		t.Fatal("redis not registered")
	}

	cases := []struct {
		name   string
		status string
		want   resource.Color
	}{
		{name: "available", status: "available", want: resource.ColorHealthy},
		{name: "creating", status: "creating", want: resource.ColorWarning},
		{name: "modifying", status: "modifying", want: resource.ColorWarning},
		{name: "snapshotting", status: "snapshotting", want: resource.ColorWarning},
		{name: "deleting", status: "deleting", want: resource.ColorWarning},
		{name: "rebooting", status: "rebooting cluster nodes", want: resource.ColorWarning},
		{name: "restore_failed", status: "restore-failed", want: resource.ColorBroken},
		{name: "incompatible_network", status: "incompatible-network", want: resource.ColorBroken},
		{name: "deleted", status: "deleted", want: resource.ColorDim},
		{name: "empty", status: "", want: resource.ColorHealthy},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := td.Color(resource.Resource{Fields: map[string]string{"status": tc.status}})
			if got != tc.want {
				t.Errorf("Color(status=%q) = %v, want %v", tc.status, got, tc.want)
			}
		})
	}
}
