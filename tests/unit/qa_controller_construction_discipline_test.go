// qa_controller_construction_discipline_test.go — one constructor, and this
// gate says so.
//
// A test that builds a real *app.Controller (directly, or transitively via
// tui.New — internal/tui's New wires an app.Controller into the returned
// Model) and then drives a ResourcesLoaded/EnrichmentChecked/
// AvailabilityChecked event through it can queue an async availability-cache
// save. If that goroutine outlives the test's t.TempDir() cleanup, the writer
// can still be calling cache.Store.SaveType under a directory os.RemoveAll is
// concurrently tearing down — and because the cache root is read live at write
// time, the leaked write can land inside whatever OTHER test's temp directory
// happens to be current when the scheduler runs it. That is the flake this
// gate exists to make unconstructible.
//
// The rule is structural and has no exceptions: every construction in this
// directory goes through newBlessedController / newBlessedModel
// (blessed_construction_test.go), which pair the close with t.Cleanup at the
// one point where the ordering is knowable. An allowlist would put the leak
// back for every entry on it, so there is none: a direct call fails, and the
// fix is the helper rather than a new line in a census.
package unit_test

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
)

// ccdConstructorFiles are the two files that define the blessed constructors;
// the direct call inside each is the definition the rule points at, not a
// violation of it.
var ccdConstructorFiles = map[string]bool{
	"blessed_construction_test.go":     true,
	"blessed_construction_ext_test.go": true,
}

// ccdSite is one direct tui.New/app.New call site the scanner found.
type ccdSite struct {
	file     string
	line     int
	funcName string
	pkg      string
}

// ccdIsNewCall reports whether call is a direct <pkg>.New*(...) call where
// pkg is "tui" or "app", returning the package identifier on match.
func ccdIsNewCall(call *ast.CallExpr) (pkg string, matched bool) {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return "", false
	}
	id, ok := sel.X.(*ast.Ident)
	if !ok {
		return "", false
	}
	if id.Name != "tui" && id.Name != "app" {
		return "", false
	}
	if !strings.HasPrefix(sel.Sel.Name, "New") {
		return "", false
	}
	return id.Name, true
}

// ccdEnclosingFuncIntervals returns, for every top-level FuncDecl in file
// with a body, its name and the [start,end) token.Pos span of that body.
// Nested ast.FuncLit closures share their enclosing FuncDecl's span (Go has
// no nested named funcs), so a position lookup against these intervals
// alone correctly attributes calls inside t.Run(func(t *testing.T){...})
// closures to the containing top-level function.
func ccdEnclosingFuncIntervals(file *ast.File) []struct {
	name       string
	start, end token.Pos
} {
	var intervals []struct {
		name       string
		start, end token.Pos
	}
	for _, decl := range file.Decls {
		fd, ok := decl.(*ast.FuncDecl)
		if !ok || fd.Body == nil {
			continue
		}
		intervals = append(intervals, struct {
			name       string
			start, end token.Pos
		}{fd.Name.Name, fd.Body.Pos(), fd.Body.End()})
	}
	return intervals
}

// ccdScanFile parses path and returns every direct tui.New/app.New call site
// in it.
func ccdScanFile(fset *token.FileSet, path string) ([]ccdSite, error) {
	src, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		return nil, err
	}
	intervals := ccdEnclosingFuncIntervals(src)
	enclosingFunc := func(pos token.Pos) string {
		for _, iv := range intervals {
			if iv.start <= pos && pos < iv.end {
				return iv.name
			}
		}
		return ""
	}

	var sites []ccdSite
	ast.Inspect(src, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		pkg, matched := ccdIsNewCall(call)
		if !matched {
			return true
		}
		pos := fset.Position(call.Pos())
		sites = append(sites, ccdSite{
			file:     filepath.Base(path),
			line:     pos.Line,
			funcName: enclosingFunc(call.Pos()),
			pkg:      pkg,
		})
		return true
	})
	return sites, nil
}

// TestControllerConstructionDisciplineGate fails on every direct
// tui.New(...)/app.New(...) call site under tests/unit outside the two files
// that define the blessed constructors.
func TestControllerConstructionDisciplineGate(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller(0) failed — cannot locate test file")
	}
	unitDir := filepath.Dir(thisFile)

	pattern := filepath.Join(unitDir, "*.go")
	files, err := filepath.Glob(pattern)
	if err != nil {
		t.Fatalf("filepath.Glob(%q): %v", pattern, err)
	}
	if len(files) == 0 {
		t.Fatalf("no .go files found under %s — check path", unitDir)
	}

	fset := token.NewFileSet()
	var violations []string
	constructorSites := 0
	for _, path := range files {
		sites, scanErr := ccdScanFile(fset, path)
		if scanErr != nil {
			t.Errorf("parse error in %s: %v", path, scanErr)
			continue
		}
		for _, site := range sites {
			if ccdConstructorFiles[site.file] {
				constructorSites++
				continue
			}
			label := site.funcName
			if label == "" {
				label = "<package-level>"
			}
			violations = append(violations, fmt.Sprintf("%s:%d %s (in %s)", site.file, site.line, site.pkg+".New", label))
		}
	}

	// The two definitions are the scanner's own canary: if the constructors
	// stop calling through, this gate is passing because it is looking at the
	// wrong tree.
	if constructorSites == 0 {
		t.Fatal("the blessed constructor files contain no tui.New/app.New call — the scanner is not reading the tree it thinks it is")
	}

	if len(violations) > 0 {
		sort.Strings(violations)
		t.Errorf("%d test(s) construct a controller or a root model directly instead of through "+
			"newBlessedController/newBlessedModel, so nothing pairs the close with t.Cleanup and the "+
			"cache writer can outlive the temp directory it is writing into:\n  %s",
			len(violations), strings.Join(violations, "\n  "))
	}
}
