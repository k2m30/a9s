package unit

// golden_research_related_test.go — Golden test: all P0 relationships from
// research docs must have a matching RegisterRelated entry.
//
// Reads every docs/design/related-resources/{shortname}.md file, extracts
// table rows marked "| P0 |", parses the target shortname from the first
// column (parenthesized, e.g. "(ec2)"), then verifies that resource.GetRelated
// for the source type contains a RelatedDef with that TargetType.
//
// This test DOES NOT check:
//   - P1 or P2 relationships (only P0 is mandatory)
//   - Whether checkers are nil (covered by TestGolden_LiveCheckerCompleteness)
//   - Targets that are not top-level resource types in a9s

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	_ "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// reP0Row matches any markdown table row that ends with "| P0 |" (with optional
// trailing whitespace).  The entire line is captured so we can extract the
// first column separately.
var reP0Row = regexp.MustCompile(`(?i)\|\s*P0\s*\|\s*$`)

// reParenShortname captures a lowercase alphanumeric+hyphen token inside
// parentheses, e.g. "(ec2)", "(sg)", "(eb-rule)".
var reParenShortname = regexp.MustCompile(`\(([a-z0-9][a-z0-9\-]*)\)`)

// resolveShortname resolves a raw token (from a filename base or a parenthesised
// reference in a table row) to the canonical resource shortname registered in
// resource.FindResourceType.  It tries:
//   1. exact match
//   2. token + "s"  (handles ct-event → ct-events and similar doc typos)
//
// Returns the resolved name, or the original token when no match is found.
func resolveShortname(token string) string {
	if resource.FindResourceType(token) != nil {
		return token
	}
	withS := token + "s"
	if resource.FindResourceType(withS) != nil {
		return withS
	}
	return token
}

// filenameToShortname converts a research-doc filename (without ".md") to
// the canonical resource shortname.
func filenameToShortname(base string) string {
	return resolveShortname(base)
}

func TestGolden_ResearchP0RelationshipsRegistered(t *testing.T) {
	docsDir := "../../docs/design/related-resources"

	entries, err := os.ReadDir(docsDir)
	if err != nil {
		t.Fatalf("cannot read research docs directory %q: %v", docsDir, err)
	}

	failures := 0

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".md") {
			continue
		}

		base := strings.TrimSuffix(name, ".md")
		sourceShortname := filenameToShortname(base)

		// Skip files that don't correspond to a known top-level resource type.
		if resource.FindResourceType(sourceShortname) == nil {
			t.Logf("SKIP %s: no resource type registered for shortname %q", name, sourceShortname)
			continue
		}

		filePath := filepath.Join(docsDir, name)
		content, err := os.ReadFile(filePath) //nolint:gosec // path is derived from ReadDir
		if err != nil {
			t.Errorf("cannot read %s: %v", filePath, err)
			continue
		}

		// Collect all P0 target shortnames from this file.
		p0Targets := extractP0Targets(string(content))
		if len(p0Targets) == 0 {
			// No P0 relationships defined — nothing to check.
			continue
		}

		// Build the set of registered target types for this source.
		defs := resource.GetRelated(sourceShortname)
		registered := make(map[string]bool, len(defs))
		for _, d := range defs {
			registered[d.TargetType] = true
		}

		for _, target := range p0Targets {
			// Only check targets that are themselves known top-level resource types.
			if resource.FindResourceType(target) == nil {
				t.Logf("SKIP %s→%s: target %q is not a registered resource type (child-only or not in a9s)",
					sourceShortname, target, target)
				continue
			}

			if !registered[target] {
				t.Errorf("%s: P0 relationship %q from research doc not registered in RegisterRelated",
					sourceShortname, target)
				failures++
			}
		}
	}

	if failures > 0 {
		t.Logf("%d P0 relationship gap(s) found — add RegisterRelated entries for each", failures)
	}
}

// extractP0Targets reads all lines of a research doc and returns the unique set
// of target shortnames (from parenthesised tokens) on rows ending in "| P0 |".
func extractP0Targets(content string) []string {
	seen := map[string]bool{}
	var targets []string

	for _, line := range strings.Split(content, "\n") {
		if !reP0Row.MatchString(line) {
			continue
		}
		// The first column of the table row contains the display name and
		// shortname, e.g. "| Target Groups (tg) | ...".  We scan the whole line
		// for ALL parenthesised tokens but practically only the first column
		// contains a shortname (later columns are How-to-Find / Scenario text
		// that doesn't follow the "(shortname)" convention).
		matches := reParenShortname.FindAllStringSubmatch(line, -1)
		for _, m := range matches {
			tok := resolveShortname(m[1])
			if !seen[tok] {
				seen[tok] = true
				targets = append(targets, tok)
			}
		}
	}
	return targets
}
