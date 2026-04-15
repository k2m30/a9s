package unit

import (
	"testing"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func TestSgColor(t *testing.T) {
	td := resource.FindResourceType("sg")
	if td == nil {
		t.Fatal("sg not registered")
	}

	cases := []struct {
		name               string
		dangerousOpenCount string
		wideOpen           string
		want               resource.Color
	}{
		{name: "safe", dangerousOpenCount: "0", wideOpen: "false", want: resource.ColorHealthy},
		{name: "ssh_open", dangerousOpenCount: "1", wideOpen: "false", want: resource.ColorBroken},
		{name: "db_open", dangerousOpenCount: "2", wideOpen: "false", want: resource.ColorBroken},
		{name: "all_protocols_open", dangerousOpenCount: "0", wideOpen: "true", want: resource.ColorBroken},
		{name: "both", dangerousOpenCount: "3", wideOpen: "true", want: resource.ColorBroken},
		{name: "empty_fields", dangerousOpenCount: "", wideOpen: "", want: resource.ColorHealthy},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fields := map[string]string{}
			if tc.dangerousOpenCount != "" || tc.wideOpen != "" {
				fields["dangerous_open_count"] = tc.dangerousOpenCount
				fields["wide_open"] = tc.wideOpen
			}
			r := resource.Resource{Fields: fields}
			got := td.Color(r)
			if got != tc.want {
				t.Errorf("Color(dangerous_open_count=%q, wide_open=%q) = %v, want %v",
					tc.dangerousOpenCount, tc.wideOpen, got, tc.want)
			}
		})
	}
}
