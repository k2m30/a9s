// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// Absorbing a fetch result does a lot of work per row, and every reader of the
// controller waits while its write lock is held. The row work is pure, so it
// runs with the lock free and only the built body is swapped in under it.
package unit

import (
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/k2m30/a9s/v3/core/app"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
)

// wipfixEC2Rows returns n rows in the shape the ec2 fetcher writes.
func wipfixEC2Rows(n int) []resource.Resource {
	rows := make([]resource.Resource, n)
	for i := range rows {
		id := "i-" + strconv.Itoa(1000000000000000+i)
		rows[i] = resource.Resource{
			ID:   id,
			Name: "example-instance-" + strconv.Itoa(i),
			Fields: map[string]string{
				"instance_id": id, "state": "running", "instance_type": "t3.micro",
				"az": "us-east-1a", "private_ip": "10.0.1.10", "vpc_id": "vpc-0123456789abcdef0",
			},
		}
	}
	return rows
}

// TestLargeFetchAbsorb_RowWorkRunsWithTheLockFree: every cell a page's rows
// produce is built while no goroutine holds the controller's write lock, and
// the body that build produced is the one the list shows.
func TestLargeFetchAbsorb_RowWorkRunsWithTheLockFree(t *testing.T) {
	const short = "views7-absorb-probe"
	const n = 6000
	var c *app.Controller
	built, locked := 0, 0
	resource.SetChildTypeForTest(resource.ResourceTypeDef{
		Name:      "Absorb probe",
		ShortName: short,
		Columns:   []domain.Column{{Key: "name", Title: "Name", Width: 30}},
		CellDecorators: map[string]func(domain.Resource, string) string{
			"name": func(_ domain.Resource, v string) string {
				built++
				if c.WriteLockedForTest() {
					locked++
				}
				return v
			},
		},
	})
	t.Cleanup(func() { resource.CleanupChildTypeForTest(short) })

	c = newTestController(t)
	c.PushChildListScreen(short)
	rows := make([]resource.Resource, n)
	for i := range rows {
		id := "probe-" + strconv.Itoa(i)
		rows[i] = resource.Resource{ID: id, Name: id, Fields: map[string]string{"name": id}}
	}
	vs, _ := handlePage(c, messages.ResourcesLoaded{
		ResourceType: short,
		Resources:    rows,
		Pagination:   &domain.PaginationMeta{IsTruncated: false},
		Provenance:   messages.FetchProvenanceChild,
	})

	if body := vs.Body.List; body == nil || len(body.Rows) != n {
		got := 0
		if body != nil {
			got = len(body.Rows)
		}
		t.Fatalf("the list holds %d rows after absorbing %d", got, n)
	}
	if built < n {
		t.Fatalf("absorbing %d rows built %d cells; the probe column was not rendered", n, built)
	}
	if locked > 0 {
		t.Errorf("absorbing %d rows built %d of %d cells with the controller's write lock held; "+
			"the row work belongs outside the lock, with only the built body swapped under it", n, locked, built)
	}
	before := built
	_ = c.Snapshot()
	if built != before {
		t.Errorf("the snapshot after the absorb rebuilt %d cells; the body built off the lock was not the one installed", built-before)
	}
}

// views7TimingPinFiles parses every test file of tests/unit.
func views7TimingPinFiles(t *testing.T) (*token.FileSet, map[string]*ast.File) {
	t.Helper()
	dir := filepath.Join(projectRoot(t), "tests", "unit")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read tests/unit: %v", err)
	}
	fset := token.NewFileSet()
	files := map[string]*ast.File{}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		f, parseErr := parser.ParseFile(fset, filepath.Join(dir, e.Name()), nil, parser.SkipObjectResolution)
		if parseErr != nil {
			t.Fatalf("parse %s: %v", e.Name(), parseErr)
		}
		files[e.Name()] = f
	}
	return fset, files
}

// views7DurationUnits are the constants a wall-clock budget is written in.
var views7DurationUnits = map[string]bool{
	"Nanosecond": true, "Microsecond": true, "Millisecond": true,
	"Second": true, "Minute": true, "Hour": true,
}

// views7HasDurationUnit reports whether an expression mentions a time unit.
func views7HasDurationUnit(n ast.Node) bool {
	found := false
	ast.Inspect(n, func(x ast.Node) bool {
		sel, ok := x.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		if pkg, ok := sel.X.(*ast.Ident); ok && pkg.Name == "time" && views7DurationUnits[sel.Sel.Name] {
			found = true
		}
		return !found
	})
	return found
}

// views7HasDurationUnitList reports whether any expression mentions a time unit.
func views7HasDurationUnitList(exprs []ast.Expr) bool {
	for _, e := range exprs {
		if views7HasDurationUnit(e) {
			return true
		}
	}
	return false
}

// views7FailsTheTest reports whether a block reports a test failure, which is
// what separates a budget from a poll condition or a stall guard.
func views7FailsTheTest(n ast.Node) bool {
	found := false
	ast.Inspect(n, func(x ast.Node) bool {
		call, ok := x.(*ast.CallExpr)
		if !ok {
			return true
		}
		if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
			switch sel.Sel.Name {
			case "Errorf", "Error", "Fatalf", "Fatal":
				found = true
			}
		}
		return !found
	})
	return found
}

// views7NamesADurationConstant reports whether an expression reads a duration
// the test wrote down.
func views7NamesADurationConstant(e ast.Expr, constants map[string]bool) bool {
	if views7HasDurationUnit(e) {
		return true
	}
	found := false
	ast.Inspect(e, func(x ast.Node) bool {
		if id, ok := x.(*ast.Ident); ok && constants[id.Name] {
			found = true
		}
		return !found
	})
	return found
}

// views7DurationResults maps each helper that hands a test a duration to the
// result positions that carry one. The position matters: a helper that returns
// a ratio beside two spans hands its caller a number, not a budget.
func views7DurationResults(files map[string]*ast.File, measuring bool) map[string]map[int]bool {
	out := map[string]map[int]bool{}
	for _, f := range files {
		for _, decl := range f.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Type.Results == nil {
				continue
			}
			if views7ReadsAClock(fn.Body) != measuring {
				continue
			}
			at := 0
			for _, r := range fn.Type.Results.List {
				n := max(len(r.Names), 1)
				sel, isSel := r.Type.(*ast.SelectorExpr)
				isDuration := false
				if isSel {
					if pkg, ok := sel.X.(*ast.Ident); ok && pkg.Name == "time" && sel.Sel.Name == "Duration" {
						isDuration = true
					}
				}
				for range n {
					if isDuration {
						if out[fn.Name.Name] == nil {
							out[fn.Name.Name] = map[int]bool{}
						}
						out[fn.Name.Name][at] = true
					}
					at++
				}
			}
		}
	}
	return out
}

// views7ClassifyDurationHelpers splits the duration-returning helpers into the
// ones that hand back a span they measured and the ones that hand back a
// number the test wrote. A helper that only passes on what a measuring helper
// gave it is measuring too, so the split is taken to a fixed point.
func views7ClassifyDurationHelpers(files map[string]*ast.File) (written, measuring map[string]map[int]bool) {
	all := map[string]map[int]bool{}
	for name, res := range views7DurationResults(files, false) {
		all[name] = res
	}
	for name, res := range views7DurationResults(files, true) {
		all[name] = res
	}

	measures := map[string]bool{}
	for _, f := range files {
		for _, decl := range f.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if ok && all[fn.Name.Name] != nil && views7ReadsAClock(fn.Body) {
				measures[fn.Name.Name] = true
			}
		}
	}
	for changed := true; changed; {
		changed = false
		for _, f := range files {
			for _, decl := range f.Decls {
				fn, ok := decl.(*ast.FuncDecl)
				if !ok || all[fn.Name.Name] == nil || measures[fn.Name.Name] {
					continue
				}
				ast.Inspect(fn.Body, func(n ast.Node) bool {
					call, ok := n.(*ast.CallExpr)
					if !ok {
						return true
					}
					if id, ok := call.Fun.(*ast.Ident); ok && measures[id.Name] {
						measures[fn.Name.Name] = true
						changed = true
					}
					return true
				})
			}
		}
	}

	written, measuring = map[string]map[int]bool{}, map[string]map[int]bool{}
	for name, res := range all {
		if measures[name] {
			measuring[name] = res
			continue
		}
		written[name] = res
	}
	return written, measuring
}

// views7ReadsAClock reports whether a body times something. A helper that does
// hands its caller a measurement; one that does not hands back a number the
// test wrote.
func views7ReadsAClock(n ast.Node) bool {
	if n == nil {
		return false
	}
	found := false
	ast.Inspect(n, func(x ast.Node) bool {
		sel, ok := x.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		if pkg, ok := sel.X.(*ast.Ident); ok && pkg.Name == "time" && (sel.Sel.Name == "Now" || sel.Sel.Name == "Since") {
			found = true
		}
		return !found
	})
	return found
}

// views7ConstantNamesIn returns the names that hold a written-down duration
// within one scope: assigned from a time-unit expression, or from the
// duration-returning position of a helper. Scoped rather than collected across
// the package, because a name is only a budget where it was written.
func views7ConstantNamesIn(scope ast.Node, funcs map[string]map[int]bool) map[string]bool {
	names := map[string]bool{}
	mark := func(lhs, rhs []ast.Expr) {
		if views7HasDurationUnitList(rhs) {
			for _, e := range lhs {
				if id, ok := e.(*ast.Ident); ok {
					names[id.Name] = true
				}
			}
			return
		}
		if len(rhs) != 1 {
			return
		}
		call, ok := rhs[0].(*ast.CallExpr)
		if !ok {
			return
		}
		id, ok := call.Fun.(*ast.Ident)
		if !ok || funcs[id.Name] == nil {
			return
		}
		for i, e := range lhs {
			if !funcs[id.Name][i] {
				continue
			}
			if name, ok := e.(*ast.Ident); ok {
				names[name.Name] = true
			}
		}
	}
	ast.Inspect(scope, func(n ast.Node) bool {
		switch d := n.(type) {
		case *ast.AssignStmt:
			mark(d.Lhs, d.Rhs)
		case *ast.ValueSpec:
			lhs := make([]ast.Expr, 0, len(d.Names))
			for _, id := range d.Names {
				lhs = append(lhs, id)
			}
			mark(lhs, d.Values)
		}
		return true
	})
	return names
}

// views7MeasuredNamesIn returns the names holding a span this run produced: the
// distance from a clock reading the test took, or a duration a helper measured
// and handed back.
func views7MeasuredNamesIn(scope ast.Node, funcs map[string]map[int]bool) map[string]bool {
	starts := map[string]bool{}
	names := map[string]bool{}

	callIs := func(e ast.Expr, pkg, fn string) (*ast.CallExpr, bool) {
		call, ok := e.(*ast.CallExpr)
		if !ok {
			return nil, false
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return nil, false
		}
		id, ok := sel.X.(*ast.Ident)
		return call, ok && id.Name == pkg && sel.Sel.Name == fn
	}

	// Two passes: a span is only a measurement when the clock it counts from
	// was read in the same scope, and the reading may come after it textually
	// (a helper's start line sits below its declaration in a closure).
	for range 2 {
		ast.Inspect(scope, func(n ast.Node) bool {
			as, ok := n.(*ast.AssignStmt)
			if !ok {
				return true
			}
			for i, rhs := range as.Rhs {
				if i < len(as.Lhs) {
					if _, isNow := callIs(rhs, "time", "Now"); isNow {
						if id, ok := as.Lhs[i].(*ast.Ident); ok {
							starts[id.Name] = true
						}
						continue
					}
					if call, isSince := callIs(rhs, "time", "Since"); isSince && len(call.Args) == 1 {
						if arg, ok := call.Args[0].(*ast.Ident); ok && starts[arg.Name] {
							if id, ok := as.Lhs[i].(*ast.Ident); ok {
								names[id.Name] = true
							}
						}
						continue
					}
				}
				call, ok := rhs.(*ast.CallExpr)
				if !ok || len(as.Rhs) != 1 {
					continue
				}
				id, ok := call.Fun.(*ast.Ident)
				if !ok || funcs[id.Name] == nil {
					continue
				}
				for j, lhs := range as.Lhs {
					if !funcs[id.Name][j] {
						continue
					}
					if name, ok := lhs.(*ast.Ident); ok {
						names[name.Name] = true
					}
				}
			}
			return true
		})
	}
	return names
}

// views7IsMeasured reports whether an expression is a span this run produced.
// A duration read off a fixture, or the time left on a deadline the code was
// handed, is not a measurement of how fast anything ran.
func views7IsMeasured(e ast.Expr, measured map[string]bool) bool {
	switch v := e.(type) {
	case *ast.Ident:
		return measured[v.Name]
	case *ast.ParenExpr:
		return views7IsMeasured(v.X, measured)
	case *ast.BinaryExpr:
		return views7IsMeasured(v.X, measured) || views7IsMeasured(v.Y, measured)
	}
	return false
}

// views7NamesAny reports whether an expression reads one of the named values.
func views7NamesAny(e ast.Expr, names map[string]bool) bool {
	found := false
	ast.Inspect(e, func(x ast.Node) bool {
		if id, ok := x.(*ast.Ident); ok && names[id.Name] {
			found = true
		}
		return !found
	})
	return found
}

// views7ConfiguredBounds returns the names a scope hands to the code under test
// as its own deadline. A span compared against one of those is asking whether
// a call respected the bound it was given, which is a fact about the code.
func views7ConfiguredBounds(scope ast.Node) map[string]bool {
	names := map[string]bool{}
	ast.Inspect(scope, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		if pkg, ok := sel.X.(*ast.Ident); !ok || pkg.Name != "context" {
			return true
		}
		if sel.Sel.Name != "WithTimeout" && sel.Sel.Name != "WithDeadline" {
			return true
		}
		for _, arg := range call.Args {
			ast.Inspect(arg, func(x ast.Node) bool {
				if id, ok := x.(*ast.Ident); ok {
					names[id.Name] = true
				}
				return true
			})
		}
		return true
	})
	return names
}

// TestNoTestFailsOnAWallClockBudget sweeps tests/unit for a test that fails
// when a measured span exceeds a duration written into the test.
//
// Such a pin reports the machine. It goes red on a loaded bench where the code
// is right, and green on a fast one where the code is wrong, and every red it
// produces costs someone the walk to find out which. A timing pin states a
// proportion or a count instead — both are read off the same run, so a slower
// machine moves both numbers and the verdict stands.
//
// A duration compared in a loop condition, a select case or a stall guard that
// does not fail the test is untouched: waiting for something with a deadline
// is not a measurement.
func TestNoTestFailsOnAWallClockBudget(t *testing.T) {
	fset, files := views7TimingPinFiles(t)
	written, measuring := views7ClassifyDurationHelpers(files)

	var offenders []string
	for _, f := range files {
		fileConstants := map[string]bool{}
		for _, decl := range f.Decls {
			if gd, ok := decl.(*ast.GenDecl); ok {
				for name := range views7ConstantNamesIn(gd, written) {
					fileConstants[name] = true
				}
			}
		}
		for _, decl := range f.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			constants := views7ConstantNamesIn(fn, written)
			for name := range fileConstants {
				constants[name] = true
			}
			measured := views7MeasuredNamesIn(fn, measuring)
			configured := views7ConfiguredBounds(fn)

			ast.Inspect(fn.Body, func(n ast.Node) bool {
				ifs, ok := n.(*ast.IfStmt)
				if !ok || !views7FailsTheTest(ifs.Body) {
					return true
				}
				ast.Inspect(ifs.Cond, func(x ast.Node) bool {
					bin, ok := x.(*ast.BinaryExpr)
					if !ok {
						return true
					}
					switch bin.Op {
					case token.GTR, token.GEQ, token.LSS, token.LEQ:
					default:
						return true
					}
					budget, span := bin.X, bin.Y
					if !views7NamesADurationConstant(budget, constants) {
						budget, span = bin.Y, bin.X
					}
					if !views7NamesADurationConstant(budget, constants) || !views7IsMeasured(span, measured) {
						return true
					}
					if views7NamesADurationConstant(span, constants) || views7NamesAny(budget, configured) {
						return true
					}
					var buf strings.Builder
					_ = printer.Fprint(&buf, fset, bin) //nolint:errcheck // a failure message, not a result
					offenders = append(offenders, fset.Position(bin.Pos()).String()+": "+buf.String())
					return true
				})
				return true
			})
		}
	}
	sort.Strings(offenders)
	for _, o := range offenders {
		t.Errorf("a test fails on a wall-clock budget: %s — the number is this machine on this day. "+
			"Pin the shape instead: the share of the whole operation the span is, or the work done in it", o)
	}
}
