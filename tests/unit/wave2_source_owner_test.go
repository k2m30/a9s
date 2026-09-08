// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package unit_test

// wave2_source_owner_test.go — a Wave-2 finding's provenance is the registry's.
//
// Finding.Source's only readers ask one question: did a Wave-2 enricher emit
// this? They test the "wave2:" prefix; nothing reads the short name after it.
// That name was passed in by every enricher at every call site, so a typo
// stamped a provenance no reader could catch, and two call sites for one type
// could disagree. The runtime already knows which type's enricher it is
// merging — it has the registry entry in hand — so the stamp belongs there and
// only there.

import (
	"testing"

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime"
)

func TestWave2SourceComesFromTheRegistryNotTheEnricher(t *testing.T) {
	cases := []struct {
		name    string
		emitted string
	}{
		{"enricher set nothing", ""},
		{"enricher named the wave only", "wave2"},
		{"enricher named its own type", "wave2:ec2"},
		// The case no reader could ever have caught: a short name that is not
		// the type whose enricher ran.
		{"enricher named the wrong type", "wave2:not-ec2"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := &domain.Resource{ID: "i-0abc", Type: "ec2"}
			findings := map[string][]domain.Finding{
				"i-0abc": {{Code: "ec2.probe", Phrase: "probe", Severity: domain.SevWarn, Source: tc.emitted}},
			}

			runtime.ApplyWave2ToRow(r, resource.ResourceTypeDef{ShortName: "ec2"}, findings, nil)

			if len(r.Findings) != 1 {
				t.Fatalf("want exactly 1 finding, got %+v", r.Findings)
			}
			if got, want := r.Findings[0].Source, "wave2:ec2"; got != want {
				t.Errorf("Source = %q, want %q — the merge point holds the registry entry, so it "+
					"owns the provenance; an enricher's own spelling is not consulted", got, want)
			}
			if !r.Findings[0].IsWave2Sourced() {
				t.Errorf("finding does not read as Wave-2 sourced: %+v", r.Findings[0])
			}
		})
	}
}
