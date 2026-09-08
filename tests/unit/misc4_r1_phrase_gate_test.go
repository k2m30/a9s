// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package unit_test

// misc4_r1_phrase_gate_test.go — misc4 row 1's gate and demo witness.
//
// fillSlot agrees a noun's number against the emitted value, but only inside a
// "<…>" slot. A plural marker written anywhere else in a declared wording is
// never resolved and reaches the operator's screen as "port(s)".

import (
	"regexp"
	"strings"
	"testing"

	"github.com/k2m30/a9s/v3/core/catalog"
	"github.com/k2m30/a9s/v3/core/demo"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

const misc4GateCodeExposedAll = domain.FindingCode("ec2.internet-exposed-all")

// TestRegisteredPhrasesCarryNoHedgeOutsideASlot is the gate: every declared
// wording, parents and children, may spell the plural marker only inside a
// slot, where fillSlot resolves it against the emitted value.
func TestRegisteredPhrasesCarryNoHedgeOutsideASlot(t *testing.T) {
	defs := 0
	for _, td := range append(catalog.All(), catalog.AllChildren()...) {
		for _, f := range td.Findings {
			defs++
			if strings.Contains(misc4OutsideSlots(f.Phrase), "(s)") {
				t.Errorf("%s: phrase %q spells the plural marker outside a slot; "+
					"move the noun into a \"<…>\" slot so the value decides its number",
					f.Code, f.Phrase)
			}
		}
	}
	if defs < 300 {
		t.Fatalf("only %d declared findings; the gate is not seeing the catalog", defs)
	}
}

// misc4SlotRE matches one "<…>" slot. Slots do not nest.
var misc4SlotRE = regexp.MustCompile(`<[^>]*>`)

// misc4OutsideSlots returns phrase with every slot removed.
func misc4OutsideSlots(phrase string) string {
	return misc4SlotRE.ReplaceAllString(phrase, "")
}

// TestEC2ExposedWideOpenHasADemoWitness pins that the wide-open code renders
// on the demo list, so the signal is visible without AWS credentials.
func TestEC2ExposedWideOpenHasADemoWitness(t *testing.T) {
	byType, cache := buildVisibilityTypeCache(t)
	td := resource.FindResourceType("ec2")
	if td == nil {
		t.Fatal("ec2 type not registered")
	}
	for _, r := range mergeWave2Findings(t, *td, byType["ec2"], cache, demo.NewServiceClients()) {
		for _, f := range r.Findings {
			if f.Code == misc4GateCodeExposedAll {
				return
			}
		}
	}
	t.Fatalf("no demo row carries %s", misc4GateCodeExposedAll)
}
