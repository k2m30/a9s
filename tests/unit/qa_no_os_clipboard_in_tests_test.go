package unit

import (
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

const osClipboardPkg = "github.com/atotto/clipboard"

// TestNoTestReachesTheOSClipboard fails when a test file imports the OS
// clipboard package.
//
// The pasteboard is one mutable object shared by every process on the
// machine: a test that writes it destroys what the developer had copied, a
// test that reads it back asserts on whoever wrote last, and two worktrees
// running the suite in parallel silently corrupt each other's assertions.
// Production reaches the pasteboard through two seams — clipboardWrite in
// internal/tui and clipboardRead in internal/tui/views — and a test that
// wants to know what was copied replaces the seam and reads the captured
// value.
//
// Scope is every _test.go in the tree plus every file under tests/, since a
// helper there is compiled into a test binary whether or not its name ends in
// _test.go.
func TestNoTestReachesTheOSClipboard(t *testing.T) {
	root := filepath.Join("..", "..")
	fset := token.NewFileSet()

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			if d.Name() == ".git" || d.Name() == "vendor" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		inTests := strings.Contains(filepath.ToSlash(path), "/tests/")
		if !inTests && !strings.HasSuffix(path, "_test.go") {
			return nil
		}

		f, parseErr := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
		if parseErr != nil {
			t.Errorf("%s: %v", path, parseErr)
			return nil
		}
		for _, imp := range f.Imports {
			p, unquoteErr := strconv.Unquote(imp.Path.Value)
			if unquoteErr != nil {
				continue
			}
			if p == osClipboardPkg {
				t.Errorf("%s:%d: imports %s — capture the copy at the seam instead (tui.SetClipboardWriteForTest / views.SetClipboardReadForTest)",
					path, fset.Position(imp.Pos()).Line, osClipboardPkg)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking %s: %v", root, err)
	}
}
