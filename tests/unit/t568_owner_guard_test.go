package unit_test

// The EventPattern evaluator (eb_pattern.go) is the one reader of an
// EventBridge event pattern, and the name matcher (predicates.go) the one
// place two names are compared by prefix, suffix or substring. The related
// checkers outside the *_related*.go files — the shared helpers every pivot
// family calls, and checkers filed with their fetcher — are held to the same
// rule as the ones inside them.

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// t568PatternReaders are the files allowed to name a rule's event pattern:
// the fetcher that stores it on the row, the catalog literal that declares
// the field, and the evaluator.
var t568PatternReaders = []string{"eb_rule.go", "catalog_messaging.go", "eb_pattern.go"}

// t568IsChecker reports whether fd has a related checker's signature:
// (context.Context, any, resource.Resource, resource.ResourceCache).
func t568IsChecker(fd *ast.FuncDecl) bool {
	var types []string
	for _, field := range fd.Type.Params.List {
		n := max(len(field.Names), 1)
		for range n {
			switch x := field.Type.(type) {
			case *ast.SelectorExpr:
				if pkg, ok := x.X.(*ast.Ident); ok {
					types = append(types, pkg.Name+"."+x.Sel.Name)
				}
			case *ast.Ident:
				types = append(types, x.Name)
			default:
				types = append(types, "?")
			}
		}
	}
	return slices.Equal(types, []string{"context.Context", "any", "resource.Resource", "resource.ResourceCache"})
}

func TestT568_EventPatternIsReadByTheEvaluatorOnly(t *testing.T) {
	files, err := filepath.Glob("../../core/aws/*.go")
	if err != nil || len(files) == 0 {
		t.Fatalf("no core/aws files: %v", err)
	}
	fset := token.NewFileSet()
	var hits []string
	for _, path := range files {
		base := filepath.Base(path)
		if strings.HasSuffix(base, "_test.go") || slices.Contains(t568PatternReaders, base) {
			continue
		}
		f, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		ast.Inspect(f, func(n ast.Node) bool {
			switch x := n.(type) {
			case *ast.SelectorExpr:
				if x.Sel.Name == "EventPattern" {
					hits = append(hits, base+":"+strconv.Itoa(fset.Position(x.Pos()).Line)+" reads Rule.EventPattern")
				}
			case *ast.BasicLit:
				if x.Kind == token.STRING && x.Value == `"event_pattern"` {
					hits = append(hits, base+":"+strconv.Itoa(fset.Position(x.Pos()).Line)+` reads Fields["event_pattern"]`)
				}
			}
			return true
		})
	}
	if len(hits) > 0 {
		t.Errorf("an event pattern read outside the evaluator:\n  %s", strings.Join(hits, "\n  "))
	}
}

func TestT568_CheckersEverywhereCompareNamesThroughTheMatcher(t *testing.T) {
	files, err := filepath.Glob("../../core/aws/*.go")
	if err != nil || len(files) == 0 {
		t.Fatalf("no core/aws files: %v", err)
	}
	fset := token.NewFileSet()
	var hits []string
	checkers := 0
	for _, path := range files {
		base := filepath.Base(path)
		if strings.HasSuffix(base, "_test.go") || strings.Contains(base, "_related") {
			continue
		}
		f, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		for _, decl := range f.Decls {
			fd, ok := decl.(*ast.FuncDecl)
			if !ok || fd.Body == nil || !(t568IsChecker(fd) || base == "related_common.go" || base == "related_shared.go") {
				continue
			}
			checkers++
			ast.Inspect(fd.Body, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				if what := t568NameCompare(call); what != "" {
					hits = append(hits, base+":"+strconv.Itoa(fset.Position(call.Pos()).Line)+" "+fd.Name.Name+" "+what)
				}
				return true
			})
		}
	}
	if checkers == 0 {
		t.Fatal("found no checker outside the *_related*.go files: the signature match no longer sees them")
	}
	if len(hits) > 0 {
		t.Errorf("%d name comparison(s) in a checker outside the name matcher:\n  %s", len(hits), strings.Join(hits, "\n  "))
	}
}
