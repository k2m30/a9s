// qa_handle_funnel_discipline_test.go — S2: the Handle funnel gate. P2 found
// internal/tui/app.go's CostsLoaded case discarding Controller.Handle's
// returned []runtime.TaskRequest entirely (`m.ctrl.Handle(msg); return m,
// nil`) — the N3 granularity fallback's own re-fetch task was computed and
// then silently dropped, leaving the frame Loading forever in the TUI lane
// alone (the headless lane, which routes through the same Handle but never
// discards the return, never showed the bug). That specific case is fixed
// (see internal/tui/app.go's CostsLoaded case), but the fix was a single
// call-site edit, not a structural guarantee — nothing stops a FUTURE
// ctrl.Handle call site (a new event case, or a refactor of the existing
// one) from reintroducing the same discard.
//
// This is a standing ratchet, not a burn-down: it scans every *.go file
// under internal/tui (excluding tests) for a direct `<expr>.ctrl.Handle(...)`
// call and asserts the returned second value (the []runtime.TaskRequest) is
// never assigned to the blank identifier. Parameterized over every call site
// found — today there is exactly one (app.go's CostsLoaded case), but a
// second event case wired through Handle in the future is covered
// automatically, with no allowlist to maintain (there is no known debt: the
// one real call site is already correct).
package unit_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"runtime"
	"testing"
)

// hfdSite is one `<expr>.ctrl.Handle(...)` call site found by hfdScanFile.
type hfdSite struct {
	file          string
	line          int
	discardsTasks bool
}

// hfdIsCtrlHandleCall reports whether call is a direct `<expr>.ctrl.Handle(...)`
// call — the Fun selector's method is "Handle" and its receiver is itself a
// selector ending in ".ctrl" (matches m.ctrl.Handle, (*Model).ctrl.Handle
// after desugaring, and any future receiver spelling ending the same way).
func hfdIsCtrlHandleCall(call *ast.CallExpr) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "Handle" {
		return false
	}
	recv, ok := sel.X.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	return recv.Sel.Name == "ctrl"
}

// hfdScanFile parses path and returns every `.ctrl.Handle(...)` call site,
// each flagged with whether its second (TaskRequest) return value is
// discarded via `_` in its enclosing two-value assignment.
func hfdScanFile(fset *token.FileSet, path string) ([]hfdSite, error) {
	src, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		return nil, err
	}

	var sites []hfdSite
	ast.Inspect(src, func(n ast.Node) bool {
		assign, ok := n.(*ast.AssignStmt)
		if !ok || len(assign.Rhs) != 1 {
			return true
		}
		call, ok := assign.Rhs[0].(*ast.CallExpr)
		if !ok || !hfdIsCtrlHandleCall(call) {
			return true
		}
		pos := fset.Position(call.Pos())
		discards := false
		if len(assign.Lhs) == 2 {
			if id, ok := assign.Lhs[1].(*ast.Ident); ok && id.Name == "_" {
				discards = true
			}
		} else {
			// A Handle call not destructured into exactly two LHS values
			// (e.g. called as a bare ExprStmt via a wrapper, or a single-
			// value assignment) cannot be proven to route its tasks
			// anywhere — treated as a discard, the same as `_`.
			discards = true
		}
		sites = append(sites, hfdSite{file: filepath.Base(path), line: pos.Line, discardsTasks: discards})
		return true
	})

	// A bare `m.ctrl.Handle(msg)` used as its own ExprStmt (no assignment at
	// all — both return values dropped) is not an AssignStmt and would be
	// invisible to the scan above; catch it explicitly.
	ast.Inspect(src, func(n ast.Node) bool {
		exprStmt, ok := n.(*ast.ExprStmt)
		if !ok {
			return true
		}
		call, ok := exprStmt.X.(*ast.CallExpr)
		if !ok || !hfdIsCtrlHandleCall(call) {
			return true
		}
		pos := fset.Position(call.Pos())
		sites = append(sites, hfdSite{file: filepath.Base(path), line: pos.Line, discardsTasks: true})
		return true
	})

	return sites, nil
}

// TestHandleFunnelDiscipline_NoTUICallSiteDiscardsReturnedTasks is the S2
// gate: every `.ctrl.Handle(...)` call site under internal/tui must route
// its returned []runtime.TaskRequest somewhere (never `_`, never dropped as
// a bare statement) — P2's specific bug, generalized to a standing ratchet
// over every message kind Handle is ever wired to return tasks for.
func TestHandleFunnelDiscipline_NoTUICallSiteDiscardsReturnedTasks(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller(0) failed — cannot locate test file")
	}
	// tests/unit/qa_handle_funnel_discipline_test.go -> ../../internal/tui
	tuiDir := filepath.Join(filepath.Dir(thisFile), "..", "..", "internal", "tui")

	pattern := filepath.Join(tuiDir, "*.go")
	files, err := filepath.Glob(pattern)
	if err != nil {
		t.Fatalf("filepath.Glob(%q): %v", pattern, err)
	}
	if len(files) == 0 {
		t.Fatalf("no .go files found under %s — check path", tuiDir)
	}

	fset := token.NewFileSet()
	var allSites []hfdSite
	for _, path := range files {
		if filepath.Ext(path) != ".go" {
			continue
		}
		base := filepath.Base(path)
		if len(base) > len("_test.go") && base[len(base)-len("_test.go"):] == "_test.go" {
			continue
		}
		sites, scanErr := hfdScanFile(fset, path)
		if scanErr != nil {
			t.Errorf("parse error in %s: %v", path, scanErr)
			continue
		}
		allSites = append(allSites, sites...)
	}

	if len(allSites) == 0 {
		t.Fatal("no .ctrl.Handle(...) call site found under internal/tui — this gate has nothing to guard; if Handle's call site was renamed/refactored away, update hfdIsCtrlHandleCall to match its new shape rather than leaving this gate silently vacuous")
	}

	found := 0
	for _, site := range allSites {
		found++
		t.Run(site.file, func(t *testing.T) {
			if site.discardsTasks {
				t.Errorf("%s:%d — this .ctrl.Handle(...) call site discards its returned []runtime.TaskRequest (via `_` or a dropped return); a task Handle computes here (e.g. a granularity-fallback re-fetch) is silently lost — route both return values through the shared dispatcher (m.dispatchTaskRequests) instead, per P2", site.file, site.line)
			}
		})
	}

	if found != 1 {
		t.Logf("informational: %d .ctrl.Handle(...) call site(s) found under internal/tui (was 1 at the time this gate was written — a second site means Handle grew a new caller, not a problem by itself, just confirming this gate is still watching all of them)", found)
	}
}
