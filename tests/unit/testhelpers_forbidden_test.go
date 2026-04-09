package unit

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// TestNoForbiddenTestHelpers walks every .go file under tests/unit/ and fails
// if any file (except itself) contains forbidden anti-patterns:
//
//  1. The literal "buildDemoResourceCache" — callers must use the shared
//     harness from testhelpers_demo_harness.go instead.
//
//  2. "Checker: nil" — a RelatedDef with a nil checker is a structural bug;
//     use the demo harness or a properly wired checker.
//
//  3. Lines matching `.Checker(<anything containing nil>)` — direct nil-client
//     checker invocations bypass the demo transport entirely.
//
// This test is expected to FAIL until T045–T049 rewrite the offenders.
// Report file paths and line numbers so engineers know exactly what to fix.
func TestNoForbiddenTestHelpers(t *testing.T) {
	t.Helper()

	selfName := "testhelpers_forbidden_test.go"

	// Compiled once; matches direct nil-client checker calls such as:
	//   SomeChecker(ctx, nil, res)
	//   resource.CheckVPC(ctx, nil, r)
	nilCheckerCallRE := regexp.MustCompile(`\.Checker\([^)]*nil[^)]*\)`)

	// Forbidden literal substrings (checked per-line for accurate line numbers).
	type literalCheck struct {
		substr string
		label  string
	}
	literals := []literalCheck{
		{"buildDemoResourceCache", "forbidden helper buildDemoResourceCache (use shared harness)"},
		{"Checker: nil", "RelatedDef with nil Checker"},
	}

	root := filepath.Join("..", "..")
	searchDir := filepath.Join(root, "tests", "unit")

	err := filepath.WalkDir(searchDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		// Skip this file — it necessarily contains the forbidden strings as
		// string literals inside the check definitions above.
		if filepath.Base(path) == selfName {
			return nil
		}

		contents, readErr := os.ReadFile(path)
		if readErr != nil {
			t.Errorf("could not read %s: %v", path, readErr)
			return nil
		}

		lines := strings.Split(string(contents), "\n")
		for lineIdx, line := range lines {
			lineNo := lineIdx + 1

			// Check literal substrings.
			for _, lc := range literals {
				if strings.Contains(line, lc.substr) {
					t.Errorf("%s:%d: %s\n\t%s", path, lineNo, lc.label, strings.TrimSpace(line))
				}
			}

			// Check nil-client checker call pattern.
			if nilCheckerCallRE.MatchString(line) {
				t.Errorf("%s:%d: direct nil-client Checker call (use demo harness)\n\t%s",
					path, lineNo, strings.TrimSpace(line))
			}
		}
		return nil
	})

	if err != nil {
		t.Fatalf("WalkDir(%s): %v", searchDir, err)
	}
}
