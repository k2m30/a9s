package unit_test

import (
	"fmt"
	"go/ast"
	"go/types"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"testing"

	"golang.org/x/tools/go/packages"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/catalog"
	"github.com/k2m30/a9s/v3/core/domain"
)

const (
	t12CatalogTypeDef = "github.com/k2m30/a9s/v3/core/catalog.ResourceTypeDef"
	t12FindingCode    = "github.com/k2m30/a9s/v3/core/domain.FindingCode"
)

// t12TypeArgFuncs are the core/aws and catalog functions whose first argument
// names the type whose findings they raise or judge on the row at hand.
var t12TypeArgFuncs = map[string]bool{ //nolint:gochecknoglobals // static function set
	"withState": true, "StateFinding": true,
	"appliesAny": true, "rowAppliesAny": true, "Applies": true, "AppliesToRow": true,
	"readPolicy": true, "DetailsDeniedCode": true, "DetailsUnavailableCode": true, "DegradedDetails": true,
}

// t12Graph is the static call graph of the module's production code, rooted
// at the functions each catalog literal's row fields and Wave 2 enricher
// name.
type t12Graph struct {
	pkgs    []*packages.Package
	roots   map[string]map[types.Object]bool
	fields  map[string][]ast.Expr
	callees map[types.Object]map[types.Object]bool
	decls   map[types.Object]*ast.FuncDecl
	info    map[types.Object]*types.Info
	reach   map[string]map[types.Object]bool
}

func t12LoadGraph(t *testing.T) *t12Graph {
	t.Helper()
	cfg := &packages.Config{Mode: packages.NeedName | packages.NeedFiles | packages.NeedSyntax | packages.NeedTypes | packages.NeedTypesInfo, Dir: filepath.Join("..", "..")}
	pkgs, err := loadPackagesOnce(cfg, "./core/...", "./internal/...", "./cmd/...")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	g := &t12Graph{
		pkgs:    pkgs,
		roots:   map[string]map[types.Object]bool{},
		fields:  map[string][]ast.Expr{},
		callees: map[types.Object]map[types.Object]bool{},
		decls:   map[types.Object]*ast.FuncDecl{},
		info:    map[types.Object]*types.Info{},
	}
	for _, p := range pkgs {
		for _, e := range p.Errors {
			t.Fatalf("%s: %v", p.PkgPath, e)
		}
		for _, f := range p.Syntax {
			for _, d := range f.Decls {
				fd, ok := d.(*ast.FuncDecl)
				if !ok || fd.Body == nil {
					continue
				}
				obj := p.TypesInfo.Defs[fd.Name]
				g.decls[obj] = fd
				g.info[obj] = p.TypesInfo
				g.callees[obj] = t12FuncsIn(p.TypesInfo, fd.Body)
			}
			ast.Inspect(f, func(n ast.Node) bool {
				lit, ok := n.(*ast.CompositeLit)
				if !ok || p.TypesInfo.TypeOf(lit) == nil || types.TypeString(p.TypesInfo.TypeOf(lit), nil) != t12CatalogTypeDef {
					return true
				}
				short := ""
				var values []ast.Expr
				for _, e := range lit.Elts {
					kv, isKV := e.(*ast.KeyValueExpr)
					if !isKV {
						continue
					}
					key, isID := kv.Key.(*ast.Ident)
					if !isID {
						continue
					}
					if s, isLit := kv.Value.(*ast.BasicLit); isLit && key.Name == "ShortName" {
						short, _ = strconv.Unquote(s.Value)
					}
					if t12RowFields[key.Name] || key.Name == "Wave2" {
						values = append(values, kv.Value)
					}
				}
				if short == "" {
					return true
				}
				if g.roots[short] == nil {
					g.roots[short] = map[types.Object]bool{}
				}
				for _, v := range values {
					g.fields[short] = append(g.fields[short], v)
					for fn := range t12FuncsIn(p.TypesInfo, v) {
						g.roots[short][fn] = true
					}
				}
				return true
			})
		}
	}
	g.reach = map[string]map[types.Object]bool{}
	for short, roots := range g.roots {
		seen := map[types.Object]bool{}
		var walk func(types.Object)
		walk = func(fn types.Object) {
			if seen[fn] {
				return
			}
			seen[fn] = true
			for c := range g.callees[fn] {
				walk(c)
			}
		}
		for fn := range roots {
			walk(fn)
		}
		g.reach[short] = seen
	}
	for _, td := range slices.Concat(catalog.All(), catalog.AllChildren()) {
		if _, ok := g.roots[td.ShortName]; !ok {
			t.Fatalf("no catalog literal found for %s; the scan lost the catalog", td.ShortName)
		}
	}
	return g
}

func t12FuncsIn(info *types.Info, n ast.Node) map[types.Object]bool {
	out := map[types.Object]bool{}
	ast.Inspect(n, func(m ast.Node) bool {
		if id, ok := m.(*ast.Ident); ok {
			if fn, isFn := info.Uses[id].(*types.Func); isFn {
				out[fn] = true
			}
		}
		return true
	})
	return out
}

type t12Body struct {
	name string
	info *types.Info
	node ast.Node
}

// bodies returns every syntax tree that runs for a row of short: the
// declarations its roots reach and its own field values.
func (g *t12Graph) bodies(short string) []t12Body {
	var out []t12Body
	for fn := range g.reach[short] {
		if fd := g.decls[fn]; fd != nil {
			out = append(out, t12Body{fn.Name(), g.info[fn], fd.Body})
		}
	}
	for _, v := range g.fields[short] {
		if info := g.infoOf(v); info != nil {
			out = append(out, t12Body{"", info, v})
		}
	}
	return out
}

// t12DegradedBuilders hand the cause their caller chose on to each other, so
// the cause a type can record is decided where one of them is called from
// elsewhere.
var t12DegradedBuilders = map[string]bool{"degradedRow": true, "markDegraded": true} //nolint:gochecknoglobals // static function set

func (g *t12Graph) infoOf(n ast.Node) *types.Info {
	for _, p := range g.pkgs {
		for _, f := range p.Syntax {
			if f.Pos() <= n.Pos() && n.End() <= f.End() {
				return p.TypesInfo
			}
		}
	}
	return nil
}

func (g *t12Graph) pos(n ast.Node) string {
	for _, p := range g.pkgs {
		for _, f := range p.Syntax {
			if f.Pos() <= n.Pos() && n.End() <= f.End() {
				at := p.Fset.Position(n.Pos())
				return fmt.Sprintf("%s:%d", filepath.Base(at.Filename), at.Line)
			}
		}
	}
	return "?"
}

// Every finding code the code that runs for a row of a type can raise is one
// that type declares, so neither emit door meets a code it has no rule for:
// a FindingCode constant named anywhere a type's row functions and Wave 2
// enricher reach, a degraded cause those functions can record, and every
// type named to a function that raises or judges a finding on the row.
func TestT12Dev_EveryCodeAType_sFunctionsReachIsDeclaredOnIt(t *testing.T) {
	g := t12LoadGraph(t)
	var found []string
	for short := range g.roots {
		for _, b := range g.bodies(short) {
			ast.Inspect(b.node, func(n ast.Node) bool {
				switch n := n.(type) {
				case *ast.Ident:
					c, ok := b.info.Uses[n].(*types.Const)
					if !ok || types.TypeString(c.Type(), nil) != t12FindingCode {
						return true
					}
					code, _ := strconv.Unquote(c.Val().ExactString())
					if !catalog.Declares(short, domain.FindingCode(code)) {
						found = append(found, fmt.Sprintf("%s: %s reaches %s, which %s does not declare", g.pos(n), short, code, short))
					}
				case *ast.CallExpr:
					name := t12CalleeName(b.info, n)
					if t12TypeArgFuncs[name] && len(n.Args) > 0 {
						if lit, isLit := n.Args[0].(*ast.BasicLit); isLit {
							if named, _ := strconv.Unquote(lit.Value); named != short {
								found = append(found, fmt.Sprintf("%s: a function of %s names %s to %s", g.pos(n), short, named, name))
							}
						}
					}
					if t12DegradedBuilders[name] && !t12DegradedBuilders[b.name] {
						need := []string{string(awsclient.DetailsUnavailableCode(short))}
						if id, isID := n.Args[len(n.Args)-1].(*ast.Ident); !isID || id.Name != "nil" {
							need = append(need, string(awsclient.DetailsDeniedCode(short)))
						}
						for _, code := range need {
							if !catalog.Declares(short, domain.FindingCode(code)) {
								found = append(found, fmt.Sprintf("%s: %s can record a degraded row, and declares no %s", g.pos(n), short, code))
							}
						}
					}
				}
				return true
			})
		}
	}
	slices.Sort(found)
	for _, f := range slices.Compact(found) {
		t.Error(f)
	}
}

func t12CalleeName(info *types.Info, call *ast.CallExpr) string {
	var id *ast.Ident
	switch fun := ast.Unparen(call.Fun).(type) {
	case *ast.Ident:
		id = fun
	case *ast.SelectorExpr:
		id = fun.Sel
	case *ast.IndexExpr:
		if x, ok := fun.X.(*ast.Ident); ok {
			id = x
		}
	}
	if id == nil {
		return ""
	}
	if _, ok := info.Uses[id].(*types.Func); !ok {
		return ""
	}
	return id.Name
}

// A row reaches a consumer only through the catalog fields Install binds,
// stamped with the type of the literal whose field produced it. A function
// that returns rows is used by production code only where every type whose
// row functions run that code is a type the rows belong to: a row builder no
// catalog field names is used only by code some type's row functions reach,
// and a function a type's field names is used only by that type's own code.
func TestT12Dev_RowsLeaveOnlyThroughTheirOwnType_sFields(t *testing.T) {
	g := t12LoadGraph(t)
	builds := g.rowBuilders()
	owners := map[types.Object]map[string]bool{}
	for short, roots := range g.roots {
		for fn := range roots {
			if builds[fn] {
				if owners[fn] == nil {
					owners[fn] = map[string]bool{}
				}
				owners[fn][short] = true
			}
		}
	}
	reachedBy := map[types.Object][]string{}
	for short, seen := range g.reach {
		for fn := range seen {
			reachedBy[fn] = append(reachedBy[fn], short)
		}
	}
	var found []string
	for caller, callees := range g.callees {
		if t12IsTestHook(g.info[caller], g.decls[caller]) {
			continue
		}
		for fn := range callees {
			f, ok := fn.(*types.Func)
			if !ok || f.Pkg() == nil || f.Pkg().Path() != t570AWSPkg || !builds[fn] || fn == caller || !t12ReturnsRows(f) {
				continue
			}
			shorts := reachedBy[caller]
			pos := g.pos(g.decls[caller])
			if len(shorts) == 0 {
				found = append(found, fmt.Sprintf("%s: %s builds rows of %s outside every type's row functions", pos, caller.Name(), f.Name()))
				continue
			}
			if own := owners[fn]; own != nil {
				for _, short := range shorts {
					if !own[short] {
						found = append(found, fmt.Sprintf("%s: %s, run for %s rows, uses %s, a row function of %s", pos, caller.Name(), short, f.Name(), t12Keys(own)))
					}
				}
			}
		}
	}
	for short, values := range g.fields {
		for _, v := range values {
			for fn := range t12FuncsIn(g.infoOf(v), v) {
				if own := owners[fn]; len(own) > 1 {
					found = append(found, fmt.Sprintf("%s: the row fields of %s name %s, which the fields of %s also name", g.pos(v), short, fn.Name(), t12Keys(own)))
				}
			}
		}
	}
	slices.Sort(found)
	for _, f := range slices.Compact(found) {
		t.Error(f)
	}
}

func (g *t12Graph) rowBuilders() map[types.Object]bool {
	builds := map[types.Object]bool{}
	for fn, fd := range g.decls {
		info := g.info[fn]
		ast.Inspect(fd.Body, func(n ast.Node) bool {
			if lit, ok := n.(*ast.CompositeLit); ok && len(lit.Elts) > 0 && info.TypeOf(lit) != nil && types.TypeString(types.Unalias(info.TypeOf(lit)), nil) == "github.com/k2m30/a9s/v3/core/domain.Resource" {
				builds[fn] = true
			}
			return !builds[fn]
		})
	}
	for grew := true; grew; {
		grew = false
		for fn, callees := range g.callees {
			if builds[fn] {
				continue
			}
			for c := range callees {
				if builds[c] {
					builds[fn] = true
					grew = true
					break
				}
			}
		}
	}
	return builds
}

func t12IsTestHook(info *types.Info, fd *ast.FuncDecl) bool {
	if fd == nil {
		return false
	}
	hook := false
	ast.Inspect(fd.Body, func(n ast.Node) bool {
		if call, ok := n.(*ast.CallExpr); ok {
			if sel, isSel := call.Fun.(*ast.SelectorExpr); isSel && sel.Sel.Name == "Testing" {
				if fn, isFn := info.Uses[sel.Sel].(*types.Func); isFn && fn.Pkg() != nil && fn.Pkg().Path() == "testing" {
					hook = true
				}
			}
		}
		return !hook
	})
	return hook
}

// A row's findings are stated once, by the findings: no core/aws code writes
// a row field from them (domain.StatusPhrase, domain.TopFinding, or a
// finding's Phrase or Code). Such a field is written before the row is bound
// and judged, so it can state a finding the row does not carry, or miss one
// it does.
func TestT12Dev_NoRowFieldIsWrittenFromTheRowsFindings(t *testing.T) {
	g := t12LoadGraph(t)
	var found []string
	for fn, fd := range g.decls {
		f, ok := fn.(*types.Func)
		if !ok || f.Pkg() == nil || f.Pkg().Path() != t570AWSPkg {
			continue
		}
		info := g.info[fn]
		ast.PreorderStack(fd.Body, nil, func(n ast.Node, stack []ast.Node) bool {
			if !t12ReadsFindingWords(info, n) {
				return true
			}
			for i := len(stack) - 1; i >= 0; i-- {
				if t12IsFieldWrite(info, stack[i]) {
					found = append(found, fmt.Sprintf("%s: %s writes a row field from its findings", g.pos(n), fn.Name()))
					break
				}
			}
			return true
		})
	}
	slices.Sort(found)
	for _, f := range found {
		t.Error(f)
	}
}

func t12ReadsFindingWords(info *types.Info, n ast.Node) bool {
	switch n := n.(type) {
	case *ast.CallExpr:
		sel, ok := n.Fun.(*ast.SelectorExpr)
		if !ok || (sel.Sel.Name != "StatusPhrase" && sel.Sel.Name != "TopFinding") {
			return false
		}
		fn, ok := info.Uses[sel.Sel].(*types.Func)
		return ok && fn.Pkg() != nil && fn.Pkg().Path() == "github.com/k2m30/a9s/v3/core/domain"
	case *ast.SelectorExpr:
		if n.Sel.Name != "Phrase" && n.Sel.Name != "Code" {
			return false
		}
		tp := info.TypeOf(n.X)
		return tp != nil && types.TypeString(types.Unalias(tp), nil) == "github.com/k2m30/a9s/v3/core/domain.Finding"
	}
	return false
}

func t12IsFieldWrite(info *types.Info, n ast.Node) bool {
	isFields := func(e ast.Expr) bool {
		tp := info.TypeOf(e)
		return tp != nil && types.TypeString(tp.Underlying(), nil) == "map[string]string"
	}
	switch n := n.(type) {
	case *ast.CompositeLit:
		return isFields(n)
	case *ast.AssignStmt:
		for _, l := range n.Lhs {
			if idx, ok := l.(*ast.IndexExpr); ok && isFields(idx.X) {
				return true
			}
		}
	}
	return false
}

func t12Keys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
