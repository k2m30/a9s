package unit

// qa_r53_color_test.go — a zone's record count no longer colours its row.
//
// INVERTED for batch w6a, the same way qa_acm_color_test.go, qa_cf_color_test.go
// and qa_logs_color_test.go were. This table used to assert the record_count
// branch in r53Color: <= 2 records rendered ColorWarning. That branch was a
// second read of a fact the row already carried — r53CodeUnusedZone fires on
// the same condition over the same data — so a zone could be coloured for a
// reason no finding named and the detail view never showed. Colour now derives
// from findings only, and a resource carrying none is Healthy whatever its
// fields say.
//
// The counts are kept as the enumeration of what must no longer colour a row on
// its own. Do not "restore" the old wants — a raw-field branch coming back is
// exactly what this now catches.

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
			// Each case asserts its own want again. w6a made the driver demand
			// Healthy for all of them, on the reading that a colour with no
			// finding behind it is a colour nobody can explain. w29 converted
			// this classifier: the fields reach the type's own predicate, which
			// produces the finding, so the colour the table always named is the
			// one the row now carries for a reason the detail view shows.
			got := td.Color(resource.Resource{Fields: fields})
			if got != tc.want {
				t.Errorf("Color(record_count=%q) = %v, want %v", tc.recordCount, got, tc.want)
			}
		})
	}
}
