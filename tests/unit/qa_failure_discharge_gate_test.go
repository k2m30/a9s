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
	"sort"
	"strings"
	"testing"
)

// failureRecorders are the calls that put a Failure into a caller-owned list.
var failureRecorders = map[string]bool{
	"MarkSkipped":        true,
	"MarkUnusable":       true,
	"FailedCall":         true,
	"FailedCallInRegion": true,
	"FailedOnPage":       true,
	"UnusableAnswer":     true,
	// markAllUninspected was deleted in this task's round 1 when the ebs-snap
	// public-share walk moved onto walkAccountPages. Named here so that if it
	// ever comes back it is covered from the start.
	"markAllUninspected": true,
}

// failureDischargers build the composite error a caller returns.
var failureDischargers = map[string]bool{
	"AggregateFailures": true,
	"Finish":            true,
}

// failureDischargeExempt lists functions that record failures, do not
// discharge them, and do not carry them out through the signature either —
// each with the reason its caller can still see them.
//
// Key shape: "<file>:<func>".
var failureDischargeExempt = map[string]string{
	"iam_roles.go:enumerateRoleInlinePolicies": "returns inlinePolicyScan, whose failures field FetchIAMRolesPage and " +
		"roleToResource both aggregate; the carrier is a struct field, which this gate does not read types for",
}

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

// scanFailureCalls returns the recorder and discharger calls made directly in
// body, by name. Nested function literals count: an enricher's ForEachParallel
// closure is where most recording happens, and it shares the enclosing
// function's failure slice.
func scanFailureCalls(body *ast.BlockStmt) (records, discharges []string) {
	seen := map[string]bool{}
	ast.Inspect(body, func(n ast.Node) bool {
		call, isCall := n.(*ast.CallExpr)
		if !isCall {
			return true
		}
		name, isIdent := call.Fun.(*ast.Ident)
		if !isIdent || seen[name.Name] {
			return true
		}
		switch {
		case failureRecorders[name.Name]:
			seen[name.Name] = true
			records = append(records, name.Name)
		case failureDischargers[name.Name]:
			seen[name.Name] = true
			discharges = append(discharges, name.Name)
		}
		return true
	})
	sort.Strings(records)
	return records, discharges
}

// carriesFailuresInSignature reports whether the function takes a *[]Failure
// (its caller owns the list) or returns a Failure / []Failure (its caller
// receives them) — either way the discharging is demonstrably elsewhere.
func carriesFailuresInSignature(sig *ast.FuncType) bool {
	if sig.Params != nil {
		for _, p := range sig.Params.List {
			if star, isPtr := p.Type.(*ast.StarExpr); isPtr && isFailureSlice(star.X) {
				return true
			}
		}
	}
	if sig.Results != nil {
		for _, r := range sig.Results.List {
			if isFailureSlice(r.Type) || isFailureIdent(r.Type) {
				return true
			}
		}
	}
	return false
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
		records, discharges := scanFailureCalls(fn.Body)
		if len(records) == 0 {
			continue
		}
		recording++
		if exempt != nil && exempt(fileName+":"+fn.Name.Name) {
			continue
		}
		if len(discharges) > 0 || carriesFailuresInSignature(fn.Type) {
			continue
		}
		violations = append(violations, fmt.Sprintf(
			"%s:%d %s records %s and neither discharges it (AggregateFailures/Finish) "+
				"nor carries it out (a *[]Failure parameter, or a Failure result)",
			fileName, fset.Position(fn.Pos()).Line, fn.Name.Name,
			strings.Join(records, ", ")))
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
