package unit_test

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"testing"

	"golang.org/x/tools/go/packages"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// serviceClientsField reports the field name when e selects an
// interface-typed field of ServiceClients: a client a session may not hold.
func serviceClientsField(info *types.Info, e ast.Expr) (string, bool) {
	sel, ok := e.(*ast.SelectorExpr)
	if !ok {
		return "", false
	}
	t := info.TypeOf(sel.X)
	if t == nil {
		return "", false
	}
	if ptr, isPtr := t.(*types.Pointer); isPtr {
		t = ptr.Elem()
	}
	named, ok := t.(*types.Named)
	if !ok || named.Obj().Name() != "ServiceClients" {
		return "", false
	}
	if !types.IsInterface(info.TypeOf(sel)) {
		return "", false
	}
	return sel.Sel.Name, true
}

// A session can hold a client set without a given service's client, so a
// ServiceClients field is nil until checked. Every call through one, and every
// hand-off of one to a function, sits where that field was checked against
// nil or asserted with the ok form: in the function itself, or in every
// function that calls it. serviceClient is the one read that does both. A
// list fetcher is covered by its type's FetcherClients, which the dispatch
// checks before the fetcher runs.
func TestServiceClientFieldsAreCheckedBeforeUse(t *testing.T) {
	type use struct {
		pos, field string
	}
	checked := map[string]map[string]bool{}
	uses := map[string][]use{}
	callers := map[string][]string{}
	fetchers := map[string]bool{}
	t570EachFunc(t, func(pkg *packages.Package, path string, fn *ast.FuncDecl) {
		info := pkg.TypesInfo
		name := fn.Name.Name
		if fn.Recv != nil {
			name = "method " + name
		}
		if t571ReturnsFetchResult(fn, info) {
			fetchers[name] = true
		}
		mine := map[string]bool{}
		checked[name] = mine
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			switch x := n.(type) {
			case *ast.BinaryExpr:
				if x.Op == token.EQL || x.Op == token.NEQ {
					for _, side := range []ast.Expr{x.X, x.Y} {
						if field, ok := serviceClientsField(info, side); ok {
							mine[field] = true
						}
					}
				}
			case *ast.TypeAssertExpr:
				if field, ok := serviceClientsField(info, x.X); ok {
					mine[field] = true
				}
			case *ast.CallExpr:
				if id, ok := x.Fun.(*ast.Ident); ok {
					callers[id.Name] = append(callers[id.Name], name)
				}
				var operands []ast.Expr
				if sel, isSel := x.Fun.(*ast.SelectorExpr); isSel {
					operands = append(operands, sel.X)
				}
				operands = append(operands, x.Args...)
				for _, o := range operands {
					if field, ok := serviceClientsField(info, o); ok {
						uses[name] = append(uses[name], use{filepath.Base(path) + ":" + strconv.Itoa(pkg.Fset.Position(o.Pos()).Line) + " " + name + " " + types.ExprString(o), field})
					}
				}
			}
			return true
		})
	})
	// covered(fn, field): fn checks field, is a fetcher, or has callers and
	// every one of them is covered.
	memo := map[string]bool{}
	var covered func(fn, field string, seen map[string]bool) bool
	covered = func(fn, field string, seen map[string]bool) bool {
		key := fn + "\x00" + field
		if v, ok := memo[key]; ok {
			return v
		}
		if checked[fn][field] || fetchers[fn] {
			return true
		}
		if seen[fn] || len(callers[fn]) == 0 {
			return false
		}
		seen[fn] = true
		all := true
		for _, c := range callers[fn] {
			if !covered(c, field, seen) {
				all = false
				break
			}
		}
		memo[key] = all
		return all
	}
	var found []string
	for fn, us := range uses {
		for _, u := range us {
			if !covered(fn, u.field, map[string]bool{}) {
				found = append(found, u.pos)
			}
		}
	}
	sort.Strings(found)
	for _, f := range found {
		t.Errorf("%s is used with no nil check on the way to it", f)
	}
}

// A read never made answers unknown whatever path its error takes: the one
// test of that is noCallMade, in related_fetch.go, and no other code in
// core/aws branches on the no-call errors itself.
func TestNoCallMadeHasOneOwner(t *testing.T) {
	wrapped := fmt.Errorf("reading redis member cluster: %w", domain.ErrClientMissing)
	if r := awsclient.ReadFailed("sg", wrapped); r.State() != domain.RelatedUnknown || r.Err() != nil {
		t.Errorf("ReadFailed(missing client) = state %v err %v, want unknown with no error", r.State(), r.Err())
	}
	var found []string
	t570EachFunc(t, func(pkg *packages.Package, path string, fn *ast.FuncDecl) {
		if filepath.Base(path) == "related_fetch.go" {
			return
		}
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			id, ok := n.(*ast.Ident)
			if ok && (id.Name == "errClientMissing" || id.Name == "errRawStructMissing") {
				if call, inCall := enclosingIsCall(fn.Body, id); inCall {
					found = append(found, filepath.Base(path)+":"+strconv.Itoa(pkg.Fset.Position(id.Pos()).Line)+" "+fn.Name.Name+" "+types.ExprString(call.Fun))
				}
			}
			return true
		})
	})
	sort.Strings(found)
	for _, f := range found {
		t.Errorf("%s tests a no-call error itself; route it through ReadFailed, unreadBy or rowReads.fail", f)
	}
}

// enclosingIsCall reports the errors.Is/errors.As call whose argument id is.
func enclosingIsCall(body *ast.BlockStmt, id *ast.Ident) (*ast.CallExpr, bool) {
	var hit *ast.CallExpr
	ast.Inspect(body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok || hit != nil {
			return hit == nil
		}
		if fun, isSel := call.Fun.(*ast.SelectorExpr); isSel && types.ExprString(fun) == "errors.Is" {
			for _, a := range call.Args {
				if a == ast.Expr(id) {
					hit = call
				}
			}
		}
		return true
	})
	return hit, hit != nil
}

// A list read answers no list only for a target type nothing can fetch, and a
// checker that ranges over that answer reads it as an empty list. Every
// target type a core/aws list read names is fetchable, so no checker is
// handed that answer: FetchRelatedTarget, relatedRowsByID, and every function
// that hands its own target parameter on to one.
func TestListReadTargetsHaveFetchers(t *testing.T) {
	readers := map[string]int{"FetchRelatedTarget": 3, "relatedRowsByID": 3}
	type decl struct {
		fn    *ast.FuncDecl
		param int
	}
	var decls []decl
	t570EachFunc(t, func(_ *packages.Package, _ string, fn *ast.FuncDecl) {
		i := 0
		for _, f := range fn.Type.Params.List {
			for _, n := range f.Names {
				if n.Name == "target" {
					decls = append(decls, decl{fn, i})
				}
				i++
			}
		}
	})
	for grew := true; grew; {
		grew = false
		for _, d := range decls {
			if _, known := readers[d.fn.Name.Name]; known {
				continue
			}
			ast.Inspect(d.fn.Body, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				id, ok := call.Fun.(*ast.Ident)
				if !ok {
					return true
				}
				pos, isReader := readers[id.Name]
				if !isReader || pos >= len(call.Args) {
					return true
				}
				if arg, isID := call.Args[pos].(*ast.Ident); isID && arg.Name == "target" {
					readers[d.fn.Name.Name] = d.param
					grew = true
				}
				return true
			})
		}
	}
	var found []string
	t570EachFunc(t, func(pkg *packages.Package, path string, fn *ast.FuncDecl) {
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			id, ok := call.Fun.(*ast.Ident)
			if !ok {
				return true
			}
			pos, isReader := readers[id.Name]
			if !isReader || pos >= len(call.Args) {
				return true
			}
			lit, ok := call.Args[pos].(*ast.BasicLit)
			if !ok {
				return true
			}
			target, _ := strconv.Unquote(lit.Value)
			if resource.GetPaginatedFetcher(target) == nil {
				found = append(found, filepath.Base(path)+":"+strconv.Itoa(pkg.Fset.Position(lit.Pos()).Line)+" "+id.Name+"("+target+")")
			}
			return true
		})
	})
	sort.Strings(found)
	for _, f := range found {
		t.Errorf("%s reads a list no fetcher answers", f)
	}
	if len(readers) < 5 {
		t.Errorf("found %d list readers, want the helpers that forward their target too: %v", len(readers), readers)
	}
}

// A reverse scan that reads each row of a target list with a call of its own
// runs on every detail open, so it reads at most relatedFanOutCap rows
// (docs/related-resources.md rule 7): the list goes through fanOut before the
// loop that records a per-row read with reads.fail.
func TestPerRowReverseScansAreCapped(t *testing.T) {
	listReaders := map[string]bool{"relatedResourcesFor": true, "FetchRelatedTarget": true, "relatedRowsByID": true, "relatedListIn": true}
	var found []string
	t570EachFunc(t, func(pkg *packages.Package, path string, fn *ast.FuncDecl) {
		lists := map[string]bool{}
		capped := map[string]bool{}
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			as, ok := n.(*ast.AssignStmt)
			if !ok || len(as.Rhs) != 1 {
				return true
			}
			call, ok := as.Rhs[0].(*ast.CallExpr)
			if !ok {
				return true
			}
			id, ok := call.Fun.(*ast.Ident)
			if !ok {
				return true
			}
			lhs, ok := as.Lhs[0].(*ast.Ident)
			if !ok {
				return true
			}
			switch {
			case listReaders[id.Name]:
				lists[lhs.Name] = true
			case id.Name == "fanOut":
				capped[lhs.Name] = true
			}
			return true
		})
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			rng, ok := n.(*ast.RangeStmt)
			if !ok {
				return true
			}
			x, ok := rng.X.(*ast.Ident)
			if !ok || !lists[x.Name] || capped[x.Name] {
				return true
			}
			perRow := false
			ast.Inspect(rng.Body, func(m ast.Node) bool {
				if call, ok := m.(*ast.CallExpr); ok {
					if sel, ok := call.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "fail" && types.ExprString(sel.X) == "reads" {
						perRow = true
					}
				}
				return !perRow
			})
			if perRow {
				found = append(found, filepath.Base(path)+":"+strconv.Itoa(pkg.Fset.Position(rng.Pos()).Line)+" "+fn.Name.Name+" ranges over "+x.Name)
			}
			return true
		})
	})
	sort.Strings(found)
	for _, f := range found {
		t.Errorf("%s with a call per row and no fanOut cap", f)
	}
}

// rule7Pairs reads the pivots docs/related-resources.md rule 7 allows a call
// per target row: the "source → target" pairs its bullets name before the
// colon.
func rule7Pairs(t *testing.T) map[string]bool {
	t.Helper()
	raw, err := os.ReadFile("../../docs/related-resources.md")
	if err != nil {
		t.Fatalf("reading the related-resources contract: %v", err)
	}
	doc := string(raw)
	start := strings.Index(doc, "7. **Call budget**")
	if start < 0 {
		t.Fatal("rule 7 not found in docs/related-resources.md")
	}
	end := strings.Index(doc[start:], "\n## ")
	if end < 0 {
		t.Fatal("rule 7 has no section after it in docs/related-resources.md")
	}
	pairs := map[string]bool{}
	pair := regexp.MustCompile("`([a-z0-9-]+)` → `([a-z0-9-]+)`")
	for line := range strings.SplitSeq(doc[start:start+end], "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "- `") {
			continue
		}
		head, _, _ := strings.Cut(trimmed, ":")
		for _, m := range pair.FindAllStringSubmatch(head, -1) {
			pairs[m[1]+" → "+m[2]] = true
		}
	}
	return pairs
}

// A checker that makes a call per row of its target type is one rule 7
// names, and every pivot rule 7 names is one whose checker does: fanOut, the
// cap every such scan goes through, is reached from exactly those checkers.
func TestRule7NamesEveryPerRowFanOut(t *testing.T) {
	direct := map[string]bool{}
	callers := map[string][]string{}
	t570EachFunc(t, func(_ *packages.Package, _ string, fn *ast.FuncDecl) {
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			if id, isID := call.Fun.(*ast.Ident); isID {
				if id.Name == "fanOut" {
					direct[fn.Name.Name] = true
				}
				callers[id.Name] = append(callers[id.Name], fn.Name.Name)
			}
			return true
		})
	})
	reaches := map[string]bool{}
	var mark func(string)
	mark = func(fn string) {
		if reaches[fn] {
			return
		}
		reaches[fn] = true
		for _, c := range callers[fn] {
			mark(c)
		}
	}
	for fn := range direct {
		mark(fn)
	}

	allowed := rule7Pairs(t)
	fanning := map[string]bool{}
	for _, td := range resource.AllResourceTypes() {
		for _, def := range td.Related {
			if def.Checker == nil {
				continue
			}
			name := runtime.FuncForPC(reflect.ValueOf(def.Checker).Pointer()).Name()
			name = name[strings.LastIndex(name, ".")+1:]
			if reaches[name] {
				fanning[td.ShortName+" → "+def.TargetType] = true
			}
		}
	}
	for pair := range fanning {
		if !allowed[pair] {
			t.Errorf("%s makes a call per target row, which docs/related-resources.md rule 7 does not name", pair)
		}
	}
	for pair := range allowed {
		if !fanning[pair] {
			t.Errorf("docs/related-resources.md rule 7 names %s, whose checker makes no call per target row", pair)
		}
	}
}
