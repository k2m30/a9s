package unit_test

// w29_end_state_test.go — the three things that are only true once the task is
// finished. Each is red today and goes green without any edit here, so the
// conversion is what turns them, not a number someone moves.
//
// Row 5 has no pin of its own: `grep 'range .*Findings' core/aws/catalog_*.go`
// already returns nothing on this base, and a first-match loop reintroduced
// later returns a colour that is neither the findings guard's nor a shared
// fallback call, which is exactly what the burn-down gate below rejects.

import (
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
)

// TestW29_BurnDownIsEmpty is rows 1 and 4. The burn-down gate keeps its own
// ratchet honest round by round; this says the ratchet reaches zero. It needs
// no edit to go green — deleting the last entry does it — so the list can
// never be declared finished while it still holds a name.
func TestW29_BurnDownIsEmpty(t *testing.T) {
	if len(classifiersOffTheSharedFallback) == 0 {
		return
	}
	names := make([]string, 0, len(classifiersOffTheSharedFallback))
	for name := range classifiersOffTheSharedFallback {
		names = append(names, name)
	}
	sort.Strings(names)
	t.Errorf("%d classifier(s) still pick a colour of their own:\n  %s\n\n"+
		"Each takes the shared fallback over its type's own predicate; when the last one does, "+
		"delete classifiersOffTheSharedFallback and the two arms that read it.",
		len(names), strings.Join(names, "\n  "))
}

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
