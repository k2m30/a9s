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
//  3. Lines passing nil for a checker's clients or resource argument — such
//     a call bypasses the demo transport entirely. The last argument is the
//     target cache, and nil there is what a checker needing none is called
//     with.
//
// Rule 3 has an audited exception list, nilClientIsTheSubject: a test whose
// assertion is about what a checker answers with nothing to read hands it nil
// on purpose, and demo clients there would answer the question the test is
// asking.
func TestNoForbiddenTestHelpers(t *testing.T) {
	t.Helper()

	selfName := "testhelpers_forbidden_test.go"

	// Compiled once; matches direct nil-client checker calls such as:
	//   SomeChecker(ctx, nil, res)
	//   resource.CheckVPC(context.Background(), nil, r)
	//
	// The trailing comma is what makes the position the test: a nil the call
	// ends on is the target cache, which a checker needing none is called
	// with, and a nil with an argument after it is neither.
	nilCheckerCallRE := regexp.MustCompile(`\.Checker\(.*,\s*nil\s*,`)

	// Each of these asserts what a checker answers when it can read nothing:
	// Unknown rather than a guessed zero, a valid result shape, and no panic
	// on a row carrying no RawStruct. The nil is the subject of the
	// assertion, so a client that answers makes the assertion vacuous.
	nilClientIsTheSubject := map[string]bool{
		"aws_ddb_related_test.go":                  true,
		"ct_events_rightcol_dispatch_test.go":      true,
		"qa_related_garbage_ids_test.go":           true,
		"related_validate_test.go":                 true,
		"views_detail_rightcol_hide_zeros_test.go": true,
	}

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

			for _, lc := range literals {
				if strings.Contains(line, lc.substr) {
					t.Errorf("%s:%d: %s\n\t%s", path, lineNo, lc.label, strings.TrimSpace(line))
				}
			}

			if nilCheckerCallRE.MatchString(line) && !nilClientIsTheSubject[filepath.Base(path)] {
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
