package unit_test

// t542_arn_hand_split_guard_test.go — an ARN is read through arn.Parse and the
// target type's resolver, never cut by hand. The guard walks every production
// file and fails on a string operation whose subject is an ARN value (an
// identifier or field key that says so), and on the six-field colon split
// that re-implements arn.Parse. What arn.Parse hands back (a.Resource) is the
// resolvers' own input and is not an ARN.

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

var (
	t542ARNIdent   = regexp.MustCompile(`^arn|Arn|ARN`)
	t542ARNLiteral = regexp.MustCompile(`(^|_)arn($|_)`)
	t542StringOps  = map[string]bool{
		"Split": true, "SplitN": true, "SplitAfter": true, "SplitAfterN": true,
		"Cut": true, "CutPrefix": true, "CutSuffix": true,
		"Index": true, "LastIndex": true, "IndexByte": true, "LastIndexByte": true,
		"IndexAny": true, "LastIndexAny": true, "TrimPrefix": true, "TrimSuffix": true,
		"HasSuffix": true, "Contains": true, "Fields": true,
	}
)

// t542NamesAnARN reports whether expr's subject is an ARN value.
func t542NamesAnARN(expr ast.Expr) bool {
	found := false
	ast.Inspect(expr, func(n ast.Node) bool {
		switch v := n.(type) {
		case *ast.Ident:
			found = found || t542ARNIdent.MatchString(v.Name)
		case *ast.BasicLit:
			found = found || v.Kind == token.STRING && t542ARNLiteral.MatchString(strings.Trim(v.Value, "\"`"))
		}
		return !found
	})
	return found
}

// t542HandSplits returns every hand ARN split in the Go source src.
func t542HandSplits(t *testing.T, name string, src any) []string {
	t.Helper()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, name, src, 0)
	if err != nil {
		t.Fatal(err)
	}
	var hits []string
	ast.Inspect(f, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok || len(call.Args) == 0 {
			return true
		}
		var fn string
		switch fun := call.Fun.(type) {
		case *ast.SelectorExpr:
			if pkg, ok := fun.X.(*ast.Ident); ok && pkg.Name == "strings" && t542StringOps[fun.Sel.Name] {
				fn = fun.Sel.Name
			}
		case *ast.Ident:
			if fun.Name == "lastSegment" {
				fn = fun.Name
			}
		}
		if fn == "" {
			return true
		}
		sixFields := fn == "SplitN" && len(call.Args) == 3 && t542Lit(call.Args[1]) == `":"` && t542Lit(call.Args[2]) == "6"
		if sixFields || t542NamesAnARN(call.Args[0]) {
			hits = append(hits, fmt.Sprintf("%s: %s(…)", fset.Position(call.Pos()), fn))
		}
		return true
	})
	return hits
}

func t542Lit(e ast.Expr) string {
	if lit, ok := e.(*ast.BasicLit); ok {
		return lit.Value
	}
	return ""
}

func TestNoProductionCodeCutsAnARNByHand(t *testing.T) {
	var hits []string
	for _, root := range []string{"core", "internal", "cmd"} {
		err := filepath.WalkDir(filepath.Join("..", "..", root), func(path string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return err
			}
			hits = append(hits, t542HandSplits(t, path, nil)...)
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	if len(hits) > 0 {
		t.Errorf("%d hand ARN cut(s); read the ARN through arn.Parse and the target type's resolver:\n  %s", len(hits), strings.Join(hits, "\n  "))
	}
}

// The guard itself: each shape a hand split has taken in this tree is caught,
// and what arn.Parse returns is not.
func TestNoProductionCodeCutsAnARNByHand_CatchesEachShape(t *testing.T) {
	cases := []struct {
		body string
		want bool
	}{
		{`parts := strings.Split(taskArn, "/")`, true},
		{`_, rest, _ := strings.Cut(r.Fields["environment_arn"], ":environment/")`, true},
		{`f := strings.SplitN(v, ":", 6)`, true},
		{`ok := strings.HasSuffix(*task.ClusterArn, "/"+name)`, true},
		{`ok := strings.Contains(arn, ":loadbalancer/")`, true},
		{`name := lastSegment(cmp.Or(aws.ToString(task.ClusterArn), cluster), "/")`, true},
		{`i := strings.LastIndex(resourceARN, "/")`, true},
		{`typ, _, _ := strings.Cut(a.Resource, "/")`, false},
		{`ok := strings.HasSuffix(warning, "/")`, false},
		{`parts := strings.Split(name, "/")`, false},
	}
	for _, tc := range cases {
		src := "package p\nfunc f() {\n" + tc.body + "\n}\n"
		if got := len(t542HandSplits(t, "probe.go", src)) > 0; got != tc.want {
			t.Errorf("%s: flagged = %v, want %v", tc.body, got, tc.want)
		}
	}
}
