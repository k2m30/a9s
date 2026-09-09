// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// views7_absorb_shape_test.go — a timing pin that measures the code.
//
// "The lock is held for under 35 ms" is a statement about the machine the
// suite happens to run on. It is green on an idle bench and red beside a full
// -race run, and neither reading says whether the row work is under the lock.
//
// What the row is about is a proportion: absorbing a fetch result does a lot
// of work, and only the swap of the built body may happen with the lock held.
// A fraction is read off two clocks that move together, so a loaded machine
// stretches both and the verdict does not change.
package unit

import (
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/k2m30/a9s/v3/core/app"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
)

// views7LockedFraction is the largest share of an absorption that may pass
// with the controller lock held. It separates two shapes, not two machines:
// doing the row work under the lock spends about a third to a half of the
// absorption there (0.46 ordinary, 0.30 under -race, measured against the tree
// before the work moved off the lock), and swapping a body built outside it
// spends 0.05 to 0.07 in either build. A loaded machine stretches the hold and
// the absorption together, so the reading stays on its own side of the bound.
const views7LockedFraction = 0.2

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

// views7AbsorbSpans absorbs n rows on a fresh controller and returns the
// longest a concurrent lock-taking reader was blocked, and how long the whole
// absorption took. The probe is Controller.GetListLane — an RLock and two
// pointer reads — so what blocks it is the writer's hold and nothing else.
func views7AbsorbSpans(t *testing.T, n int) (held, total time.Duration) {
	t.Helper()
	c := newTestController(t)
	_, _ = c.Apply(app.Action{Kind: app.ActionCommand, Arg: "ec2"})
	rows := wipfixEC2Rows(n)

	var stop atomic.Bool
	worst := make(chan time.Duration, 1)
	go func() {
		var longest time.Duration
		for !stop.Load() {
			start := time.Now()
			c.GetListLane()
			if d := time.Since(start); d > longest {
				longest = d
			}
		}
		worst <- longest
	}()

	// Let the probe loop reach steady state so its own first-call costs are
	// not read as contention.
	time.Sleep(5 * time.Millisecond)

	start := time.Now()
	_, _ = handlePage(c, messages.ResourcesLoaded{
		ResourceType: "ec2",
		Resources:    rows,
		Pagination:   &domain.PaginationMeta{IsTruncated: false},
		Provenance:   messages.FetchProvenanceCanonicalList,
	})
	total = time.Since(start)

	stop.Store(true)
	held = <-worst

	if body := c.Snapshot().Body.List; body == nil || len(body.Rows) != n {
		got := 0
		if body != nil {
			got = len(body.Rows)
		}
		t.Fatalf("the list holds %d rows after absorbing %d — keeping work off the lock must not lose it", got, n)
	}
	return held, total
}

// views7MinLockedShare returns the smallest locked share observed over a few
// absorptions. A blocked reader's wait is the writer's hold plus whatever the
// scheduler added, never less, so the smallest reading is the closest one to
// the hold itself.
func views7MinLockedShare(t *testing.T, n int) (share float64, held, total time.Duration) {
	t.Helper()
	share = math.MaxFloat64
	for range 3 {
		h, tot := views7AbsorbSpans(t, n)
		if tot <= 0 {
			t.Fatalf("absorbing %d rows took no measurable time", n)
		}
		if s := float64(h) / float64(tot); s < share {
			share, held, total = s, h, tot
		}
	}
	return share, held, total
}

// TestLargeFetchAbsorb_LockIsHeldForASmallPartOfTheAbsorb is row 23's pin
// rewritten as the shape it is about. The two sizes are what separates a swap
// from row work: a swap is a small share of a larger absorption and a smaller
// share of a larger one still, while row work under the lock is nearly all of
// either.
func TestLargeFetchAbsorb_LockIsHeldForASmallPartOfTheAbsorb(t *testing.T) {
	for _, n := range []int{6000, 12000} {
		share, held, total := views7MinLockedShare(t, n)
		if share > views7LockedFraction {
			t.Errorf("absorbing %d rows spent %.0f%% of the absorption with the controller lock held "+
				"(%v of %v) — the row work is pure in-memory and belongs outside the lock, with only the "+
				"built body swapped under it", n, share*100, held, total)
		}
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

// TestNoTestFailsOnAWallClockBudget sweeps tests/unit for the same defect the
// pin above was rewritten out of: a test that fails when a measured span
// exceeds a duration written into the test.
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
