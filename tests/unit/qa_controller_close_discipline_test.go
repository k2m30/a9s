// qa_controller_close_discipline_test.go — Close-discipline gate.
//
// Root cause: tui.New/app.New start a background availability-save goroutine
// on first cache write (queueAvailabilitySave -> runAvailabilitySaveLoop, see
// app_availsave_tempdir_cleanup_race_test.go's header for the full mechanism
// this mirrors). A test function that constructs a root model/controller via
// tui.New(...) or app.New(...) but never closes it (CloseController /
// Close) leaks that goroutine, which can still be mid-SaveType (MkdirAll +
// CreateTemp + Rename) when the SAME test's own t.TempDir() cleanup runs
// RemoveAll on that directory — "TempDir RemoveAll cleanup: ... The
// directory is not empty", observed on Windows CI.
//
// NARROWED SCOPE — the physics, stated explicitly: this gate only flags a
// function that BOTH constructs (tui.New/app.New) AND owns a temp-dir-backed
// config folder (t.TempDir() or a t.Setenv/os.Setenv of A9S_CONFIG_FOLDER)
// in that SAME body. A bare constructor call with neither cannot reproduce
// either observed CI failure:
//
//   - Cross-test contamination (a leaked writer landing in a LATER test's
//     directory because it re-read A9S_CONFIG_FOLDER live at first-use time)
//     is closed by the session cacheRoot pin (see
//     cache_root_pin_and_path_safety_test.go) — once a Session's cache root
//     is captured at construction, a leaked writer's SaveType targets
//     whatever directory was live WHEN ITS OWN SESSION WAS BUILT, never a
//     later, unrelated test's directory.
//   - That leaves only a SAME-body race: a leaked writer bound to a
//     directory this exact test function created (t.TempDir()) and will
//     remove (t.Cleanup(RemoveAll), registered by testing.T itself) when the
//     function returns. A construct call whose body never establishes such
//     a directory — no t.TempDir(), no A9S_CONFIG_FOLDER override — has its
//     writer (if any) bound to whatever directory was live for the whole
//     test binary's run (a package-level TestMain default, or the real
//     ~/.a9s/cache), which outlives the test and is never RemoveAll'd by it;
//     there is no directory for that leaked writer to race.
//
// This was verified empirically, not assumed: an earlier, unnarrowed version
// of this gate matched 247 functions across 90 files; cross-checking each
// for an in-body TempDir/A9S_CONFIG_FOLDER signal found only 8 with one —
// the narrowed rule below reproduces exactly that 8, and a hand-checked
// sample of the excluded 239 (e.g. TestModel_HasAppContext, which calls
// tui.New("", "") and touches no directory at all) confirmed each is
// structurally incapable of the race this gate exists to prevent.
//
// DESIGN CHOICE (mirrors qa_multifinding_no_legacy_gate_test.go): no
// allowlist. Every run does a fresh AST scan and unconditionally fails via
// t.Errorf, listing every violating function. This is a purge gate, not a
// slow ratchet — the narrowed violation set must be, and stay, zero; the fix
// is always "add t.Cleanup(...Close...) in the same function body", never a
// test-file-shape change that would need an allowlist entry to stay green.
//
// Detection is intentionally per-function and lexical, not interprocedural:
// a FuncDecl's body is flagged only if IT directly calls tui.New(...)/
// app.New(...) AND establishes a TempDir/A9S_CONFIG_FOLDER signal somewhere
// in its own body (including nested closures, e.g. a t.Run subtest literal)
// AND that same body never references any selector named "CloseController"
// or "Close" (e.g. m.CloseController(), c.Close, t.Cleanup(c.Close)). A
// helper like newEC2ListModel that already closes correctly is never
// flagged; every direct caller of such a helper is also never flagged,
// because IT does not itself call tui.New/app.New.
//
// Scope: every ../../tests/unit/*.go file (this package's own directory,
// non-recursive) plus every top-level ../../tests/integration/*.go file
// (its scenario/*.go subdirectories, e.g. tests/integration/web/, are out of
// scope — not "trivially includable" as a single flat glob).
//
// VERIFIED CENSUS (2026-07-15, narrowed scanner run by hand against HEAD
// after the 8 Close fixes below landed): 0 violations. The 8 sites the
// narrowed rule matches (qa_ec2_test.go: TestQA_EC2_A4_StatusColoring_
// StoppedRowHasANSI, TestQA_EC2_A12_1_EmptyInstanceList,
// TestQA_EC2_A13_1_LoadingState, TestQA_EC2_A14_1_TerminalTooNarrow,
// TestQA_EC2_A14_5_TerminalTooShort, TestQA_EC2_D1_FullNavigationStack;
// qa_load_more_dedup_test.go: TestLoadMore_TUI_ColdOpen_NoDuplicates;
// detail_ports_test.go:
// Test_Detail_EnterOnNavigableField_TUIKeyRoute_NavigatesToTarget) all
// already carry a t.Cleanup(func() { m.CloseController() }) placed
// immediately after construction, after their existing t.TempDir()/t.Setenv
// line, so LIFO cleanup order closes the controller before testing.T's own
// TempDir RemoveAll runs.
package unit_test

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// ccldConstructSite is one Close-discipline violation found by ccldScanFile:
// a function whose body calls tui.New/app.New but never references a Close.
type ccldConstructSite struct {
	file string
	line int
	fn   string
	kind string
}

func (v ccldConstructSite) String() string {
	return fmt.Sprintf("%s:%d: func %s constructs via %s but its body never references CloseController/Close",
		v.file, v.line, v.fn, v.kind)
}

// ccldRootConstructKind reports whether call is a "tui.New(...)" or
// "app.New(...)" call, matched purely on the selector's package identifier
// and method name (lexical, not type-resolved) — the same convention
// qa_multifinding_no_legacy_gate_test.go's mfnlScanFileForDirectWrite uses
// for its own selector matching.
func ccldRootConstructKind(call *ast.CallExpr) (kind string, ok bool) {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "New" {
		return "", false
	}
	pkg, ok := sel.X.(*ast.Ident)
	if !ok {
		return "", false
	}
	switch pkg.Name {
	case "tui":
		return "tui.New", true
	case "app":
		return "app.New", true
	default:
		return "", false
	}
}

// ccldBodyReferencesClose reports whether body contains any selector
// expression named "CloseController" or "Close" anywhere in its subtree
// (including nested function literals), e.g. m.CloseController(), c.Close,
// or t.Cleanup(c.Close).
func ccldBodyReferencesClose(body ast.Node) bool {
	found := false
	ast.Inspect(body, func(n ast.Node) bool {
		if found {
			return false
		}
		sel, ok := n.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		if sel.Sel.Name == "CloseController" || sel.Sel.Name == "Close" {
			found = true
			return false
		}
		return true
	})
	return found
}

// ccldBodyOwnsTempDirBackedConfigFolder reports whether body establishes a
// temp-dir-backed A9S_CONFIG_FOLDER in its own scope — either a call to
// t.TempDir() (any selector named "TempDir"), or a Setenv call (t.Setenv/
// os.Setenv) whose "A9S_CONFIG_FOLDER" argument is a literal string anywhere
// in the body. This is the gate's narrowing precondition (see file header
// "NARROWED SCOPE"): only a function that owns such a directory can race its
// own leaked writer against its own TempDir RemoveAll.
func ccldBodyOwnsTempDirBackedConfigFolder(body ast.Node) bool {
	found := false
	ast.Inspect(body, func(n ast.Node) bool {
		if found {
			return false
		}
		switch v := n.(type) {
		case *ast.SelectorExpr:
			if v.Sel.Name == "TempDir" {
				found = true
				return false
			}
		case *ast.BasicLit:
			if strings.Contains(v.Value, "A9S_CONFIG_FOLDER") {
				found = true
				return false
			}
		}
		return true
	})
	return found
}

// ccldScanFile parses path and returns one ccldConstructSite for every
// top-level function declaration whose body (a) calls tui.New/app.New at
// least once, AND (b) owns a temp-dir-backed A9S_CONFIG_FOLDER in that same
// body (ccldBodyOwnsTempDirBackedConfigFolder — the gate's narrowing
// precondition, see file header), but (c) never references a Close anywhere
// in that same body.
func ccldScanFile(fset *token.FileSet, path string) ([]ccldConstructSite, error) {
	src, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		return nil, err
	}
	rel := filepath.Base(path)
	var violations []ccldConstructSite
	for _, decl := range src.Decls {
		fd, ok := decl.(*ast.FuncDecl)
		if !ok || fd.Body == nil {
			continue
		}

		var constructKind string
		var constructPos token.Pos
		ast.Inspect(fd.Body, func(n ast.Node) bool {
			if constructKind != "" {
				return false
			}
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			if kind, ok := ccldRootConstructKind(call); ok {
				constructKind = kind
				constructPos = call.Pos()
			}
			return true
		})
		if constructKind == "" {
			continue
		}
		if !ccldBodyOwnsTempDirBackedConfigFolder(fd.Body) {
			continue
		}
		if ccldBodyReferencesClose(fd.Body) {
			continue
		}

		pos := fset.Position(constructPos)
		violations = append(violations, ccldConstructSite{
			file: rel,
			line: pos.Line,
			fn:   fd.Name.Name,
			kind: constructKind,
		})
	}
	return violations, nil
}

// ccldScanGlob globs pattern and scans every matched file via ccldScanFile,
// failing the test via t.Fatalf on any glob/parse error rather than
// silently under-scanning.
func ccldScanGlob(t *testing.T, pattern string) []ccldConstructSite {
	t.Helper()
	files, err := filepath.Glob(pattern)
	if err != nil {
		t.Fatalf("filepath.Glob(%q): %v", pattern, err)
	}
	if len(files) == 0 {
		t.Fatalf("no files matched %q — check path/glob", pattern)
	}
	fset := token.NewFileSet()
	var violations []ccldConstructSite
	for _, path := range files {
		found, scanErr := ccldScanFile(fset, path)
		if scanErr != nil {
			t.Fatalf("parse error in %s: %v", path, scanErr)
		}
		violations = append(violations, found...)
	}
	return violations
}

// TestControllerCloseDiscipline_EveryRootConstructorClosesInSameBody is the
// gate: every function across tests/unit/*.go and tests/integration/*.go
// that calls tui.New(...)/app.New(...) AND owns a temp-dir-backed
// A9S_CONFIG_FOLDER in its own body must also reference CloseController/
// Close in that same body (typically via t.Cleanup(m.CloseController) or
// t.Cleanup(c.Close), registered AFTER t.TempDir()/t.Setenv so LIFO cleanup
// runs Close before any TempDir RemoveAll). A construct call with no such
// directory in-body is out of scope — see file header "NARROWED SCOPE" for
// why it cannot reproduce the race this gate targets. See this file's
// header for the verified 0-violation census.
func TestControllerCloseDiscipline_EveryRootConstructorClosesInSameBody(t *testing.T) {
	var violations []ccldConstructSite
	violations = append(violations, ccldScanGlob(t, "*.go")...)
	violations = append(violations, ccldScanGlob(t, filepath.Join("..", "integration", "*.go"))...)

	if len(violations) == 0 {
		return
	}
	lines := make([]string, len(violations))
	for i, v := range violations {
		lines[i] = v.String()
	}
	sort.Strings(lines)
	t.Errorf(
		"%d function(s) construct a root model/controller without closing it in the same body:\n%s\n\n"+
			"Add t.Cleanup(func() { m.CloseController() }) (tui.New) or t.Cleanup(c.Close) (app.New) "+
			"immediately after the constructor call, registered AFTER any t.TempDir()/t.Setenv so LIFO "+
			"cleanup order closes the controller before its cache directory is removed — a leaked "+
			"availability-save goroutine racing the test's own TempDir RemoveAll is the exact "+
			"'directory is not empty' failure this gate exists to prevent.",
		len(violations), strings.Join(lines, "\n"),
	)
}
