package unit

// w27_wave_registration_test.go — no docs/resources/<short>.md page claims a
// wave it does not register: the §S1 badge paragraph ("N uses the same
// aggregation as the menu badge (Wave 1 issue-colored rows + Wave 2
// `!`-severity findings)") is false for a type that registers no Wave 2
// enricher, such as opensearch (docs/resources/opensearch.md says outright
// "opensearch registers no Wave 2 enricher").

import (
	"os"
	"strings"
	"testing"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/resource"
)

// wave2BadgeClaim is the exact fragment the copied §S1 paragraph uses to
// credit Wave 2 findings toward the badge count.
const wave2BadgeClaim = "Wave 2 `!`-severity findings"

// TestDocPage_NoWaveItDoesNotRegister walks every registered type with a
// docs/resources page and fails when a type with no Wave 2 enricher still
// has a page claiming Wave 2 findings contribute to its badge count.
// Red today for opensearch.
func TestDocPage_NoWaveItDoesNotRegister(t *testing.T) {
	var falsePages []string
	for _, td := range resource.AllResourceTypes() {
		if _, hasWave2 := awsclient.Wave2EnricherFor(td.ShortName); hasWave2 {
			continue
		}
		path := "../../docs/resources/" + td.ShortName + ".md"
		b, err := os.ReadFile(path)
		if err != nil {
			continue // no page for this type; nothing to check
		}
		if strings.Contains(string(b), wave2BadgeClaim) {
			falsePages = append(falsePages, td.ShortName)
		}
	}
	if len(falsePages) > 0 {
		t.Errorf("%d page(s) claim Wave 2 findings contribute to the badge count for a type that registers no "+
			"Wave 2 enricher: %v", len(falsePages), falsePages)
	}
}

// TestDocPage_OpensearchRegistersNoWave2 pins the fact the false claim rests
// on: opensearch's own §3.2 already says it registers no Wave 2 enricher, and
// the catalog agrees. If this stops being true the test above needs a
// different witness, not a silently vanished one.
func TestDocPage_OpensearchRegistersNoWave2(t *testing.T) {
	if _, hasWave2 := awsclient.Wave2EnricherFor("opensearch"); hasWave2 {
		t.Fatal("opensearch now registers a Wave 2 enricher; TestDocPage_NoWaveItDoesNotRegister's opensearch " +
			"witness for row 6 is gone and needs replacing")
	}
}
