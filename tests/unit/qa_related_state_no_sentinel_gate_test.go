// qa_related_state_no_sentinel_gate_test.go — task #58 conformance gate.
//
// The `Count == -1` sentinel on related-resource results/rows is being
// replaced by an explicit RelatedRowState enum. This gate pins the purge: no
// composite literal of a related result/row type — production OR test, under
// internal/ or tests/ — may set its Count (RelatedCheckResult /
// DetailRelatedRow / RelatedBlock) or count (rightColumnRow) field to a
// NEGATIVE integer literal. RED today (the checkers hand-roll `Count: -1`);
// GREEN once every producer routes through a state constructor
// (UnknownRelated / ErrorRelated / DeferredRelated / LoadingRelated) that
// sets the enum and leaves Count at its resolved zero value.
//
// Scope (Batch 2, task #58 follow-up): the scan walks BOTH internal/ (every
// .go file, including internal/**_test.go white-box tests) and tests/ (every
// .go file under tests/unit, tests/integration, tests/stories, tests/testdata
// — tests/e2e has no .go files). Test-side stub checkers and fake results are
// exactly as bound by this gate as production checkers: a test fixture that
// hand-rolls `RelatedCheckResult{Count: -1}` to simulate "unknown" reintroduces
// the retired sentinel encoding just as surely as a production checker would.
//
// DESIGN CHOICE (mirrors qa_multifinding_no_legacy_gate_test.go): no
// allowlist. A fresh AST scan every run, an unconditional t.Errorf listing
// every violation, GREEN only at zero. This file references no new production
// symbol, so it compiles clean against HEAD — "RED" here means "runs and
// fails via t.Errorf", never a compile break, so it carries zero blast radius
// for concurrent sibling work.
//
// AST shape matched: an *ast.CompositeLit whose type names one of the four
// related types — as a bare Ident (RelatedCheckResult{...}), a qualified
// SelectorExpr (resource.RelatedCheckResult{...}, domain....{...}), or the
// element type of a []T / [N]T / map[K]T literal whose inner element literals
// elide their own type ([]RelatedCheckResult{{Count: -1}}) — carrying a
// KeyValueExpr whose key ident is "Count" or "count" and whose value is a
// negative int literal (*ast.UnaryExpr Op=SUB wrapping an INT *ast.BasicLit).
// A comparison like `r.Count == -1` or `PaginationMeta.TotalHint == -1` is a
// BinaryExpr, never a CompositeLit element, so it can never false-positive.
// A prose mention inside a `//` comment or a string literal (e.g. an Errorf
// format string) is never part of the AST's CompositeLit walk either, so this
// file's own historical/documentation references to the retired sentinel
// cannot self-trip the gate — go/parser does not evaluate `//go:build` tags,
// so build-tag-gated files under tests/integration are parsed (and scanned)
// unconditionally too.
package unit_test

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// rsnsRelatedTypes is the set of related result/row type names whose
// composite literals may not carry a negative Count sentinel. Verified real
// on 2026-07-07: RelatedCheckResult (core/domain/contracts.go:122 and
// core/runtime/messages/event.go:112), DetailRelatedRow
// (core/app/screenstate.go:132), RelatedBlock (core/app/viewstate.go:226),
// rightColumnRow (internal/tui/views/rightcolumn.go:17, field spelled "count").
var rsnsRelatedTypes = map[string]bool{
	"RelatedCheckResult": true,
	"DetailRelatedRow":   true,
	"RelatedBlock":       true,
	"rightColumnRow":     true,
}

// rsnsTypeName returns the bare type name of a composite-literal type
// expression: "T" for an *ast.Ident, "T" for a "pkg.T" *ast.SelectorExpr, ""
// otherwise.
func rsnsTypeName(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.SelectorExpr:
		return t.Sel.Name
	}
	return ""
}

// rsnsElementTypeName returns the element type name of a []T / [N]T / map[K]T
// container type, so inner element literals that elide their own type (e.g.
// []RelatedCheckResult{{Count: -1}}) are still attributed to T.
func rsnsElementTypeName(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.ArrayType:
		return rsnsTypeName(t.Elt)
	case *ast.MapType:
		return rsnsTypeName(t.Value)
	}
	return ""
}

// rsnsIsNegativeIntLit reports whether expr is a negative integer literal —
// an *ast.UnaryExpr with token.SUB wrapping an INT *ast.BasicLit (`-1`, `-2`,
// …). Any negative magnitude is a sentinel; the codebase only uses -1.
func rsnsIsNegativeIntLit(expr ast.Expr) bool {
	u, ok := expr.(*ast.UnaryExpr)
	if !ok || u.Op != token.SUB {
		return false
	}
	lit, ok := u.X.(*ast.BasicLit)
	return ok && lit.Kind == token.INT
}

// rsnsViolation is one negative-Count sentinel literal.
type rsnsViolation struct {
	file     string
	line     int
	typeName string
}

func (v rsnsViolation) String() string {
	return fmt.Sprintf("%s:%d: %s{… Count: -1 …}", v.file, v.line, v.typeName)
}

// rsnsCheckElts records a violation for every KeyValueExpr in elts whose key
// is "Count"/"count" and whose value is a negative int literal.
func rsnsCheckElts(elts []ast.Expr, typeName, rel string, fset *token.FileSet, out *[]rsnsViolation) {
	for _, elt := range elts {
		kv, ok := elt.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		key, ok := kv.Key.(*ast.Ident)
		if !ok || (key.Name != "Count" && key.Name != "count") {
			continue
		}
		if !rsnsIsNegativeIntLit(kv.Value) {
			continue
		}
		pos := fset.Position(kv.Pos())
		*out = append(*out, rsnsViolation{file: rel, line: pos.Line, typeName: typeName})
	}
}

func rsnsScanFile(fset *token.FileSet, path, rel string) ([]rsnsViolation, error) {
	src, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		return nil, err
	}
	var violations []rsnsViolation
	ast.Inspect(src, func(n ast.Node) bool {
		cl, ok := n.(*ast.CompositeLit)
		if !ok {
			return true
		}
		// Direct: T{...} / pkg.T{...} where T names a related type.
		if name := rsnsTypeName(cl.Type); rsnsRelatedTypes[name] {
			rsnsCheckElts(cl.Elts, name, rel, fset, &violations)
			return true
		}
		// Container: []T{{…}} / map[K]T{k: {…}} where T is a related type —
		// the inner element literals elide their type, so attribute each
		// element's Count key to T here (the direct path above already covers
		// any element that DOES restate its type).
		if name := rsnsElementTypeName(cl.Type); rsnsRelatedTypes[name] {
			for _, elt := range cl.Elts {
				inner := elt
				if kv, ok := elt.(*ast.KeyValueExpr); ok {
					inner = kv.Value // map literal: the element is the value
				}
				if lit, ok := inner.(*ast.CompositeLit); ok && lit.Type == nil {
					rsnsCheckElts(lit.Elts, name, rel, fset, &violations)
				}
			}
		}
		return true
	})
	return violations, nil
}

// rsnsScanRoots are the two trees the gate walks: ALL of internal/ (including
// internal/**_test.go white-box tests — no "_test.go" exclusion, unlike the
// original production-only scan) and ALL of tests/ (unit, integration,
// stories, testdata; tests/e2e has no .go files to match). See this file's
// header for the Batch 2 scope rationale.
var rsnsScanRoots = []string{"../../core", "../../internal", "../../tests"}

// TestRelatedStateNoSentinel_NoNegativeCountLiteralInRelatedTypes is the
// task #58 gate: no .go file under internal/ or tests/ — production or test —
// may construct a related result/row literal with a negative Count sentinel.
// See this file's header for the exact AST shape matched and the
// no-allowlist rationale.
func TestRelatedStateNoSentinel_NoNegativeCountLiteralInRelatedTypes(t *testing.T) {
	fset := token.NewFileSet()
	var violations []rsnsViolation
	for _, r := range rsnsScanRoots {
		root, err := filepath.Abs(r)
		if err != nil {
			t.Fatalf("filepath.Abs(%q): %v", r, err)
		}
		rootLabel := filepath.Base(root)
		walkErr := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() {
				return nil
			}
			if !strings.HasSuffix(path, ".go") {
				return nil
			}
			rel, rerr := filepath.Rel(root, path)
			if rerr != nil {
				return rerr
			}
			rel = filepath.ToSlash(filepath.Join(rootLabel, rel))
			found, serr := rsnsScanFile(fset, path, rel)
			if serr != nil {
				return serr
			}
			violations = append(violations, found...)
			return nil
		})
		if walkErr != nil {
			t.Fatalf("walk %s failed: %v", r, walkErr)
		}
	}
	if len(violations) == 0 {
		return
	}
	lines := make([]string, len(violations))
	for i, v := range violations {
		lines[i] = v.String()
	}
	sort.Strings(lines)
	t.Errorf(
		"related-state sentinel gate — %d negative Count literal(s) in related result/row types "+
			"(RelatedCheckResult / DetailRelatedRow / RelatedBlock / rightColumnRow):\n%s\n\n"+
			"Replace each with an explicit RelatedRowState constructor (UnknownRelated / "+
			"ErrorRelated / DeferredRelated / LoadingRelated) that sets the state enum and leaves "+
			"Count at its resolved zero value; the `Count == -1` sentinel is retired in task #58.",
		len(violations), strings.Join(lines, "\n"),
	)
}
