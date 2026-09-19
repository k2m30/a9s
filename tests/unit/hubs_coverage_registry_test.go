package unit_test

// hubs_coverage_registry_test.go pins that the demo-state burn-down allowlist
// lives with the fixtures it describes. One shared map literal would make
// every type's burn-down edit the same file, so two tasks closing gaps on two
// different types would conflict.

import (
	"os"
	"sort"
	"strings"
	"testing"

	_ "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/demo/fixtures"
	"github.com/k2m30/a9s/v3/core/resource"
)

// TestCoverageGaps_EveryKeyNamesARealTypeAndGap pins the key shape
// "<shortName>:<bucket|finding code>". A key that matches no registered type,
// bucket or FindingDef is never looked up: it can neither hold a gap open nor
// ever be reported as ready for burn-down, so a typo would quietly disarm the
// ratchet for that entry forever.
func TestCoverageGaps_EveryKeyNamesARealTypeAndGap(t *testing.T) {
	if len(fixtures.CoverageGaps()) == 0 {
		t.Fatal("no type registered a coverage gap — the ratchet's allowlist is empty, " +
			"so every gap qa_demo_state_coverage_test.go still finds is an unallowlisted failure")
	}

	buckets := map[string]bool{"healthy": true, "warning": true, "broken": true, "dim": true}

	codesByType := make(map[string]map[string]bool)
	for _, td := range resource.AllResourceTypes() {
		codes := make(map[string]bool)
		for _, fd := range td.Findings {
			codes[string(fd.Code)] = true
		}
		codesByType[td.ShortName] = codes
	}

	var bad []string
	for key := range fixtures.CoverageGaps() {
		shortName, gap, ok := strings.Cut(key, ":")
		codes, known := codesByType[shortName]
		switch {
		case !ok:
			bad = append(bad, key+" — not in \"<shortName>:<gap>\" shape")
		case !known:
			bad = append(bad, key+" — no registered resource type "+shortName)
		case !buckets[gap] && !codes[gap]:
			bad = append(bad, key+" — neither a domain.Color bucket nor a FindingDef code of "+shortName)
		}
	}
	sort.Strings(bad)
	for _, b := range bad {
		t.Errorf("unusable coverage-gap key: %s", b)
	}
}

// A map literal beside the registry would be a second list that can disagree
// with it, and a shared hub file.
func TestCoverageGaps_AllowlistLiteralIsGone(t *testing.T) {
	raw, err := os.ReadFile("qa_demo_state_coverage_test.go")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "knownStateCoverageGaps = map[string]bool{") {
		t.Error("qa_demo_state_coverage_test.go declares the allowlist as a map literal — " +
			"the gaps belong on each type's fixtures.Pin")
	}
}
