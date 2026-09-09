package unit

// qa_failure_discharge_gate_test.go — a recorded failure has to leave the
// function that recorded it.
//
// The shape this catches: a function declares its own `var failures []Failure`,
// records into it with MarkSkipped/FailedCall/…, and then returns nothing, or
// nil, or an error built from something else. The slice is written and
// dropped, so a refused AWS call reaches the operator as silence — the row
// renders "?" with no cause anywhere, or renders clean.
//
// Three real instances of it were found by hand (EnrichIAMRoleLastUsed,
// EnrichIAMGroup, enrichDBIEngineVersions); this gate is what makes the fourth
// impossible to add quietly.
//
// The rule: a function whose body records a failure must ALSO either
//   - discharge it — call AggregateFailures or Finish, which builds the
//     composite error a caller returns; or
//   - hand it onward — take a *[]Failure parameter, or return a Failure /
//     []Failure, so the discharging is demonstrably its caller's job.
//
// It is structural on purpose. A rule that reads the returned expression
// would have to follow errors.Join across helpers, and the two shapes above
// are what every correct site in core/aws already uses.

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"sort"
	"strings"
	"testing"
)

// failureRecorders maps a recorder call to the 0-based index of the argument
// naming the list it writes into, or listByReturn when the call builds a
// Failure its caller appends itself.
//
// The index is what lets the gate follow the slice rather than read the
// function's shape: a call site hands one list and a second call site in the
// same function may hand another, and only the one each recorder actually
// wrote to has to leave.
var failureRecorders = map[string]int{
	"MarkSkipped":  2,
	"MarkUnusable": 2,
	// Covered by name so a helper of this shape is gated from the start.
	"markAllUninspected": 2,
	"FailedCall":         listByReturn,
	"FailedCallInRegion": listByReturn,
	"FailedOnPage":       listByReturn,
	"UnusableAnswer":     listByReturn,
}

// listByReturn marks a recorder that returns the Failure instead of taking the
// list: the destination is whatever the enclosing assignment appends to.
const listByReturn = -1

// failureDischargers build the composite error a caller returns.
var failureDischargers = map[string]bool{
	"AggregateFailures": true,
	"Finish":            true,
}

// failureDischargeExempt is empty and stays empty. The gate follows the slice
// each recorder writes to, so a function that carries its failures out on a
// struct field is read correctly rather than allowlisted; an entry here would
// be a function whose dropped list nobody has to account for.
//
// Key shape: "<file>:<func>".
var failureDischargeExempt = map[string]string{}

func TestFailureDischargeGate_ARecordedFailureLeavesItsFunction(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller(0) failed")
	}
	repoRoot := filepath.Dir(filepath.Dir(filepath.Dir(thisFile)))
	awsDir := filepath.Join(repoRoot, "core", "aws")

	fset := token.NewFileSet()
	var violations []string
	seenExempt := map[string]bool{}
	recording := 0

	err := filepath.WalkDir(awsDir, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		src, perr := parser.ParseFile(fset, path, nil, 0)
		if perr != nil {
			t.Fatalf("parse %s: %v", path, perr)
		}
		found, n := failureDischargeViolations(fset, filepath.Base(path), src, func(key string) bool {
			reason, exempt := failureDischargeExempt[key]
			if !exempt {
				return false
			}
			seenExempt[key] = true
			if reason == "" {
				t.Errorf("%s: exemption needs a reason", key)
			}
			return true
		})
		recording += n
		violations = append(violations, found...)
		return nil
	})
	if err != nil {
		t.Fatalf("walking %s: %v", awsDir, err)
	}

	if recording < 50 {
		t.Fatalf("only %d recording functions found under core/aws; the gate is not "+
			"looking at the tree it thinks it is", recording)
	}
	for key := range failureDischargeExempt {
		if !seenExempt[key] {
			t.Errorf("stale exemption %q: that function no longer records failures, "+
				"or was renamed — drop the entry", key)
		}
	}
	if len(violations) > 0 {
		sort.Strings(violations)
		t.Errorf("a recorded failure never leaves its function, so a refused AWS call "+
			"reaches the operator as silence:\n  %s", strings.Join(violations, "\n  "))
	}
}

// rootIdent names the variable an expression is rooted at: "failures" for
// failures, &failures, *failures and failures[0], "scan" for scan.failures.
// Empty when the expression is rooted at something that is not a variable (a
// call result, a literal), which is a destination the gate cannot follow and
// therefore does not accuse.
func rootIdent(e ast.Expr) string {
	switch v := e.(type) {
	case *ast.Ident:
		return v.Name
	case *ast.UnaryExpr:
		if v.Op == token.AND {
			return rootIdent(v.X)
		}
	case *ast.StarExpr:
		return rootIdent(v.X)
	case *ast.SelectorExpr:
		return rootIdent(v.X)
	case *ast.IndexExpr:
		return rootIdent(v.X)
	case *ast.ParenExpr:
		return rootIdent(v.X)
	}
	return ""
}

// failureScan is what one function's body says about its failure lists: the
// recorder names it called (for the message), which list each of them wrote
// to, and which of those lists demonstrably leaves the function.
type failureScan struct {
	records []string
	// destinations maps a list's root variable to the recorders that wrote to
	// it. A destination the gate could not follow is keyed "".
	destinations map[string][]string
	// carried names every list that leaves: a *[]Failure the caller owns, a
	// list handed to AggregateFailures/Finish, a value the function returns.
	carried map[string]bool
}

// scanFailureCalls reads body for recorder and discharger calls and follows
// the list each recorder writes to. Nested function literals count: an
// enricher's ForEachParallel closure is where most recording happens, and it
// shares the enclosing function's failure slice.
func scanFailureCalls(sig *ast.FuncType, body *ast.BlockStmt) failureScan {
	scan := failureScan{destinations: map[string][]string{}, carried: map[string]bool{}}
	seen := map[string]bool{}

	// A *[]Failure parameter is a list its caller owns and discharges.
	for _, p := range paramsOf(sig) {
		if star, isPtr := p.Type.(*ast.StarExpr); isPtr && isFailureSlice(star.X) {
			for _, name := range p.Names {
				scan.carried[name.Name] = true
			}
		}
	}

	note := func(name, dest string) {
		if !seen[name] {
			seen[name] = true
			scan.records = append(scan.records, name)
		}
		scan.destinations[dest] = append(scan.destinations[dest], name)
	}

	ast.Inspect(body, func(n ast.Node) bool {
		switch v := n.(type) {
		case *ast.FuncLit:
			// A closure may take its own *[]Failure; its body is walked by
			// this same Inspect, so record the parameter and carry on.
			for _, p := range paramsOf(v.Type) {
				if star, isPtr := p.Type.(*ast.StarExpr); isPtr && isFailureSlice(star.X) {
					for _, name := range p.Names {
						scan.carried[name.Name] = true
					}
				}
			}
		case *ast.ReturnStmt:
			for _, r := range v.Results {
				if root := rootIdent(r); root != "" {
					scan.carried[root] = true
				}
			}
		case *ast.AssignStmt:
			// A recorder that returns its Failure writes to whatever the
			// enclosing append assigns back to. Only the append shape counts:
			// a Failure bound to a plain variable, or one built inside a
			// closure the statement happens to assign, is not a write to a
			// list this statement names.
			if len(v.Lhs) != 1 || len(v.Rhs) != 1 {
				break
			}
			for _, name := range appendedRecorderNames(v.Rhs[0]) {
				note(name, rootIdent(v.Lhs[0]))
			}
		case *ast.CallExpr:
			name, isIdent := v.Fun.(*ast.Ident)
			if !isIdent {
				return true
			}
			if failureDischargers[name.Name] {
				for _, arg := range v.Args {
					if root := rootIdent(arg); root != "" {
						scan.carried[root] = true
					}
				}
				return true
			}
			idx, isRecorder := failureRecorders[name.Name]
			if !isRecorder || idx == listByReturn {
				return true
			}
			dest := ""
			if idx < len(v.Args) {
				dest = rootIdent(v.Args[idx])
			}
			note(name.Name, dest)
		}
		return true
	})
	sort.Strings(scan.records)
	return scan
}

// paramsOf returns sig's parameter fields, or nil when it declares none.
func paramsOf(sig *ast.FuncType) []*ast.Field {
	if sig == nil || sig.Params == nil {
		return nil
	}
	return sig.Params.List
}

// appendedRecorderNames lists the Failure-returning recorders e appends, when
// e is an append(...) call. Nothing else: a Failure read for one of its fields
// is not a write to any list, and a closure e assigns carries its own
// destinations, which the walk reaches on its own.
func appendedRecorderNames(e ast.Expr) []string {
	call, isCall := e.(*ast.CallExpr)
	if !isCall {
		return nil
	}
	if id, isIdent := call.Fun.(*ast.Ident); !isIdent || id.Name != "append" {
		return nil
	}
	var names []string
	for _, arg := range call.Args[1:] {
		inner, isInner := arg.(*ast.CallExpr)
		if !isInner {
			continue
		}
		if id, isIdent := inner.Fun.(*ast.Ident); isIdent && failureRecorders[id.Name] == listByReturn {
			names = append(names, id.Name)
		}
	}
	return names
}

func isFailureSlice(e ast.Expr) bool {
	arr, isArr := e.(*ast.ArrayType)
	return isArr && isFailureIdent(arr.Elt)
}

func isFailureIdent(e ast.Expr) bool {
	id, isIdent := e.(*ast.Ident)
	return isIdent && id.Name == "Failure"
}

// failureDischargeViolations reports every function in file that records a
// failure and neither discharges it nor hands it onward, and how many
// functions in the file record at all. exempt is asked once per recording
// function, keyed "<file>:<func>".
func failureDischargeViolations(fset *token.FileSet, fileName string, file *ast.File, exempt func(key string) bool) (violations []string, recording int) {
	for _, decl := range file.Decls {
		fn, isFunc := decl.(*ast.FuncDecl)
		if !isFunc || fn.Body == nil {
			continue
		}
		scan := scanFailureCalls(fn.Type, fn.Body)
		if len(scan.records) == 0 {
			continue
		}
		recording++
		if exempt != nil && exempt(fileName+":"+fn.Name.Name) {
			continue
		}
		var dropped []string
		for dest, recorders := range scan.destinations {
			// "" is a destination the gate could not follow to a variable; it
			// accuses nothing it cannot name.
			if dest == "" || scan.carried[dest] {
				continue
			}
			sort.Strings(recorders)
			dropped = append(dropped, fmt.Sprintf("%s (written by %s)", dest, strings.Join(slices.Compact(recorders), ", ")))
		}
		if len(dropped) == 0 {
			continue
		}
		sort.Strings(dropped)
		violations = append(violations, fmt.Sprintf(
			"%s:%d %s records into %s and neither discharges that list (AggregateFailures/Finish) "+
				"nor hands it on (a *[]Failure parameter, or returning it)",
			fileName, fset.Position(fn.Pos()).Line, fn.Name.Name,
			strings.Join(dropped, "; ")))
	}
	return violations, recording
}

// TestFailureDischargeGate_FollowsTheSliceEachRecorderWritesTo pins what the
// rule is actually about: the list a recorder writes into is the one that has
// to leave the function. Deciding it from the signature alone reads a shape,
// not a slice — a function that takes a *[]Failure for one walk and drops a
// local one for another satisfies the shape while a refused call in the second
// walk still reaches the operator as silence.
//
// Each probe below is a whole file, so the analyser sees exactly what it sees
// in core/aws.
func TestFailureDischargeGate_FollowsTheSliceEachRecorderWritesTo(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want []string
	}{
		{
			name: "a parameter list the caller owns",
			src: `package aws
func walk(out *[]Failure) {
	for _, id := range ids() {
		MarkSkipped(nil, id, out, errBoom)
	}
}`,
		},
		{
			name: "a local list the function discharges",
			src: `package aws
func walk() error {
	var failures []Failure
	for _, id := range ids() {
		MarkSkipped(nil, id, &failures, errBoom)
	}
	return AggregateFailures("walk", failures, 1)
}`,
		},
		{
			name: "a local list dropped beside a parameter the function also takes",
			src: `package aws
func walk(out *[]Failure) error {
	for _, id := range ids() {
		MarkSkipped(nil, id, out, errBoom)
	}
	var dropped []Failure
	for _, id := range more() {
		MarkSkipped(nil, id, &dropped, errBoom)
	}
	return nil
}`,
			want: []string{"walk"},
		},
		{
			name: "a local list dropped beside one the function returns",
			src: `package aws
func walk() []Failure {
	var carried []Failure
	MarkSkipped(nil, "a", &carried, errBoom)
	var dropped []Failure
	MarkUnusable(nil, "b", &dropped, "no name")
	return carried
}`,
			want: []string{"walk"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fset := token.NewFileSet()
			file, err := parser.ParseFile(fset, "probe.go", tc.src, 0)
			if err != nil {
				t.Fatalf("parse probe: %v", err)
			}
			got, recording := failureDischargeViolations(fset, "probe.go", file, nil)
			if recording != 1 {
				t.Fatalf("the probe has %d recording functions, want 1 — the analyser is not reading it", recording)
			}
			if len(got) != len(tc.want) {
				t.Fatalf("the gate reported %d violation(s) %v, want %d %v — it decides from the "+
					"function's shape instead of following the slice each recorder writes to",
					len(got), got, len(tc.want), tc.want)
			}
			for i, name := range tc.want {
				if !strings.Contains(got[i], name) {
					t.Errorf("violation %d is %q, want it to name %q", i, got[i], name)
				}
			}
		})
	}
}
