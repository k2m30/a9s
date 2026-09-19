package unit_test

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestW29_CloudFormationPhraseParserIsGone: colouring a stack by rendering
// its status into a phrase and reading the phrase back lets the classifier
// disagree with the finding that produced the words.
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
