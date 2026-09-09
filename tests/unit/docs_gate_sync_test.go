package unit

// docs_gate_sync_test.go pins the Makefile's `ready-to-push` prerequisite
// list against the Stage 6 enumeration in docs/development-process.md, so
// the two cannot drift apart when either is edited on its own.
// docs/development-process.md's Stage 6 section names this file directly
// and states that the enumeration is pinned by it.

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

var reReadyToPushTarget = regexp.MustCompile(`(?m)^ready-to-push:[ \t]*(.*)$`)

// parseMakefileReadyToPush reads the single `ready-to-push:` line from the
// Makefile and returns its prerequisite target names in declaration order.
func parseMakefileReadyToPush(t *testing.T, makefilePath string) []string {
	t.Helper()
	data, err := os.ReadFile(makefilePath)
	if err != nil {
		t.Fatalf("cannot read %s: %v", makefilePath, err)
	}
	m := reReadyToPushTarget.FindSubmatch(data)
	if m == nil {
		t.Fatalf("%s: no `ready-to-push:` target line found", makefilePath)
	}
	targets := strings.Fields(string(m[1]))
	if len(targets) == 0 {
		t.Fatalf("%s: `ready-to-push:` line has no prerequisite targets", makefilePath)
	}
	return targets
}

var (
	reStage6Heading      = regexp.MustCompile(`(?m)^### Stage 6 `)
	reNextHeading        = regexp.MustCompile(`(?m)^#{2,4} `)
	reNumberedMakeTarget = regexp.MustCompile("(?m)^[0-9]+\\. `make ([a-zA-Z0-9_-]+)`")
)

// parseDocStage6Targets reads the numbered `make <target>` list under the
// "### Stage 6" heading in docs/development-process.md and returns the
// target names in the order they are enumerated.
func parseDocStage6Targets(t *testing.T, docPath string) []string {
	t.Helper()
	data, err := os.ReadFile(docPath)
	if err != nil {
		t.Fatalf("cannot read %s: %v", docPath, err)
	}
	content := string(data)

	headingLoc := reStage6Heading.FindStringIndex(content)
	if headingLoc == nil {
		t.Fatalf(`%s: no "### Stage 6" heading found`, docPath)
	}
	rest := content[headingLoc[1]:]
	section := rest
	if nextLoc := reNextHeading.FindStringIndex(rest); nextLoc != nil {
		section = rest[:nextLoc[0]]
	}

	matches := reNumberedMakeTarget.FindAllStringSubmatch(section, -1)
	if len(matches) == 0 {
		t.Fatalf("%s: Stage 6 section has no numbered `make <target>` lines", docPath)
	}
	targets := make([]string, len(matches))
	for i, m := range matches {
		targets[i] = m[1]
	}
	return targets
}

func stringSet(items []string) map[string]bool {
	set := make(map[string]bool, len(items))
	for _, item := range items {
		set[item] = true
	}
	return set
}

func sameOrder(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// TestDocsGateSync_Stage6MatchesMakefileReadyToPush verifies that the
// Makefile's `ready-to-push:` prerequisite list and the numbered Stage 6
// enumeration in docs/development-process.md name the exact same targets
// in the exact same order, in both directions.
func TestDocsGateSync_Stage6MatchesMakefileReadyToPush(t *testing.T) {
	makeTargets := parseMakefileReadyToPush(t, "../../Makefile")
	docTargets := parseDocStage6Targets(t, "../../docs/development-process.md")

	makeSet := stringSet(makeTargets)
	docSet := stringSet(docTargets)

	var missingFromDoc []string
	for _, target := range makeTargets {
		if !docSet[target] {
			missingFromDoc = append(missingFromDoc, target)
		}
	}
	var extraInDoc []string
	for _, target := range docTargets {
		if !makeSet[target] {
			extraInDoc = append(extraInDoc, target)
		}
	}

	if len(missingFromDoc) > 0 {
		t.Errorf("Makefile's ready-to-push runs %v, which docs/development-process.md's Stage 6 enumeration does not list: "+
			"the Makefile gained a gate target — document it", missingFromDoc)
	}
	if len(extraInDoc) > 0 {
		t.Errorf("docs/development-process.md's Stage 6 enumeration lists %v, which the Makefile's ready-to-push prerequisites do not contain: "+
			"update docs/development-process.md §Stage 6", extraInDoc)
	}

	if len(missingFromDoc) == 0 && len(extraInDoc) == 0 && !sameOrder(makeTargets, docTargets) {
		t.Errorf("docs/development-process.md's Stage 6 enumeration lists the same %d targets as the Makefile's ready-to-push "+
			"but in a different order — Makefile order: %v, doc order: %v: "+
			"update docs/development-process.md §Stage 6 to match the Makefile's declaration order",
			len(makeTargets), makeTargets, docTargets)
	}
}
