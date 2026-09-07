package unit_test

// w29_end_state_test.go — what is only true once the task is finished.
//
// The burn-down pin that lived here is gone with the list it read: every
// classifier decides through the shared fallback, so
// TestClassifiersDecideThroughTheSharedFallback says that outright instead of
// counting down to it.
//
// Row 5 has no pin of its own: `grep 'range .*Findings' core/aws/catalog_*.go`
// already returns nothing on this base, and a first-match loop reintroduced
// later returns a colour that is neither the findings guard's nor a shared
// fallback call, which is exactly what the burn-down gate below rejects.

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestW29_CloudFormationPhraseParserIsGone is row 2. Colouring a stack meant
// rendering its status into a phrase and reading the phrase back, so the
// classifier could disagree with the finding that produced the words. The
// parser goes rather than moving.
func TestW29_CloudFormationPhraseParserIsGone(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	files, err := filepath.Glob(filepath.Join(filepath.Dir(thisFile), "..", "..", "core", "aws", "*.go"))
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	if len(files) == 0 {
		t.Fatal("no core/aws sources found; the pin would pass vacuously")
	}
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("read %s: %v", f, err)
		}
		if strings.Contains(string(b), "func cfnStackColor(") {
			t.Errorf("%s still defines cfnStackColor; the stack classifier takes findings like the rest, "+
				"so nothing parses the rendered phrase back into a colour", filepath.Base(f))
		}
	}
}
