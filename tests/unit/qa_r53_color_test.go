package unit

// qa_r53_color_test.go — a zone's record count does not colour its row.
//
// A record_count branch in r53Color would be a second read of a fact the row
// already carries — r53CodeUnusedZone fires on the same condition over the
// same data — so a zone could be coloured for a reason no finding named and
// the detail view never showed. Colour derives from findings only, and a
// resource carrying none is Healthy whatever its fields say. The counts are
// the enumeration of what must not colour a row on its own, so a raw-field
// branch is what this catches.

import (
	"testing"

	"github.com/k2m30/a9s/v3/core/resource"
)

func TestR53Color(t *testing.T) {
	td := resource.FindResourceType("r53")
	if td == nil {
		t.Fatal("r53 resource type not registered")
	}

	cases := []struct {
		name        string
		recordCount string
		want        resource.Color
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
			// The fields reach the type\'s own predicate, which produces the
			// finding, so the colour the table names is one the row carries for a
			// reason the detail view shows.
			got := td.Color(resource.Resource{Fields: fields})
			if got != tc.want {
				t.Errorf("Color(record_count=%q) = %v, want %v", tc.recordCount, got, tc.want)
			}
		})
	}
}
