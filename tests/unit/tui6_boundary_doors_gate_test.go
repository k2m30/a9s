// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package unit_test

// tui6_boundary_doors_gate_test.go — every door into the controller that
// carries AWS-supplied text reaches the boundary that cleans it.
//
// The boundary is not a place in the code, it is a rule about a set of
// functions, and a rule nothing enumerates is a comment. That is not a
// hypothetical: three doors shipped past it while seven behavioural pins were
// green, including the one the terminal actually loads a page through, so no
// page the real app fetched was ever cleaned. Each of those pins drives a door
// it knows about; none of them can see a door nobody wrote a pin for.
//
// So the doors are enumerated from the code. A door is an exported method on
// the controller whose parameters carry AWS-supplied text: a resource, a page
// of them, findings, attention rows, or an error whose message quotes what AWS
// rejected. Every door must reach a boundary call, directly or through the
// package's own functions. A door that reads its argument without ever writing
// painted text is listed below with the reason; the list is exceptions, not
// writers, so a new door is a failure until someone decides which it is.

import (
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/k2m30/a9s/v3/core/costs"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
)

// The two classes of AWS-supplied text a door can carry. They are checked
// separately because the calls that clean them are different calls.
const (
	rowCarrier = iota
	textCarrier
)

// rowBoundaryCalls clean a resource, its findings or its attention rows.
// domain.Sanitize is deliberately NOT one of them: it cleans a single string,
// so a door that carries a page and happens to sanitise an error message on
// one of its branches would otherwise pass with the page untouched. That is
// not hypothetical — the door the terminal loads every page through reaches
// ListState.setFetchError two calls down, and a gate that took any cleaning
// call as proof would have called it compliant while no page was ever cleaned.
var rowBoundaryCalls = map[string]bool{ //nolint:gochecknoglobals // test-only lookup
	"Sanitized":                 true, // Resource.Sanitized
	"SanitizedRows":             true,
	"SanitizedFindings":         true,
	"SanitizedAttentionDetails": true,
}

// textBoundaryCalls clean a single string: an AWS error's message, or a name
// on its way to a title.
var textBoundaryCalls = map[string]bool{ //nolint:gochecknoglobals // test-only lookup
	"Sanitize":      true, // domain.Sanitize
	"setFetchError": true, // the one writer of the list's error marker
}

// carrierTypes are the leaf types that carry text an AWS response wrote. A
// parameter counts as a carrier when one of these appears anywhere in its
// type — inside a slice, a map, a pointer or a variadic — or when it is a
// message whose own fields carry one.
var rowCarriers = map[string]bool{ //nolint:gochecknoglobals // test-only lookup
	"resource.Resource":      true,
	"domain.Finding":         true,
	"domain.AttentionDetail": true,
}

// textCarriers carry AWS-supplied text without carrying a resource: an error
// whose message quotes the input AWS refused.
var textCarriers = map[string]bool{ //nolint:gochecknoglobals // test-only lookup
	"error": true,
}

// doorsThatWriteNoText are the exceptions: exported methods that take a
// carrier and never put its text on a surface. Each needs its reason, and a
// door added here is a claim someone has to defend, which is the point — the
// default for a new door is "must sanitise".
var doorsThatWriteNoText = map[string]string{ //nolint:gochecknoglobals // test-only lookup
	"ReplayRelatedCache":         "takes a resource to match related-panel cache entries against; writes no painted text",
	"PatchListReapplyChecker":    "takes a resource to re-run a related checker over; writes no painted text",
	"ApplyReapplyCheckerAgainst": "merges the matched IDs of a page into RelatedIDSet; no text off the page is kept",
	"BeginDetailWorkload":        "hands its resource to the detail lane, which sanitises at ensureDetailState",
}

// typeIdents collects every named type in e — the leaves of a slice, map,
// pointer or variadic — so a carrier is spotted whatever it is wrapped in
// without a case per shape, and without the substring match that would read
// resource.ResourceTypeDef as resource.Resource.
func typeIdents(e ast.Expr, out map[string]bool) {
	switch t := e.(type) {
	case *ast.Ident:
		out[t.Name] = true
	case *ast.SelectorExpr:
		if pkg, isIdent := t.X.(*ast.Ident); isIdent {
			out[pkg.Name+"."+t.Sel.Name] = true
		}
	case *ast.StarExpr:
		typeIdents(t.X, out)
	case *ast.ArrayType:
		typeIdents(t.Elt, out)
	case *ast.Ellipsis:
		typeIdents(t.Elt, out)
	case *ast.MapType:
		typeIdents(t.Key, out)
		typeIdents(t.Value, out)
	}
}

// recvTypeName returns the receiver's type name without its pointer star.
func recvTypeName(e ast.Expr) string {
	names := map[string]bool{}
	typeIdents(e, names)
	for name := range names {
		return name
	}
	return ""
}

// carryingMessages reads core/runtime/messages and returns every message type
// with a carrier-typed field. A door that takes one of these takes a page of
// rows, findings or an AWS error by another name — which is what the door the
// terminal loads every page through does, and why it was missed.
func carryingMessages(t *testing.T, repoRoot string) map[int]map[string]map[string]bool {
	t.Helper()
	fset := token.NewFileSet()
	dir := filepath.Join(repoRoot, "core", "runtime", "messages")
	pkgs, err := parser.ParseDir(fset, dir, notATest, 0)
	if err != nil {
		t.Fatalf("parsing %s: %v", dir, err)
	}
	out := map[int]map[string]map[string]bool{rowCarrier: {}, textCarrier: {}}
	for _, pkg := range pkgs {
		for _, file := range pkg.Files {
			ast.Inspect(file, func(n ast.Node) bool {
				spec, isType := n.(*ast.TypeSpec)
				if !isType {
					return true
				}
				st, isStruct := spec.Type.(*ast.StructType)
				if !isStruct || st.Fields == nil {
					return true
				}
				for _, field := range st.Fields.List {
					names := map[string]bool{}
					typeIdents(field.Type, names)
					for name := range names {
						for class, carriers := range map[int]map[string]bool{rowCarrier: rowCarriers, textCarrier: textCarriers} {
							if !carriers[name] {
								continue
							}
							key := "messages." + spec.Name.Name
							if out[class][key] == nil {
								out[class][key] = map[string]bool{}
							}
							for _, fieldName := range field.Names {
								out[class][key][fieldName.Name] = true
							}
						}
					}
				}
				return true
			})
		}
	}
	if len(out[rowCarrier]) == 0 || len(out[textCarrier]) == 0 {
		t.Fatal("no message type carries a resource or an error — the gate is not reading core/runtime/messages")
	}
	return out
}

// carriesAWSText reports which kinds of AWS-supplied text fn's parameters
// carry, directly or as a message that holds one.
func carriesAWSText(fn *ast.FuncDecl, messages map[int]map[string]map[string]bool) (rows, text bool) {
	if fn.Type.Params == nil {
		return false, false
	}
	for _, field := range fn.Type.Params.List {
		names := map[string]bool{}
		typeIdents(field.Type, names)
		for name := range names {
			if rowCarriers[name] || messages[rowCarrier][name] != nil {
				rows = true
			}
			if textCarriers[name] || messages[textCarrier][name] != nil {
				text = true
			}
		}
	}
	return rows, text
}

func notATest(fi fs.FileInfo) bool {
	return !strings.HasSuffix(fi.Name(), "_test.go")
}

// controllerMethod returns the method name when fn is a method on Controller,
// else "".
func controllerMethod(fn *ast.FuncDecl) string {
	if fn.Recv == nil || len(fn.Recv.List) != 1 {
		return ""
	}
	if recvTypeName(fn.Recv.List[0].Type) != "Controller" {
		return ""
	}
	return fn.Name.Name
}

// calleesOf returns the names of every function fn calls: a bare call by its
// identifier, a method or qualified call by its selector. Both resolve against
// the package's own function table, and a selector also matches a boundary
// call written as domain.Sanitize or r.Sanitized().
func calleesOf(fn *ast.FuncDecl) []string {
	var out []string
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		switch f := call.Fun.(type) {
		case *ast.Ident:
			out = append(out, f.Name)
		case *ast.SelectorExpr:
			out = append(out, f.Sel.Name)
		}
		return true
	})
	return out
}

// forwardedParams maps the arguments of call that carry a tracked value onto
// the parameter names they land in. Following the position is what keeps the
// walk on the value: a page handed to one parameter and a canonical short name
// handed to the next are not the same thing, and tracking both makes every
// door downstream look compliant.
func forwardedParams(fn *ast.FuncDecl, call *ast.CallExpr, tracked map[string]bool) map[string]bool {
	var params []string
	if fn.Type.Params != nil {
		for _, field := range fn.Type.Params.List {
			for _, name := range field.Names {
				params = append(params, name.Name)
			}
		}
	}
	out := map[string]bool{}
	for i, arg := range call.Args {
		if i < len(params) && mentions(arg, tracked) {
			out[params[i]] = true
		}
	}
	return out
}

// mentions reports whether e reads one of the tracked identifiers — the
// parameter itself, a field of it, an element of it.
func mentions(e ast.Expr, tracked map[string]bool) bool {
	found := false
	ast.Inspect(e, func(n ast.Node) bool {
		switch t := n.(type) {
		case *ast.SelectorExpr:
			// A message parameter is tracked field by field ("msg.Resources"),
			// not whole: a canonical short name read off the same message is a
			// string, and tracking it would follow the walk everywhere the name
			// goes instead of everywhere the page goes.
			if x, isIdent := t.X.(*ast.Ident); isIdent && tracked[x.Name+"."+t.Sel.Name] {
				found = true
			}
		case *ast.Ident:
			if tracked[t.Name] {
				found = true
			}
		}
		return !found
	})
	return found
}

// cleansItsArgument reports whether the door named name cleans what it was
// handed, following the argument rather than the package.
//
// Following the package is what a plain reachability walk does, and in a
// package as connected as core/app it is satisfied for free: the door the
// terminal loads every page through reaches ListState.setFetchError two calls
// down, on the branch that installs an error marker, with the page itself
// untouched. So the walk only descends into a callee the tracked value is
// actually passed to, and only a call that cleans THAT class of text counts —
// an error message cleaned on some other branch is not a page cleaned.
func cleansItsArgument(name string, tracked map[string]bool, boundary map[string]bool, funcs map[string]*ast.FuncDecl, seen map[string]bool) bool {
	if seen[name] {
		return false
	}
	seen[name] = true
	fn, ok := funcs[name]
	if !ok || fn.Body == nil {
		return false
	}
	expandTracked(fn.Body, tracked)
	cleaned := false
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		call, isCall := n.(*ast.CallExpr)
		if !isCall || cleaned {
			return !cleaned
		}
		var callee string
		switch f := call.Fun.(type) {
		case *ast.Ident:
			callee = f.Name
		case *ast.SelectorExpr:
			callee = f.Sel.Name
			// r.Sanitized() reads the receiver, not an argument.
			if boundary[callee] && mentions(f.X, tracked) {
				cleaned = true
				return false
			}
		}
		carries := false
		for _, arg := range call.Args {
			if mentions(arg, tracked) {
				carries = true
			}
		}
		if !carries {
			return true
		}
		if boundary[callee] {
			cleaned = true
			return false
		}
		if inner, known := funcs[callee]; known && cleansItsArgument(callee, forwardedParams(inner, call, tracked), boundary, funcs, seen) {
			cleaned = true
			return false
		}
		return true
	})
	return cleaned
}

// expandTracked adds the names a tracked value is re-bound to inside body: the
// loop variables of a range over it, and the left side of an assignment that
// reads it. Without this a function that cleans each entry of a map it was
// handed reads as cleaning nothing, since the cleaning call names the loop
// variable and never the parameter. Repeated to a fixed point because a
// re-binding can be read by a later one.
func expandTracked(body *ast.BlockStmt, tracked map[string]bool) {
	for {
		before := len(tracked)
		ast.Inspect(body, func(n ast.Node) bool {
			switch st := n.(type) {
			case *ast.RangeStmt:
				if mentions(st.X, tracked) {
					for _, v := range []ast.Expr{st.Key, st.Value} {
						if id, isIdent := v.(*ast.Ident); isIdent {
							tracked[id.Name] = true
						}
					}
				}
			case *ast.AssignStmt:
				for _, rhs := range st.Rhs {
					// Only a direct reference carries the value on: a call's
					// result is a NEW value derived from it (a canonical short
					// name taken off a page is a string, not the page), and
					// following those spreads the tracking across the package
					// until every door looks compliant.
					if _, derived := rhs.(*ast.CallExpr); derived {
						continue
					}
					if !mentions(rhs, tracked) {
						continue
					}
					for _, lhs := range st.Lhs {
						if id, isIdent := lhs.(*ast.Ident); isIdent {
							tracked[id.Name] = true
						}
					}
				}
			}
			return true
		})
		if len(tracked) == before {
			return
		}
	}
}

// carrierParams returns the names of fn's parameters that carry the given
// class of AWS-supplied text.
func carrierParams(fn *ast.FuncDecl, carriers map[string]bool, messages map[string]map[string]bool) map[string]bool {
	out := map[string]bool{}
	if fn.Type.Params == nil {
		return out
	}
	for _, field := range fn.Type.Params.List {
		names := map[string]bool{}
		typeIdents(field.Type, names)
		for n := range names {
			switch {
			case carriers[n]:
				for _, name := range field.Names {
					out[name.Name] = true
				}
			case messages[n] != nil:
				for _, name := range field.Names {
					for carrierField := range messages[n] {
						out[name.Name+"."+carrierField] = true
					}
				}
			}
		}
	}
	return out
}

// TestBoundaryDoors_EveryDoorReachesTheBoundary is the enforcement half of
// the boundary: the doors are read out of core/app rather than listed here,
// and each one must reach a call that cleans what it carries.
func TestBoundaryDoors_EveryDoorReachesTheBoundary(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	repoRoot := filepath.Join(filepath.Dir(thisFile), "..", "..")
	messageCarriers := carryingMessages(t, repoRoot)
	appDir := filepath.Join(repoRoot, "core", "app")

	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, appDir, notATest, 0)
	if err != nil {
		t.Fatalf("parsing core/app: %v", err)
	}
	pkg, ok := pkgs["app"]
	if !ok {
		t.Fatalf("core/app holds no package app, only %v", pkgs)
	}

	funcs := map[string]*ast.FuncDecl{}
	for _, file := range pkg.Files {
		for _, decl := range file.Decls {
			if fn, isFunc := decl.(*ast.FuncDecl); isFunc {
				funcs[fn.Name.Name] = fn
			}
		}
	}

	var doors []string
	positions := map[string]string{}
	decls := map[string]*ast.FuncDecl{}
	carriesRows := map[string]bool{}
	carriesText := map[string]bool{}
	for _, file := range pkg.Files {
		for _, decl := range file.Decls {
			fn, isFunc := decl.(*ast.FuncDecl)
			if !isFunc {
				continue
			}
			name := controllerMethod(fn)
			if name == "" || !ast.IsExported(name) {
				continue
			}
			rows, text := carriesAWSText(fn, messageCarriers)
			if !rows && !text {
				continue
			}
			doors = append(doors, name)
			decls[name] = fn
			carriesRows[name] = rows
			carriesText[name] = text
			pos := fset.Position(fn.Pos())
			positions[name] = filepath.Base(pos.Filename) + ":" + strconv.Itoa(pos.Line)
		}
	}
	sort.Strings(doors)
	if len(doors) < 5 {
		t.Fatalf("only %d doors found (%v) — the gate is not reading core/app", len(doors), doors)
	}

	for _, door := range doors {
		if _, exempt := doorsThatWriteNoText[door]; exempt {
			continue
		}
		if carriesRows[door] && !cleansItsArgument(door, carrierParams(decls[door], rowCarriers, messageCarriers[rowCarrier]), rowBoundaryCalls, funcs, map[string]bool{}) {
			t.Errorf("%s (%s) takes a resource, findings or attention rows into the controller and reaches no call that cleans them.\n"+
				"Either sanitise it, or add it to doorsThatWriteNoText with the reason its argument never becomes painted text.\n"+
				"Doors found: %v", door, positions[door], doors)
		}
		if carriesText[door] && !carriesRows[door] && !cleansItsArgument(door, carrierParams(decls[door], textCarriers, messageCarriers[textCarrier]), textBoundaryCalls, funcs, map[string]bool{}) {
			t.Errorf("%s (%s) takes AWS error text into the controller and reaches no call that cleans it.\n"+
				"Either sanitise it, or add it to doorsThatWriteNoText with the reason its argument never becomes painted text.\n"+
				"Doors found: %v", door, positions[door], doors)
		}
	}

	for door := range doorsThatWriteNoText {
		if _, found := positions[door]; !found {
			t.Errorf("doorsThatWriteNoText names %q, which is no longer a door on Controller — delete the entry", door)
		}
	}
}

// TestHostileAWSString_CostsErrorMsgIsInert is the behavioural half of the
// violation the gate above reports, so the defect is visible as a failure
// about the screen and not only as a rule about the code.
//
// A Cost Explorer refusal quotes what it refused — a dimension value, a tag
// key — and ApplyCostsLoaded puts that message on the costs body verbatim
// (core/app/costs_state.go:1038), where buildCostsBody paints it as the
// screen's error state. It is the same lane as the list's error marker, which
// already crosses the boundary.
func TestHostileAWSString_CostsErrorMsgIsInert(t *testing.T) {
	const hostile = "web\u001b[31m-prod\u0007"
	c := newCostsController(t, fixedCostsNow)
	c.Handle(messages.CostsLoaded{
		Query: costs.Query{
			Granularity: costs.GranularityMonth.APIGranularity(),
			GroupBy:     []costs.Dimension{costs.DimensionService},
		},
		Err: errors.New("ValidationException: the dimension " + hostile + " was rejected"),
	})
	body := c.Snapshot().Body.Costs
	if body == nil {
		t.Fatal("no costs body after the refusal")
	}
	if body.ErrorMsg == "" {
		t.Fatal("the refusal left no error message on the costs body, so this pin proves nothing")
	}
	for _, r := range body.ErrorMsg {
		if r < 0x20 || r == 0x7f || (r >= 0x80 && r <= 0x9f) {
			t.Errorf("the costs error message carries the control rune %q: %q", r, body.ErrorMsg)
			break
		}
	}
	if strings.Contains(body.ErrorMsg, "[31m") {
		t.Errorf("the costs error message kept the CSI payload as literal text: %q", body.ErrorMsg)
	}
	if !strings.Contains(body.ErrorMsg, "ValidationException") {
		t.Errorf("the costs error message lost what it was reporting: %q", body.ErrorMsg)
	}
}
