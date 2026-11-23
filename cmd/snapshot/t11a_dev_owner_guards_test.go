package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"

	"golang.org/x/tools/go/packages"
)

const collectorPkg = "github.com/k2m30/a9s/v3/cmd/snapshot"

var paginationTokenNames = map[string]bool{
	"NextToken":             true,
	"Marker":                true,
	"NextMarker":            true,
	"NextPageToken":         true,
	"PaginationToken":       true,
	"Position":              true,
	"NextContinuationToken": true,
	"ContinuationToken":     true,
}

func collectorSourceFiles(t *testing.T) []string {
	t.Helper()
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, f := range files {
		if !strings.HasSuffix(f, "_test.go") {
			out = append(out, f)
		}
	}
	return out
}

// A pagination token is read and set in pageAll alone: a collector file that
// names one walks pages by hand, which is how a list gets read once.
func TestT11aDev_OnlyPageAllNamesAPaginationToken(t *testing.T) {
	fset := token.NewFileSet()
	var violations []string
	for _, f := range collectorSourceFiles(t) {
		if f == "pagination.go" {
			continue
		}
		file, err := parser.ParseFile(fset, f, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		ast.Inspect(file, func(n ast.Node) bool {
			id, ok := n.(*ast.Ident)
			if ok && paginationTokenNames[id.Name] {
				violations = append(violations, fset.Position(id.Pos()).String()+" "+id.Name)
			}
			return true
		})
	}
	for _, v := range violations {
		t.Errorf("pagination token named outside pageAll: %s", v)
	}
}

// Every SDK call whose output carries a pagination token is walked by pageAll
// (its bound is an argument of the call) or by the SDK's own paginator.
func TestT11aDev_EveryPaginatedCallIsWalkedToTheEnd(t *testing.T) {
	cfg := &packages.Config{
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedSyntax | packages.NeedTypes | packages.NeedTypesInfo,
	}
	pkgs, err := packages.Load(cfg, collectorPkg)
	if err != nil {
		t.Fatal(err)
	}
	if len(pkgs) != 1 || len(pkgs[0].Errors) > 0 {
		t.Fatalf("load %s: %d packages, errors %v", collectorPkg, len(pkgs), pkgs[0].Errors)
	}
	pkg := pkgs[0]

	var violations []string
	for _, file := range pkg.Syntax {
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
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			s, isSel := pkg.TypesInfo.Selections[sel]
			if !isSel || s.Kind() != types.MethodVal || isPaginator(s.Recv()) {
				return true
			}
			sig, ok := pkg.TypesInfo.TypeOf(call.Fun).(*types.Signature)
			if !ok || sig.Results().Len() == 0 || tokenOf(sig.Results().At(0).Type()) == "" {
				return true
			}
			for i := len(stack) - 2; i > 0; i-- {
				lit, isLit := stack[i].(*ast.FuncLit)
				if !isLit {
					continue
				}
				if parent, isCall := stack[i-1].(*ast.CallExpr); isCall && isPageAllCall(pkg.TypesInfo, parent) && slices.Contains(parent.Args, ast.Expr(lit)) {
					return true
				}
			}
			pos := pkg.Fset.Position(call.Pos())
			violations = append(violations, filepath.Base(pos.Filename)+":"+strconv.Itoa(pos.Line)+" "+sel.Sel.Name)
			return true
		})
	}
	for _, v := range violations {
		t.Errorf("paginated call read outside pageAll: %s", v)
	}
}

func isPaginator(t types.Type) bool {
	if p, ok := t.(*types.Pointer); ok {
		t = p.Elem()
	}
	n, ok := t.(*types.Named)
	return ok && strings.HasSuffix(n.Obj().Name(), "Paginator")
}

func tokenOf(t types.Type) string {
	if p, ok := t.(*types.Pointer); ok {
		t = p.Elem()
	}
	n, ok := t.(*types.Named)
	if !ok || n.Obj().Pkg() == nil || !strings.HasPrefix(n.Obj().Pkg().Path(), "github.com/aws/aws-sdk-go-v2/service/") {
		return ""
	}
	st, ok := n.Underlying().(*types.Struct)
	if !ok {
		return ""
	}
	for i := range st.NumFields() {
		if paginationTokenNames[st.Field(i).Name()] {
			return st.Field(i).Name()
		}
	}
	return ""
}

func isPageAllCall(info *types.Info, call *ast.CallExpr) bool {
	fun := call.Fun
	switch f := fun.(type) {
	case *ast.IndexExpr:
		fun = f.X
	case *ast.IndexListExpr:
		fun = f.X
	}
	id, ok := fun.(*ast.Ident)
	if !ok {
		return false
	}
	obj, ok := info.Uses[id].(*types.Func)
	return ok && obj.Name() == "pageAll" && obj.Pkg() != nil && obj.Pkg().Path() == collectorPkg
}

// Every pageAll call site resumes through a declared pair: an output member
// that names a further page and that resumePairs does not pair would end the
// walk after one page, or panic.
func TestT11aDev_EveryPageAllCallResolvesItsResumeMembers(t *testing.T) {
	pkg := loadCollector(t)
	byName := map[string][2]reflect.Type{}
	for in, out := range sdkOutputTypes() {
		byName[in.Elem().PkgPath()+"."+in.Elem().Name()] = [2]reflect.Type{in.Elem(), out.Elem()}
	}
	sites := 0
	for id, inst := range pkg.TypesInfo.Instances {
		obj, ok := pkg.TypesInfo.Uses[id].(*types.Func)
		if !ok || obj.Name() != "pageAll" || obj.Pkg() == nil || obj.Pkg().Path() != collectorPkg {
			continue
		}
		sites++
		named, ok := inst.TypeArgs.At(0).(*types.Named)
		pos := pkg.Fset.Position(id.Pos())
		if !ok {
			t.Errorf("%s: pageAll input %s is not a named SDK type", pos, inst.TypeArgs.At(0))
			continue
		}
		pair, ok := byName[named.Obj().Pkg().Path()+"."+named.Obj().Name()]
		if !ok {
			t.Errorf("%s: %s is not an operation input of the SDK clients the app builds", pos, named)
			continue
		}
		if resume, unresolved := resumeFields(pair[0], pair[1]); len(resume) == 0 || len(unresolved) > 0 {
			t.Errorf("%s: %s resumes through %v and leaves %v unpaired", pos, named.Obj().Name(), resume, unresolved)
		}
	}
	if sites == 0 {
		t.Fatal("found no pageAll call site")
	}
}

// The word for a listed item a describe answered nothing for is written by
// describeListed alone, by name or spelled out; a code is read off an error by
// the outcome owner alone.
func TestT11aDev_OnlyTheOwnerWritesNotFoundOrReadsACode(t *testing.T) {
	fset := token.NewFileSet()
	for _, f := range collectorSourceFiles(t) {
		if f == "sources.go" {
			continue
		}
		file, err := parser.ParseFile(fset, f, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		ast.Inspect(file, func(n ast.Node) bool {
			switch x := n.(type) {
			case *ast.BasicLit:
				if x.Kind == token.STRING && strings.Contains(x.Value, "not_found") {
					t.Errorf("%s writes the not_found word itself", fset.Position(x.Pos()))
				}
			case *ast.Ident:
				if x.Name == "outcomeNotFound" || x.Name == "apiErrorCode" {
					t.Errorf("%s names %s outside sources.go", fset.Position(x.Pos()), x.Name)
				}
			}
			return true
		})
	}
	if _, err := os.Stat("sources.go"); err != nil {
		t.Fatal(err)
	}
}

// The "error" outcome word is written by outcomeOf alone, so every failed read
// is recorded with the code AWS returned, in one spelling.
func TestT11aDev_OnlyTheOutcomeOwnerWritesTheErrorWord(t *testing.T) {
	fset := token.NewFileSet()
	for _, f := range collectorSourceFiles(t) {
		if f == "sources.go" {
			continue
		}
		file, err := parser.ParseFile(fset, f, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		ast.Inspect(file, func(n ast.Node) bool {
			if lit, ok := n.(*ast.BasicLit); ok && lit.Kind == token.STRING && lit.Value == `"error"` {
				t.Errorf("%s writes the error word itself; use outcomeOf", fset.Position(lit.Pos()))
			}
			return true
		})
	}
}

// readErrorOwners are the files that turn an error into what the snapshot
// records: the outcome words, the pages, and the report of a failed type.
var readErrorOwners = map[string]bool{"sources.go": true, "pagination.go": true, "main.go": true}

// An error a read returned reaches outcomeOf or readOf, or is returned by the
// function whose first read it is: the list a capture or a part is made of,
// whose failure fails it, or the describe a batch helper records. A later
// read's error that is returned fails the list it only adds to; one that is
// tested, logged or dropped is recorded as an empty answer. Errors of the
// standard library (URL and JSON parsing, files) are not reads of AWS.
func TestT11aDev_EveryReadErrorReachesTheOwnerOrIsTheFirstReturned(t *testing.T) {
	pkg := loadCollector(t)
	info := pkg.TypesInfo
	errType := types.Universe.Lookup("error").Type()
	calleeOf := func(call *ast.CallExpr) types.Object {
		fun := call.Fun
		switch f := fun.(type) {
		case *ast.IndexExpr:
			fun = f.X
		case *ast.IndexListExpr:
			fun = f.X
		}
		switch f := fun.(type) {
		case *ast.Ident:
			return info.Uses[f]
		case *ast.SelectorExpr:
			return info.Uses[f.Sel]
		}
		return nil
	}
	isRead := func(e ast.Expr) bool {
		call, ok := e.(*ast.CallExpr)
		if !ok {
			return false
		}
		obj := calleeOf(call)
		return obj != nil && obj.Pkg() != nil && strings.HasPrefix(obj.Pkg().Path(), "github.com/")
	}
	isOwnerCall := func(call *ast.CallExpr) bool {
		obj := calleeOf(call)
		return obj != nil && obj.Pkg() != nil && obj.Pkg().Path() == collectorPkg && (obj.Name() == "outcomeOf" || obj.Name() == "readOf")
	}
	errorObj := func(e ast.Expr) types.Object {
		id, ok := e.(*ast.Ident)
		if !ok {
			return nil
		}
		obj := info.ObjectOf(id)
		if obj == nil || !types.Identical(obj.Type(), errType) {
			return nil
		}
		return obj
	}

	type readAssign struct {
		obj   types.Object
		pos   token.Pos
		first bool
	}
	analyze := func(body *ast.BlockStmt) {
		var reads []readAssign
		assigned := map[types.Object][]token.Pos{}
		ast.Inspect(body, func(n ast.Node) bool {
			if _, nested := n.(*ast.FuncLit); nested {
				return false
			}
			as, ok := n.(*ast.AssignStmt)
			if !ok {
				return true
			}
			for _, lhs := range as.Lhs {
				obj := errorObj(lhs)
				if obj == nil {
					continue
				}
				assigned[obj] = append(assigned[obj], as.Pos())
				if len(as.Rhs) == 1 && isRead(as.Rhs[0]) {
					reads = append(reads, readAssign{obj: obj, pos: as.End(), first: len(reads) == 0})
				}
			}
			if len(as.Rhs) == 1 && isRead(as.Rhs[0]) {
				sig, ok := info.TypeOf(as.Rhs[0].(*ast.CallExpr).Fun).(*types.Signature)
				if ok && sig.Results().Len() == len(as.Lhs) {
					last := sig.Results().Len() - 1
					if id, isID := as.Lhs[last].(*ast.Ident); isID && id.Name == "_" && types.Identical(sig.Results().At(last).Type(), errType) {
						t.Errorf("%s discards a read's error", pkg.Fset.Position(as.Pos()))
					}
				}
			}
			return true
		})
		for _, r := range reads {
			end := body.End()
			for _, p := range assigned[r.obj] {
				if p > r.pos && p < end {
					end = p
				}
			}
			recorded, returned := false, false
			var stack []ast.Node
			ast.Inspect(body, func(n ast.Node) bool {
				if n == nil {
					stack = stack[:len(stack)-1]
					return true
				}
				stack = append(stack, n)
				id, ok := n.(*ast.Ident)
				if !ok || id.Pos() <= r.pos || id.Pos() >= end || info.Uses[id] != r.obj {
					return true
				}
			ancestors:
				for i := len(stack) - 2; i >= 0; i-- {
					switch anc := stack[i].(type) {
					case *ast.CallExpr:
						if isOwnerCall(anc) {
							recorded = true
							break ancestors
						}
					case *ast.ReturnStmt:
						if !slices.ContainsFunc(stack[:i], func(a ast.Node) bool { _, lit := a.(*ast.FuncLit); return lit }) {
							returned = true
						}
					case *ast.BinaryExpr:
						if anc.Op == token.EQL {
							t.Errorf("%s tests %s for nil; record the read with outcomeOf instead", pkg.Fset.Position(anc.Pos()), r.obj.Name())
						}
					}
				}
				return true
			})
			pos := pkg.Fset.Position(r.pos)
			switch {
			case returned && !r.first:
				t.Errorf("%s returns the error of a read that is not the function's first; record it with readOf", pos)
			case recorded, returned:
			default:
				t.Errorf("%s: %s reaches neither outcomeOf nor readOf, nor a return", pos, r.obj.Name())
			}
		}
	}
	for _, file := range pkg.Syntax {
		name := filepath.Base(pkg.Fset.Position(file.Pos()).Filename)
		if strings.HasSuffix(name, "_test.go") || readErrorOwners[name] {
			continue
		}
		ast.Inspect(file, func(n ast.Node) bool {
			switch fn := n.(type) {
			case *ast.FuncDecl:
				if fn.Body != nil {
					analyze(fn.Body)
				}
			case *ast.FuncLit:
				analyze(fn.Body)
			}
			return true
		})
	}
}

// A type listed from several calls is declared once, in the registry, through
// composed; a capture that settles sources by hand is a second way to do it.
func TestT11aDev_OnlyComposedSettlesSources(t *testing.T) {
	fset := token.NewFileSet()
	for _, f := range collectorSourceFiles(t) {
		if f == "sources.go" {
			continue
		}
		file, err := parser.ParseFile(fset, f, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		ast.Inspect(file, func(n ast.Node) bool {
			if id, ok := n.(*ast.Ident); ok && (id.Name == "runSources" || id.Name == "settleSources") {
				t.Errorf("%s uses %s outside composed", fset.Position(id.Pos()), id.Name)
			}
			return true
		})
	}
}
