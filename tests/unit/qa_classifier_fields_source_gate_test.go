package unit_test

// qa_classifier_fields_source_gate_test.go — one shape for every catalog
// classifier: after the findings lookup it either returns, or decides through
// the shared fallback helper running the type's own findings predicate. Reading
// a raw Fields entry a second way is the defect this batch removed from fifteen
// types, and it is a defect because the classifier and the predicate then hold
// the same fact twice and can disagree.
//
// The classifiers that still read a raw field are listed below by name. The
// gate errors both ways: an unlisted classifier that decides for itself is a
// new violation, and a listed one that has stopped is an entry to delete. The
// list can only shrink.

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// classifiersOffTheSharedFallback is the burn-down list: every catalog
// classifier that still picks a colour of its own after the findings lookup,
// with the reason "not yet on the shared fallback". Task w29 empties it and
// deletes this gate.
var classifiersOffTheSharedFallback = map[string]string{ //nolint:gochecknoglobals // test-only burn-down list
	"colorECSTask":      "not yet on the shared fallback",
	"colorEB":           "not yet on the shared fallback",
	"colorEBS":          "not yet on the shared fallback",
	"colorEKSCluster":   "not yet on the shared fallback",
	"colorEKSNodeGroup": "not yet on the shared fallback",
	"colorAthena":       "not yet on the shared fallback",
	"colorELB":          "not yet on the shared fallback",
	"colorVPC":          "not yet on the shared fallback",
	"colorSubnet":       "not yet on the shared fallback",
	"colorNAT":          "not yet on the shared fallback",
	"colorIGW":          "not yet on the shared fallback",
	"colorVPCE":         "not yet on the shared fallback",
	"colorTGW":          "not yet on the shared fallback",
	"colorENI":          "not yet on the shared fallback",
	"colorSecrets":      "not yet on the shared fallback",
	"colorKMS":          "not yet on the shared fallback",
	"colorCFN":          "not yet on the shared fallback: parses a phrase through cfnStackColor",
	// These two live in catalog_color_helpers.go rather than a catalog_<cat>.go
	// data file, which is why a sweep of the category files did not see them.
	"r53Color": "not yet on the shared fallback",
}

// sharedFallbackCalls are the two helpers a classifier is allowed to hand a
// raw field to. They take the type's own findings predicate, so the field is
// read once, by the predicate, and the classifier never interprets it. It also
// excludes colorAnyFindingOrHealthy from the scan, which has the classifier
// signature but is the shared machinery rather than a type's own.
var sharedFallbackCalls = map[string]bool{ //nolint:gochecknoglobals // test-only lookup
	"colorFromFindings":        true,
	"colorAnyFindingOrHealthy": true,
}

// decidesForItself reports whether fn picks a colour on its own after the
// findings lookup, rather than handing the type's predicate to the shared
// fallback and returning what it says.
//
// Reading a Fields entry is not the test. A compliant classifier reads several
// of them, because the predicate takes typed arguments and something has to
// convert the strings. What separates the two shapes is the return: a
// classifier that answers with the fallback's verdict cannot disagree with the
// predicate, and one that answers with a colour of its own can.
//
// Deciding is therefore any return, other than the guard's own, that is neither
// a shared fallback call nor the single unconditional colour a classifier with
// no signals at all returns.
func decidesForItself(fn *ast.FuncDecl) bool {
	guard := findingsGuard(fn)
	constants := map[string]bool{}
	decides := false

	ast.Inspect(fn.Body, func(n ast.Node) bool {
		if guard != nil && n == ast.Node(guard.Body) {
			// The guard's own "return c" is the findings lookup, not a decision.
			return false
		}
		ret, ok := n.(*ast.ReturnStmt)
		if !ok || len(ret.Results) != 1 {
			return true
		}
		switch e := ret.Results[0].(type) {
		case *ast.CallExpr:
			id, ok := e.Fun.(*ast.Ident)
			if !ok || !sharedFallbackCalls[id.Name] {
				decides = true
			}
		case *ast.SelectorExpr:
			if x, ok := e.X.(*ast.Ident); ok && x.Name == "domain" {
				constants[e.Sel.Name] = true
			} else {
				decides = true
			}
		default:
			// Returning a local means the colour was worked out above.
			decides = true
		}
		return true
	})

	return decides || len(constants) > 1
}

// findingsGuard returns the "if c, ok := colorFromAnyFinding(r); ok" statement
// every classifier opens with, or nil when it has none.
func findingsGuard(fn *ast.FuncDecl) *ast.IfStmt {
	for _, stmt := range fn.Body.List {
		ifStmt, ok := stmt.(*ast.IfStmt)
		if !ok || ifStmt.Init == nil {
			continue
		}
		assign, ok := ifStmt.Init.(*ast.AssignStmt)
		if !ok || len(assign.Rhs) != 1 {
			continue
		}
		call, ok := assign.Rhs[0].(*ast.CallExpr)
		if !ok {
			continue
		}
		if id, ok := call.Fun.(*ast.Ident); ok && id.Name == "colorFromAnyFinding" {
			return ifStmt
		}
	}
	return nil
}

// isClassifier reports whether fn has the catalog classifier signature,
// func(domain.Resource) domain.Color.
func isClassifier(fn *ast.FuncDecl) bool {
	if fn.Recv != nil || fn.Body == nil {
		return false
	}
	if fn.Type.Params == nil || len(fn.Type.Params.List) != 1 {
		return false
	}
	if fn.Type.Results == nil || len(fn.Type.Results.List) != 1 {
		return false
	}
	return exprIs(fn.Type.Params.List[0].Type, "domain", "Resource") &&
		exprIs(fn.Type.Results.List[0].Type, "domain", "Color")
}

func exprIs(e ast.Expr, pkg, name string) bool {
	sel, ok := e.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	id, ok := sel.X.(*ast.Ident)
	return ok && id.Name == pkg && sel.Sel.Name == name
}

// scanClassifiers returns every catalog classifier by name, mapped to whether
// it reads a raw Fields entry, plus the file it lives in.
func scanClassifiers(t *testing.T) (violates map[string]bool, where map[string]string) {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	repoRoot := filepath.Join(filepath.Dir(thisFile), "..", "..")
	files, err := filepath.Glob(filepath.Join(repoRoot, "core", "aws", "catalog_*.go"))
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	if len(files) == 0 {
		t.Fatal("no catalog_*.go files found — the gate would pass vacuously")
	}

	violates = map[string]bool{}
	where = map[string]string{}
	fset := token.NewFileSet()
	for _, f := range files {
		file, err := parser.ParseFile(fset, f, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", f, err)
		}
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || !isClassifier(fn) || sharedFallbackCalls[fn.Name.Name] {
				continue
			}
			violates[fn.Name.Name] = decidesForItself(fn)
			where[fn.Name.Name] = filepath.Base(f) + ":" + strconv.Itoa(fset.Position(fn.Pos()).Line)
		}
	}
	return violates, where
}

// TestClassifiersDecideThroughTheSharedFallback is the gate. It fails when an
// unlisted classifier reads a raw Fields entry, and when a listed one no longer
// does — the second direction is what stops the list outliving the debt.
// burnDownDiff compares a scan against the burn-down list. unlisted is a new
// violation, stale is an entry that has been paid off or names nothing.
func burnDownDiff(decides map[string]bool, allow map[string]string) (unlisted, stale []string) {
	for name, bad := range decides {
		_, listed := allow[name]
		switch {
		case bad && !listed:
			unlisted = append(unlisted, name)
		case !bad && listed:
			stale = append(stale, name)
		}
	}
	for name := range allow {
		if _, exists := decides[name]; !exists {
			stale = append(stale, name+" (no such classifier)")
		}
	}
	sort.Strings(unlisted)
	sort.Strings(stale)
	return unlisted, stale
}

// TestClassifiersDecideThroughTheSharedFallback is the gate. It fails when an
// unlisted classifier decides for itself, and when a listed one no longer does
// — the second direction is what stops the list outliving the debt.
func TestClassifiersDecideThroughTheSharedFallback(t *testing.T) {
	decides, where := scanClassifiers(t)
	if len(decides) < 40 {
		t.Fatalf("scanned only %d classifiers; the gate is not seeing the catalog", len(decides))
	}

	unlisted, stale := burnDownDiff(decides, classifiersOffTheSharedFallback)
	for i, name := range unlisted {
		unlisted[i] = name + " (" + where[name] + ")"
	}

	if len(unlisted) > 0 {
		t.Errorf("%d classifier(s) pick a colour of their own after the findings lookup and are not on the burn-down list:\n  %s\n\n"+
			"Hand the type's findings predicate to colorFromFindings and return that, so the classifier and the predicate cannot disagree.",
			len(unlisted), strings.Join(unlisted, "\n  "))
	}
	if len(stale) > 0 {
		t.Errorf("%d entr(y/ies) on the burn-down list now decide through the shared fallback — delete them:\n  %s",
			len(stale), strings.Join(stale, "\n  "))
	}
}

// ---------------------------------------------------------------------------
// The gate proven in both directions. A gate nobody has seen fail is a gate
// that might be asserting nothing, so the detector and the comparison are each
// run against input built to trip them.
// ---------------------------------------------------------------------------

const mutantClassifiers = `package aws

func compliantFallback(r domain.Resource) domain.Color {
	if c, ok := colorFromAnyFinding(r); ok {
		return c
	}
	count, _ := strconv.Atoi(r.Fields["count"])
	return colorFromFindings(somePredicate(r.Fields["state"], count))
}

func compliantNoSignals(r domain.Resource) domain.Color {
	if c, ok := colorFromAnyFinding(r); ok {
		return c
	}
	return domain.ColorHealthy
}

func decidingSwitch(r domain.Resource) domain.Color {
	if c, ok := colorFromAnyFinding(r); ok {
		return c
	}
	switch r.Fields["state"] {
	case "bad":
		return domain.ColorBroken
	}
	return domain.ColorHealthy
}

func decidingViaLocal(r domain.Resource) domain.Color {
	if c, ok := colorFromAnyFinding(r); ok {
		return c
	}
	base := domain.ColorHealthy
	if r.Fields["state"] == "bad" {
		base = domain.ColorBroken
	}
	return base
}

func decidingViaHelper(r domain.Resource) domain.Color {
	if c, ok := colorFromAnyFinding(r); ok {
		return c
	}
	return somePhraseParser(r.Fields["status"])
}
`

func TestClassifierGate_DetectorSeparatesTheTwoShapes(t *testing.T) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "mutants.go", mutantClassifiers, 0)
	if err != nil {
		t.Fatalf("parse mutants: %v", err)
	}

	want := map[string]bool{
		"compliantFallback":  false,
		"compliantNoSignals": false,
		"decidingSwitch":     true,
		"decidingViaLocal":   true,
		"decidingViaHelper":  true,
	}

	seen := map[string]bool{}
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || !isClassifier(fn) {
			continue
		}
		seen[fn.Name.Name] = true
		if got := decidesForItself(fn); got != want[fn.Name.Name] {
			t.Errorf("decidesForItself(%s) = %v, want %v", fn.Name.Name, got, want[fn.Name.Name])
		}
	}
	for name := range want {
		if !seen[name] {
			t.Errorf("%s was not recognised as a classifier, so the detector never judged it", name)
		}
	}
}

func TestClassifierGate_ErrorsInBothDirections(t *testing.T) {
	allow := map[string]string{"colorListed": "not yet on the shared fallback"}

	t.Run("unlisted violation is reported", func(t *testing.T) {
		unlisted, stale := burnDownDiff(map[string]bool{"colorNew": true, "colorListed": true}, allow)
		if len(unlisted) != 1 || unlisted[0] != "colorNew" {
			t.Errorf("unlisted = %v, want [colorNew]", unlisted)
		}
		if len(stale) != 0 {
			t.Errorf("stale = %v, want none", stale)
		}
	})

	t.Run("listed classifier that now passes is reported", func(t *testing.T) {
		unlisted, stale := burnDownDiff(map[string]bool{"colorListed": false}, allow)
		if len(stale) != 1 || stale[0] != "colorListed" {
			t.Errorf("stale = %v, want [colorListed]", stale)
		}
		if len(unlisted) != 0 {
			t.Errorf("unlisted = %v, want none", unlisted)
		}
	})

	t.Run("entry naming nothing is reported", func(t *testing.T) {
		_, stale := burnDownDiff(map[string]bool{"colorOther": false}, allow)
		if len(stale) != 1 || !strings.Contains(stale[0], "no such classifier") {
			t.Errorf("stale = %v, want the missing-name report", stale)
		}
	})

	t.Run("a paid-off list is silent", func(t *testing.T) {
		unlisted, stale := burnDownDiff(map[string]bool{"colorListed": true}, allow)
		if len(unlisted) != 0 || len(stale) != 0 {
			t.Errorf("unlisted = %v, stale = %v, want both empty", unlisted, stale)
		}
	})
}
