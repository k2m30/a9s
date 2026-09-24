package unit_test

// One EventBridge pattern evaluator and one boundary-safe matcher own their
// kinds of join. A related checker hands a pattern and the resource to the
// evaluator and a pair of names to the matcher; a checker that tests one
// name against another with strings.Contains, HasPrefix or HasSuffix, or
// reads an event pattern itself, is a second reading of the same rule.

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
)

var t568PatternFunc = regexp.MustCompile(`(?i)ebrule|pattern`)

// t568NameCompare reports a strings.Contains, HasPrefix or HasSuffix call
// whose second argument is not a string literal: a test of one value against
// another, which is a name join. A test against a fixed literal ("ami-",
// ".dynamodb") reads a shape and is left to the resolvers' own guard.
func t568NameCompare(call *ast.CallExpr) string {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return ""
	}
	pkg, ok := sel.X.(*ast.Ident)
	if !ok || pkg.Name != "strings" || len(call.Args) != 2 {
		return ""
	}
	switch sel.Sel.Name {
	case "Contains", "HasPrefix", "HasSuffix":
	default:
		return ""
	}
	if lit, ok := call.Args[1].(*ast.BasicLit); ok && lit.Kind == token.STRING {
		return ""
	}
	return "strings." + sel.Sel.Name
}

// t568ReadsPattern reports whether fd reads an EventBridge event pattern: it
// names the rule's EventPattern (or the event_pattern field) or is named for
// one, and parses or searches text itself.
func t568ReadsPattern(fd *ast.FuncDecl) bool {
	names := t568PatternFunc.MatchString(fd.Name.Name)
	parses := false
	ast.Inspect(fd.Body, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.SelectorExpr:
			if x.Sel.Name == "EventPattern" {
				names = true
			}
			if pkg, ok := x.X.(*ast.Ident); ok && (pkg.Name == "json" && x.Sel.Name == "Unmarshal" || pkg.Name == "strings" && x.Sel.Name == "Contains") {
				parses = true
			}
		case *ast.BasicLit:
			if x.Kind == token.STRING && strings.Contains(x.Value, "event_pattern") {
				names = true
			}
		}
		return true
	})
	return names && parses
}

func TestRelatedCheckers_JoinThroughTheOwners(t *testing.T) {
	files, err := filepath.Glob("../../core/aws/*_related*.go")
	if err != nil || len(files) == 0 {
		t.Fatalf("no related checker files found: %v", err)
	}
	var hits []string
	fset := token.NewFileSet()
	for _, path := range files {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		for _, decl := range f.Decls {
			fd, ok := decl.(*ast.FuncDecl)
			if !ok || fd.Body == nil || strings.Contains(fd.Name.Name, "RefToID") {
				continue
			}
			if t568ReadsPattern(fd) {
				pos := fset.Position(fd.Pos())
				hits = append(hits, filepath.Base(pos.Filename)+":"+strconv.Itoa(pos.Line)+" "+fd.Name.Name+" reads an event pattern")
			}
			ast.Inspect(fd.Body, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				if what := t568NameCompare(call); what != "" {
					pos := fset.Position(call.Pos())
					hits = append(hits, filepath.Base(pos.Filename)+":"+strconv.Itoa(pos.Line)+" "+fd.Name.Name+" "+what)
				}
				return true
			})
		}
	}
	if len(hits) > 0 {
		sort.Strings(hits)
		t.Errorf("%d join(s) decided in a related checker instead of the pattern evaluator or the name matcher:\n  %s", len(hits), strings.Join(hits, "\n  "))
	}
}
