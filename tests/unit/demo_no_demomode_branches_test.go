package unit

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestDemo_NoDemoModeBranches walks every non-test .go file under
// internal/tui/, internal/aws/, and internal/resource/ and fails if any file
// contains the substring "demoMode" or "DemoMode".
//
// This is a TDD guardrail for feature 014-demo-transport-mock: once the coder
// deletes every demoMode branch (T034–T037*), this test must pass. Until then
// it must fail (current code has many references). Do NOT delete the
// references — that is the coder's job.
//
// go test sets the working directory to tests/unit/, so the paths below are
// relative to that directory.
func TestDemo_NoDemoModeBranches(t *testing.T) {
	root := filepath.Join("..", "..")
	searchDirs := []string{
		filepath.Join(root, "internal", "tui"),
		filepath.Join(root, "internal", "aws"),
		filepath.Join(root, "internal", "resource"),
	}

	forbiddenSubstrings := []string{"demoMode", "DemoMode"}

	for _, dir := range searchDirs {
		err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}
			if !strings.HasSuffix(path, ".go") {
				return nil
			}
			// Skip test files — they are allowed to reference demoMode in helper
			// harnesses or setup code during the migration period.
			if strings.HasSuffix(path, "_test.go") {
				return nil
			}

			contents, readErr := os.ReadFile(path)
			if readErr != nil {
				t.Errorf("could not read %s: %v", path, readErr)
				return nil
			}

			text := string(contents)
			for _, substr := range forbiddenSubstrings {
				if strings.Contains(text, substr) {
					// Find line numbers for each occurrence for actionable output.
					lines := strings.Split(text, "\n")
					for lineIdx, line := range lines {
						if strings.Contains(line, substr) {
							t.Errorf("%s:%d: forbidden substring %q found: %s",
								path, lineIdx+1, substr, strings.TrimSpace(line))
						}
					}
				}
			}
			return nil
		})

		if err != nil {
			t.Fatalf("WalkDir(%s): %v", dir, err)
		}
	}
}
