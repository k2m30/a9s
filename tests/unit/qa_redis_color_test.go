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

	// Post-migration (2026-04-23): Fields["status"] carries §4 PHRASES, not
	// bare keywords. Color function matches on phrase, with a prefix branch
	// for shard-level phrases on cluster-mode-enabled multi-shard RGs.
	cases := []struct {
		name   string
		status string
		want   resource.Color
	}{
		// Healthy silence.
		{name: "empty_healthy", status: "", want: resource.ColorHealthy},
		// §4 Warning phrases.
		{name: "creating", status: "creating — new group", want: resource.ColorWarning},
		{name: "modifying", status: "modifying — config change", want: resource.ColorWarning},
		{name: "snapshotting", status: "snapshotting — backup running", want: resource.ColorWarning},
		{name: "deleting", status: "deleting — teardown", want: resource.ColorWarning},
		{name: "multiaz_no_failover", status: "multi-AZ without auto-failover", want: resource.ColorWarning},
		// §4 Broken phrase.
		{name: "create_failed", status: "create failed — see events", want: resource.ColorBroken},
		// Rule-7 (+N) suffix must not defeat the phrase match.
		{name: "modifying_plus_one", status: "modifying — config change (+1)", want: resource.ColorWarning},
		// Shard-level phrases (multi-shard §4 — prefix branch).
		{name: "shard_0001_modifying", status: "shard 0001: modifying", want: resource.ColorWarning},
		{name: "shard_0002_snapshotting_plus_one", status: "shard 0002: snapshotting (+1)", want: resource.ColorWarning},
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
