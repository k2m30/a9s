package unit

// qa_r53_color_test.go — Behavioral tests for the Route 53 Hosted Zone Color function.
//
// The Color function for r53 is keyed on the "record_count" field, which is set
// from HostedZone.ResourceRecordSetCount by the fetcher.  A zone with <= 2 records
// is considered empty/stub and rendered as ColorWarning; anything >= 3 is ColorHealthy.
// A missing field defaults to ColorHealthy.

import (
	"testing"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func TestR53Color(t *testing.T) {
	td := resource.FindResourceType("r53")
	if td == nil {
		t.Fatal("r53 resource type not registered")
	}

	cases := []struct {
		name       string
		recordCount string
		want       resource.Color
	}{
		// A zone with exactly 2 records is a stub (SOA + NS only) — warn the operator.
		{name: "empty_zone", recordCount: "2", want: resource.ColorWarning},
		// A zone with 1 record (just SOA) is also essentially empty — warn.
		{name: "one_record", recordCount: "1", want: resource.ColorWarning},
		// A zone with 3 records has at least one real record in addition to SOA/NS.
		{name: "three_records", recordCount: "3", want: resource.ColorHealthy},
		// A zone with many records is healthy.
		{name: "many_records", recordCount: "50", want: resource.ColorHealthy},
		// No record_count field present at all — default to healthy.
		{name: "missing", recordCount: "", want: resource.ColorHealthy},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fields := map[string]string{}
			if tc.recordCount != "" {
				fields["record_count"] = tc.recordCount
			}
			got := td.Color(resource.Resource{Fields: fields})
			if got != tc.want {
				t.Errorf("Color(record_count=%q) = %v, want %v", tc.recordCount, got, tc.want)
			}
		})
	}
}
