package unit

import (
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"golang.org/x/tools/go/packages"
)

const pagingGuardAWSPkg = "github.com/k2m30/a9s/v3/core/aws"

// pagingTokenFields are the output fields AWS SDK v2 list/describe calls use
// to hand back the next page. An output carrying one of them is a
// paginated API, whatever the call site passes as a filter.
var pagingTokenFields = map[string]bool{
	"NextToken":             true,
	"Marker":                true,
	"NextMarker":            true,
	"NextPageToken":         true,
	"PaginationToken":       true,
	"Position":              true,
	"NextContinuationToken": true,
}

// pagingTokenField returns the pagination-token field of an AWS SDK output
// type, or "" when t is not one.
func pagingTokenField(t types.Type) string {
	if ptr, ok := t.(*types.Pointer); ok {
		t = ptr.Elem()
	}
	named, ok := t.(*types.Named)
	if !ok || named.Obj().Pkg() == nil || !strings.HasPrefix(named.Obj().Pkg().Path(), "github.com/aws/aws-sdk-go-v2/service/") {
		return ""
	}
	st, ok := named.Underlying().(*types.Struct)
	if !ok {
		return ""
	}
	for i := range st.NumFields() {
		if pagingTokenFields[st.Field(i).Name()] {
			return st.Field(i).Name()
		}
	}
	return ""
}

// isPageAllCall reports whether call invokes core/aws.PageAll (plain or with
// explicit type arguments).
func isPageAllCall(info *types.Info, call *ast.CallExpr) bool {
	fun := call.Fun
	switch f := fun.(type) {
	case *ast.IndexExpr:
		fun = f.X
	case *ast.IndexListExpr:
		fun = f.X
	}
	var id *ast.Ident
	switch f := fun.(type) {
	case *ast.Ident:
		id = f
	case *ast.SelectorExpr:
		id = f.Sel
	default:
		return false
	}
	obj, ok := info.Uses[id].(*types.Func)
	return ok && obj.Name() == "PageAll" && obj.Pkg() != nil && obj.Pkg().Path() == pagingGuardAWSPkg
}

// A related checker that reads one page of a paginated AWS API reports a
// partial answer as exact. Every call in core/aws/*_related*.go whose SDK
// output type carries a pagination token must sit inside a function literal
// handed to PageAll, which walks the pages and reports whether it stopped
// at the cap.
func TestRelatedCheckers_EveryPaginatedCallGoesThroughPageAll(t *testing.T) {
	cfg := &packages.Config{
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedSyntax | packages.NeedTypes | packages.NeedTypesInfo,
		Dir:  filepath.Join("..", ".."),
	}
	pkgs, err := packages.Load(cfg, pagingGuardAWSPkg)
	if err != nil {
		t.Fatalf("load %s: %v", pagingGuardAWSPkg, err)
	}
	if len(pkgs) != 1 {
		t.Fatalf("load %s: %d packages, want 1", pagingGuardAWSPkg, len(pkgs))
	}
	if len(pkgs[0].Errors) > 0 {
		t.Fatalf("load %s: %v", pagingGuardAWSPkg, pkgs[0].Errors)
	}
	pkg := pkgs[0]

	var violations []string
	scanned := 0
	for _, file := range pkg.Syntax {
		path := pkg.Fset.Position(file.Pos()).Filename
		if ok, _ := filepath.Match("*_related*.go", filepath.Base(path)); !ok || strings.HasSuffix(path, "_test.go") {
			continue
		}
		scanned++
		var stack []ast.Node
		ast.Inspect(file, func(n ast.Node) bool {
			if n == nil {
				stack = stack[:len(stack)-1]
				return true
			}
			stack = append(stack, n)
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			// Only the SDK method call itself is a read; a wrapper such as
			// RetryOnThrottle returning its output is the same call again.
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			if s, isSel := pkg.TypesInfo.Selections[sel]; !isSel || s.Kind() != types.MethodVal {
				return true
			}
			sig, ok := pkg.TypesInfo.TypeOf(call.Fun).(*types.Signature)
			if !ok || sig.Results().Len() == 0 {
				return true
			}
			field := pagingTokenField(sig.Results().At(0).Type())
			if field == "" {
				return true
			}
			for i := len(stack) - 2; i > 0; i-- {
				lit, isLit := stack[i].(*ast.FuncLit)
				if !isLit {
					continue
				}
				if parent, isCall := stack[i-1].(*ast.CallExpr); isCall && isPageAllCall(pkg.TypesInfo, parent) {
					for _, arg := range parent.Args {
						if arg == lit {
							return true
						}
					}
				}
			}
			pos := pkg.Fset.Position(call.Pos())
			violations = append(violations, filepath.Base(pos.Filename)+":"+strconv.Itoa(pos.Line)+" "+sel.Sel.Name+" (output has "+field+")")
			return true
		})
	}
	if scanned < 20 {
		t.Fatalf("scanned %d *_related*.go files, want the whole related-checker set", scanned)
	}
	sort.Strings(violations)
	for _, v := range violations {
		t.Errorf("paginated call read outside PageAll: %s", v)
	}
}

// A hand-rolled page loop bounded by PerParentPageCap, either a
// `for range PerParentPageCap` or a page counter compared against it, is a
// second copy of the walk PageAll owns; each copy decides separately whether
// hitting the cap is reported.
func TestNoHandRolledPerParentPageCapLoops(t *testing.T) {
	dir := filepath.Join("..", "..", "core", "aws")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read %s: %v", dir, err)
	}
	fset := token.NewFileSet()
	var violations []string
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") || name == "pager.go" {
			continue
		}
		file, err := parser.ParseFile(fset, filepath.Join(dir, name), nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		isCap := func(x ast.Expr) bool {
			id, ok := x.(*ast.Ident)
			return ok && id.Name == "PerParentPageCap"
		}
		ast.Inspect(file, func(n ast.Node) bool {
			switch n := n.(type) {
			case *ast.RangeStmt:
				if isCap(n.X) {
					violations = append(violations, name+":"+strconv.Itoa(fset.Position(n.Pos()).Line)+" for range PerParentPageCap")
				}
			case *ast.BinaryExpr:
				switch n.Op {
				case token.LSS, token.LEQ, token.GTR, token.GEQ, token.EQL:
					if isCap(n.X) || isCap(n.Y) {
						violations = append(violations, name+":"+strconv.Itoa(fset.Position(n.Pos()).Line)+" page counter compared with PerParentPageCap")
					}
				}
			}
			return true
		})
	}
	for _, v := range violations {
		t.Errorf("hand-rolled page loop: %s", v)
	}
}
