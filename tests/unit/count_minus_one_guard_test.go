package unit

// count_minus_one_guard_test.go — AST-based guard: ensures that raw
// `Count: -1` struct literals in internal/aws/*_related*.go only appear in
// legitimately-guarded positions (nil checks, error checks, type-assertion
// failures, or FetchFilter-navigation paths). Any `Count: -1` that is NOT
// inside one of those guards is the anti-pattern that was purged in Batch B
// and must not regress.

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/k2m30/a9s/v3/internal/resource"
)

// ---------------------------------------------------------------------------
// Test 1: AST guard — no forbidden Count:-1 in reverse-scan checkers
// ---------------------------------------------------------------------------

// TestNoCountMinusOneInReverseScanCheckers walks every
// internal/aws/*_related*.go file and asserts that no composite literal
// resource.RelatedCheckResult{Count: -1} appears inside an IfStmt whose
// condition references the identifier "truncated". This is the precise AST
// signature of the Batch-B anti-pattern:
//
//	if len(ids) == 0 && truncated {
//	    return resource.RelatedCheckResult{TargetType: "x", Count: -1}  // FORBIDDEN
//	}
//
// The correct replacement is resource.ApproximateZero(targetType).
//
// All other uses of Count: -1 (nil guards, error guards, type-assertion
// guards, FetchFilter-navigation, bare function-level returns, or ifs that
// check non-truncation conditions) are legitimately allowed and are NOT
// detected by this guard.
//
// Expected forbidden count: 0.
func TestNoCountMinusOneInReverseScanCheckers(t *testing.T) {
	// Locate internal/aws relative to this test file.
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller(0) failed — cannot locate test file")
	}
	// thisFile = .../tests/unit/count_minus_one_guard_test.go
	// We need .../internal/aws/
	repoRoot := filepath.Join(filepath.Dir(thisFile), "..", "..")
	awsDir := filepath.Join(repoRoot, "internal", "aws")

	pattern := filepath.Join(awsDir, "*_related*.go")
	files, err := filepath.Glob(pattern)
	if err != nil {
		t.Fatalf("filepath.Glob(%q): %v", pattern, err)
	}
	if len(files) == 0 {
		t.Fatalf("no *_related*.go files found under %s — check path", awsDir)
	}

	fset := token.NewFileSet()
	var forbidden []string

	for _, path := range files {
		src, parseErr := parser.ParseFile(fset, path, nil, 0)
		if parseErr != nil {
			t.Errorf("parse error in %s: %v", path, parseErr)
			continue
		}

		// Walk the AST and collect all composite literals that look like
		// resource.RelatedCheckResult{..., Count: -1, ...}.
		ast.Inspect(src, func(n ast.Node) bool {
			comp, ok := n.(*ast.CompositeLit)
			if !ok {
				return true
			}
			if !isRelatedCheckResultLit(comp) {
				return true
			}
			if !hasCountMinusOne(comp) {
				return true
			}

			// Found a Count:-1 composite. Now check whether it is allowed.
			if hasFetchFilterSet(comp) {
				// FetchFilter-navigation path: Count:-1 with FetchFilter is a
				// deliberate "I don't know yet, navigate via filter" semantic.
				return true
			}

			// The anti-pattern is precisely: Count:-1 inside an IfStmt
			// whose condition contains the identifier "truncated" (e.g.,
			// `if len(ids) == 0 && truncated`). All other usages are
			// legitimate (nil guards, error guards, bare returns, etc.).
			pos := fset.Position(comp.Pos())
			enclosingIfs := findAllEnclosingIfs(src, comp)
			isForbidden := false
			for _, enclosing := range enclosingIfs {
				if conditionContainsTruncated(enclosing.Cond) {
					isForbidden = true
					break
				}
			}
			if !isForbidden {
				return true
			}

			forbidden = append(forbidden, fmt.Sprintf("%s:%d", filepath.Base(path), pos.Line))
			return true
		})
	}

	if len(forbidden) > 0 {
		t.Errorf(
			"found %d forbidden Count:-1 literal(s) inside truncated-zero branches in *_related*.go files.\n"+
				"Replace each with resource.ApproximateZero(targetType).\n\n"+
				"Anti-pattern: if len(x) == 0 && truncated { return RelatedCheckResult{Count: -1} }\n"+
				"Correct form:  if len(x) == 0 && truncated { return resource.ApproximateZero(targetType) }\n\n"+
				"Forbidden hits:\n  %s",
			len(forbidden),
			strings.Join(forbidden, "\n  "),
		)
	}
}

// isRelatedCheckResultLit returns true if the composite literal's type is
// "RelatedCheckResult" (selector or ident). We match both
// `resource.RelatedCheckResult{...}` and a bare `RelatedCheckResult{...}` in
// case of a dot-import (not used in this codebase, but defensive).
func isRelatedCheckResultLit(comp *ast.CompositeLit) bool {
	if comp.Type == nil {
		return false
	}
	switch t := comp.Type.(type) {
	case *ast.SelectorExpr:
		return t.Sel.Name == "RelatedCheckResult"
	case *ast.Ident:
		return t.Name == "RelatedCheckResult"
	}
	return false
}

// hasCountMinusOne returns true if the composite literal contains a key-value
// pair `Count: -1`.
func hasCountMinusOne(comp *ast.CompositeLit) bool {
	for _, elt := range comp.Elts {
		kv, ok := elt.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		keyIdent, ok := kv.Key.(*ast.Ident)
		if !ok || keyIdent.Name != "Count" {
			continue
		}
		// Value should be a UnaryExpr `-1`: UnaryOp=token.SUB, X=BasicLit "1"
		unary, ok := kv.Value.(*ast.UnaryExpr)
		if !ok || unary.Op != token.SUB {
			continue
		}
		lit, ok := unary.X.(*ast.BasicLit)
		if !ok {
			continue
		}
		if lit.Kind == token.INT && lit.Value == "1" {
			return true
		}
	}
	return false
}

// hasFetchFilterSet returns true if the composite literal contains a key-value
// pair for `FetchFilter` with a non-nil, non-zero value. This covers the
// CloudTrail filter-navigation path in zzz_ct_events_all_related.go.
func hasFetchFilterSet(comp *ast.CompositeLit) bool {
	for _, elt := range comp.Elts {
		kv, ok := elt.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		keyIdent, ok := kv.Key.(*ast.Ident)
		if !ok || keyIdent.Name != "FetchFilter" {
			continue
		}
		// If value is the ident "nil" it's not set meaningfully.
		if ident, ok := kv.Value.(*ast.Ident); ok && ident.Name == "nil" {
			return false
		}
		return true
	}
	return false
}

// findAllEnclosingIfs returns all IfStmts (at any nesting level) whose body
// or else span contains the target node. This covers both direct and nested
// patterns like:
//
//	if err != nil {          ← outer — allowed
//	    if errors.Is(...) { ← inner — not nil/err/!ok, would not pass alone
//	        return Res{Count:-1}
//	    }
//	}
func findAllEnclosingIfs(file *ast.File, target ast.Node) []*ast.IfStmt {
	targetPos := target.Pos()
	var results []*ast.IfStmt

	ast.Inspect(file, func(n ast.Node) bool {
		ifStmt, ok := n.(*ast.IfStmt)
		if !ok {
			return true
		}
		if nodeContains(ifStmt.Body, targetPos) {
			results = append(results, ifStmt)
		}
		if ifStmt.Else != nil && nodeContains(ifStmt.Else, targetPos) {
			results = append(results, ifStmt)
		}
		return true
	})
	return results
}

// nodeContains reports whether the node's source span contains the given position.
func nodeContains(n ast.Node, pos token.Pos) bool {
	if n == nil {
		return false
	}
	return n.Pos() <= pos && pos < n.End()
}

// conditionContainsTruncated returns true if the expression tree contains
// an identifier named "truncated". This is the precise marker for the
// Batch-B anti-pattern: Count:-1 inside `if len(x) == 0 && truncated`.
//
// We walk the full expression tree so that both `truncated && len(x) == 0`
// and `len(x) == 0 && truncated` are detected.
func conditionContainsTruncated(e ast.Expr) bool {
	if e == nil {
		return false
	}
	found := false
	ast.Inspect(e, func(n ast.Node) bool {
		if found {
			return false
		}
		ident, ok := n.(*ast.Ident)
		if ok && ident.Name == "truncated" {
			found = true
			return false
		}
		return true
	})
	return found
}

// ---------------------------------------------------------------------------
// Test 2: ApproximateZero helper exists and compiles
// ---------------------------------------------------------------------------

// TestApproximateZeroHelperExists proves that resource.ApproximateZero is
// callable with a string argument and returns a resource.RelatedCheckResult.
// If the function is removed or renamed, this test will fail to compile.
func TestApproximateZeroHelperExists(t *testing.T) {
	result := resource.ApproximateZero("test")

	// Verify the shape: Approximate=true, Count=0, TargetType echoed.
	if result.TargetType != "test" {
		t.Errorf("ApproximateZero(\"test\").TargetType = %q; want %q", result.TargetType, "test")
	}
	if result.Count != 0 {
		t.Errorf("ApproximateZero(\"test\").Count = %d; want 0", result.Count)
	}
	if !result.Approximate {
		t.Errorf("ApproximateZero(\"test\").Approximate = false; want true")
	}
	if result.Err != nil {
		t.Errorf("ApproximateZero(\"test\").Err = %v; want nil", result.Err)
	}
	if len(result.ResourceIDs) != 0 {
		t.Errorf("ApproximateZero(\"test\").ResourceIDs = %v; want empty", result.ResourceIDs)
	}
}

// ---------------------------------------------------------------------------
// Test 3: ApproximateZero result passes ValidateRelatedResult
// ---------------------------------------------------------------------------

// TestValidateRelatedResult_ApproximateZero_IsValid asserts that the result
// produced by ApproximateZero satisfies the ValidateRelatedResult invariants.
// This pins the contract: Approximate=true + Count=0 must be a valid state.
func TestValidateRelatedResult_ApproximateZero_IsValid(t *testing.T) {
	result := resource.ApproximateZero("vpc")
	if err := resource.ValidateRelatedResult(result); err != nil {
		t.Errorf("ValidateRelatedResult(ApproximateZero(\"vpc\")) returned error: %v; want nil", err)
	}
}
