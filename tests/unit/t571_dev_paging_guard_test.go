package unit_test

import (
	"go/ast"
	"go/types"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"testing"

	"golang.org/x/tools/go/packages"
)

// t571PageTokenFields are the request fields an AWS list or describe call
// resumes from: an input carrying one is a paged API.
var t571PageTokenFields = []string{
	"Marker", "NextToken", "ExclusiveStartKey", "ExclusiveStartTagKey",
	"StartRecordName", "Position", "ContinuationToken", "PaginationToken",
}

// t571PagedInput reports whether t is a pointer to an AWS SDK request struct
// with a page-token field.
func t571PagedInput(t types.Type) bool {
	ptr, ok := types.Unalias(t).(*types.Pointer)
	if !ok {
		return false
	}
	named, ok := types.Unalias(ptr.Elem()).(*types.Named)
	if !ok || named.Obj().Pkg() == nil || !strings.Contains(named.Obj().Pkg().Path(), "aws-sdk-go-v2/service/") {
		return false
	}
	st, ok := named.Underlying().(*types.Struct)
	if !ok {
		return false
	}
	for i := range st.NumFields() {
		for _, name := range t571PageTokenFields {
			if st.Field(i).Name() == name {
				return true
			}
		}
	}
	return false
}

// t571ReturnsFetchResult reports whether fn is a list-page fetcher: it hands
// its cursor back to the list view through the FetchResult it returns.
func t571ReturnsFetchResult(fn *ast.FuncDecl, info *types.Info) bool {
	if fn.Type.Results == nil {
		return false
	}
	for _, r := range fn.Type.Results.List {
		if named, ok := types.Unalias(info.TypeOf(r.Type)).(*types.Named); ok && named.Obj().Name() == "FetchResult" {
			return true
		}
	}
	return false
}

// A paged list or describe read made outside PageAll stops at its first page
// or walks pages with no cap and no completeness reaching the answer. Every
// such read goes through PageAll, except a list fetcher that returns its
// cursor to the list view.
func TestT571_PagedReadsGoThroughPageAll(t *testing.T) {
	var found []string
	t570EachFunc(t, func(pkg *packages.Package, path string, fn *ast.FuncDecl) {
		info := pkg.TypesInfo
		if t571ReturnsFetchResult(fn, info) {
			return
		}
		var pageAllArgs []*ast.FuncLit
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			id, ok := call.Fun.(*ast.Ident)
			if ix, isIndex := call.Fun.(*ast.IndexListExpr); isIndex {
				id, ok = ix.X.(*ast.Ident)
			}
			if ok && (id.Name == "PageAll" || id.Name == "walkAccountPages" || id.Name == "firstPage") {
				for _, arg := range call.Args {
					if lit, isLit := arg.(*ast.FuncLit); isLit {
						pageAllArgs = append(pageAllArgs, lit)
					}
				}
			}
			return true
		})
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			if _, isMethod := call.Fun.(*ast.SelectorExpr); !isMethod {
				return true
			}
			var input ast.Expr
			for _, arg := range call.Args {
				if t571PagedInput(info.TypeOf(arg)) {
					input = arg
				}
			}
			if input == nil || t571CallerOwnsCursor(info, fn, input) || t571EC2ByIDs(info, fn, input) {
				return true
			}
			for _, lit := range pageAllArgs {
				if call.Pos() >= lit.Pos() && call.End() <= lit.End() {
					return true
				}
			}
			sel := call.Fun.(*ast.SelectorExpr)
			found = append(found, filepath.Base(path)+":"+strconv.Itoa(pkg.Fset.Position(call.Pos()).Line)+" "+fn.Name.Name+" "+sel.Sel.Name)
			return true
		})
	})
	sort.Strings(found)
	for _, f := range found {
		t.Errorf("%s reads a paged API outside PageAll", f)
	}
}

// t571CallerOwnsCursor reports whether the request's cursor is the caller's:
// the request is fn's own parameter, forwarded as given, or its page-token
// field is set from one of fn's parameters.
func t571CallerOwnsCursor(info *types.Info, fn *ast.FuncDecl, input ast.Expr) bool {
	params := map[types.Object]bool{}
	for _, f := range fn.Type.Params.List {
		for _, name := range f.Names {
			params[info.Defs[name]] = true
		}
	}
	// A local that starts as a parameter's value carries the caller's cursor.
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		if as, ok := n.(*ast.AssignStmt); ok && len(as.Lhs) == 1 && len(as.Rhs) == 1 {
			lhs, lok := as.Lhs[0].(*ast.Ident)
			rhs, rok := as.Rhs[0].(*ast.Ident)
			if lok && rok && params[info.Uses[rhs]] {
				params[info.Defs[lhs]] = true
			}
		}
		return true
	})
	// A cursor the function advances itself in a loop is a walk of its own,
	// not the caller's cursor, unless the function hands it back.
	if t571AdvancesInLoop(info, fn, params) && !t571ReturnsCursor(info, fn) {
		return false
	}
	if id, ok := input.(*ast.Ident); ok {
		if params[info.Uses[id]] {
			return true
		}
		// input.<token> = <from a parameter>
		owned := false
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			as, ok := n.(*ast.AssignStmt)
			if !ok || len(as.Lhs) != 1 || len(as.Rhs) != 1 {
				return true
			}
			sel, ok := as.Lhs[0].(*ast.SelectorExpr)
			if !ok || !slices.Contains(t571PageTokenFields, sel.Sel.Name) {
				return true
			}
			if x, ok := sel.X.(*ast.Ident); !ok || info.Uses[x] != info.Uses[id] {
				return true
			}
			ast.Inspect(as.Rhs[0], func(m ast.Node) bool {
				if p, ok := m.(*ast.Ident); ok && params[info.Uses[p]] {
					owned = true
				}
				return !owned
			})
			return !owned
		})
		return owned
	}
	lit, ok := input.(*ast.UnaryExpr)
	if !ok {
		return false
	}
	comp, ok := lit.X.(*ast.CompositeLit)
	if !ok {
		return false
	}
	for _, elt := range comp.Elts {
		kv, ok := elt.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		key, ok := kv.Key.(*ast.Ident)
		if !ok || !slices.Contains(t571PageTokenFields, key.Name) {
			continue
		}
		found := false
		ast.Inspect(kv.Value, func(n ast.Node) bool {
			if id, ok := n.(*ast.Ident); ok && params[info.Uses[id]] {
				found = true
			}
			return !found
		})
		if found {
			return true
		}
	}
	return false
}

// t571AdvancesInLoop reports whether a loop in fn assigns to one of vars.
func t571AdvancesInLoop(info *types.Info, fn *ast.FuncDecl, vars map[types.Object]bool) bool {
	advanced := false
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		var body *ast.BlockStmt
		switch loop := n.(type) {
		case *ast.ForStmt:
			body = loop.Body
		case *ast.RangeStmt:
			body = loop.Body
		default:
			return !advanced
		}
		ast.Inspect(body, func(m ast.Node) bool {
			as, ok := m.(*ast.AssignStmt)
			if !ok {
				return !advanced
			}
			for _, lhs := range as.Lhs {
				if id, ok := lhs.(*ast.Ident); ok && vars[info.Uses[id]] {
					advanced = true
				}
			}
			return !advanced
		})
		return !advanced
	})
	return advanced
}

// t571ReturnsCursor reports whether fn hands a cursor back to its caller: a
// string or *string result beside the items.
func t571ReturnsCursor(info *types.Info, fn *ast.FuncDecl) bool {
	sig, ok := info.Defs[fn.Name].Type().(*types.Signature)
	if !ok {
		return false
	}
	for v := range sig.Results().Variables() {
		t := types.Unalias(v.Type())
		if ptr, isPtr := t.(*types.Pointer); isPtr {
			t = ptr.Elem()
		}
		if b, isBasic := t.(*types.Basic); isBasic && b.Kind() == types.String {
			return true
		}
	}
	return false
}

// t571EC2ByIDs reports whether input is an EC2 describe request that names
// its resources by ID: such a call is unpaginated, and IDs with MaxResults
// fail with InvalidParameterCombination
// (https://docs.aws.amazon.com/AWSEC2/latest/APIReference/Query-Requests.html#api-pagination,
// https://docs.aws.amazon.com/ec2/latest/devguide/ec2-api-pagination.html).
func t571EC2ByIDs(info *types.Info, fn *ast.FuncDecl, input ast.Expr) bool {
	ptr, ok := types.Unalias(info.TypeOf(input)).(*types.Pointer)
	if !ok {
		return false
	}
	named, ok := types.Unalias(ptr.Elem()).(*types.Named)
	if !ok || !strings.HasSuffix(named.Obj().Pkg().Path(), "/service/ec2") {
		return false
	}
	var comp *ast.CompositeLit
	if u, ok := input.(*ast.UnaryExpr); ok {
		comp, _ = u.X.(*ast.CompositeLit)
	}
	if comp == nil {
		return false
	}
	for _, elt := range comp.Elts {
		if kv, ok := elt.(*ast.KeyValueExpr); ok {
			if key, ok := kv.Key.(*ast.Ident); ok && strings.HasSuffix(key.Name, "Ids") {
				return true
			}
		}
	}
	return false
}

// t571WalkCall reports whether e is a paged walk: PageAll, or a core/aws
// function that walks with PageAll and hands on its shape (items, complete,
// err).
func t571WalkCall(info *types.Info, walkers map[types.Object]bool, e ast.Expr) bool {
	if t571PageAllCall(e) {
		return true
	}
	call, ok := e.(*ast.CallExpr)
	if !ok {
		return false
	}
	id, ok := call.Fun.(*ast.Ident)
	if !ok || !walkers[info.Uses[id]] {
		return false
	}
	tuple, ok := info.TypeOf(e).(*types.Tuple)
	if !ok || tuple.Len() != 3 {
		return false
	}
	_, isSlice := tuple.At(0).Type().Underlying().(*types.Slice)
	return isSlice && types.Identical(tuple.At(1).Type(), types.Typ[types.Bool]) &&
		types.Identical(tuple.At(2).Type(), types.Universe.Lookup("error").Type())
}

// t571Walkers are the core/aws functions whose body calls PageAll.
func t571Walkers(pkg *packages.Package) map[types.Object]bool {
	out := map[types.Object]bool{}
	for _, f := range pkg.Syntax {
		for _, decl := range f.Decls {
			fd, ok := decl.(*ast.FuncDecl)
			if !ok || fd.Body == nil {
				continue
			}
			ast.Inspect(fd.Body, func(n ast.Node) bool {
				if call, ok := n.(ast.Expr); ok && t571PageAllCall(call) {
					out[pkg.TypesInfo.Defs[fd.Name]] = true
				}
				return true
			})
		}
	}
	return out
}

// t571PageAllCall reports whether e calls PageAll.
func t571PageAllCall(e ast.Expr) bool {
	call, ok := e.(*ast.CallExpr)
	if !ok {
		return false
	}
	fun := call.Fun
	if ix, isIndex := fun.(*ast.IndexListExpr); isIndex {
		fun = ix.X
	}
	if ix, isIndex := fun.(*ast.IndexExpr); isIndex {
		fun = ix.X
	}
	id, ok := fun.(*ast.Ident)
	return ok && id.Name == "PageAll"
}

// A related checker whose walk fails on page N has the items of pages
// 1..N-1 in hand: they are a lower bound beside the failure, never dropped
// for an error row. The walk's (complete, err) reach the answer through
// pagedRead, so no branch on the walk's error returns early.
func TestT571_RelatedWalkErrorKeepsTheItemsRead(t *testing.T) {
	var found []string
	var walkers map[types.Object]bool
	t570EachFunc(t, func(pkg *packages.Package, path string, fn *ast.FuncDecl) {
		if ok, _ := filepath.Match("*_related*.go", filepath.Base(path)); !ok {
			return
		}
		info := pkg.TypesInfo
		if walkers == nil {
			walkers = t571Walkers(pkg)
		}
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			block, ok := n.(*ast.BlockStmt)
			if !ok {
				return true
			}
			for i, st := range block.List {
				as, ok := st.(*ast.AssignStmt)
				if !ok || len(as.Lhs) != 3 || len(as.Rhs) != 1 || !t571WalkCall(info, walkers, as.Rhs[0]) || i+1 == len(block.List) {
					continue
				}
				errID, ok := as.Lhs[2].(*ast.Ident)
				if !ok {
					continue
				}
				errObj := info.Defs[errID]
				if errObj == nil {
					errObj = info.Uses[errID]
				}
				ifs, ok := block.List[i+1].(*ast.IfStmt)
				if !ok {
					continue
				}
				itemsID, _ := as.Lhs[0].(*ast.Ident)
				tests, weighsItems := false, false
				ast.Inspect(ifs.Cond, func(c ast.Node) bool {
					if id, ok := c.(*ast.Ident); ok {
						tests = tests || info.Uses[id] == errObj
						weighsItems = weighsItems || (itemsID != nil && id.Name == itemsID.Name)
					}
					return true
				})
				// A branch that also asks whether the walk found anything
				// returns only when it found nothing.
				if !tests || weighsItems {
					continue
				}
				for _, body := range ifs.Body.List {
					ret, isReturn := body.(*ast.ReturnStmt)
					if !isReturn {
						continue
					}
					keeps := false
					ast.Inspect(ret, func(r ast.Node) bool {
						if id, ok := r.(*ast.Ident); ok && itemsID != nil && id.Name == itemsID.Name && id.Name != "_" {
							keeps = true
						}
						return !keeps
					})
					if !keeps {
						found = append(found, filepath.Base(path)+":"+strconv.Itoa(pkg.Fset.Position(ifs.Pos()).Line)+" "+fn.Name.Name)
					}
				}
			}
			return true
		})
	})
	sort.Strings(found)
	for _, f := range found {
		t.Errorf("%s returns on a walk's error and drops the items it read; fold (complete, err) in with pagedRead", f)
	}
}

// PageAll, walkAccountPages and firstPage retry every call they make through
// RetryOnThrottle; a callback that retries again multiplies the attempts and
// wraps the error twice.
func TestT571_NoRetryInsideAPager(t *testing.T) {
	var found []string
	t570EachFunc(t, func(pkg *packages.Package, path string, fn *ast.FuncDecl) {
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			fun := call.Fun
			if ix, isIndex := fun.(*ast.IndexListExpr); isIndex {
				fun = ix.X
			}
			if ix, isIndex := fun.(*ast.IndexExpr); isIndex {
				fun = ix.X
			}
			id, ok := fun.(*ast.Ident)
			if !ok || (id.Name != "PageAll" && id.Name != "walkAccountPages" && id.Name != "firstPage") {
				return true
			}
			for _, arg := range call.Args {
				lit, ok := arg.(*ast.FuncLit)
				if !ok {
					continue
				}
				ast.Inspect(lit.Body, func(m ast.Node) bool {
					inner, ok := m.(*ast.CallExpr)
					if !ok {
						return true
					}
					fn2 := inner.Fun
					if ix, isIndex := fn2.(*ast.IndexExpr); isIndex {
						fn2 = ix.X
					}
					if rid, ok := fn2.(*ast.Ident); ok && rid.Name == "RetryOnThrottle" {
						found = append(found, filepath.Base(path)+":"+strconv.Itoa(pkg.Fset.Position(inner.Pos()).Line)+" "+fn.Name.Name)
					}
					return true
				})
			}
			return true
		})
	})
	sort.Strings(found)
	for _, f := range found {
		t.Errorf("%s retries inside a pager that already retries every call", f)
	}
}

// Every EC2 by-ID fetch goes through fetchEC2ByIDs, the one owner of the
// 1,000-ID batch and of dropping the IDs a NotFound error names: a fetch
// that describes EC2 resources by ID any other way fails one drill whole for
// one deleted ID.
func TestT571_EC2ByIDFetchesShareTheOwner(t *testing.T) {
	var found []string
	t570EachFunc(t, func(pkg *packages.Package, path string, fn *ast.FuncDecl) {
		if !strings.HasPrefix(fn.Name.Name, "Fetch") || !strings.HasSuffix(fn.Name.Name, "ByIDs") {
			return
		}
		describesEC2, owned := false, false
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			if id, ok := call.Fun.(*ast.Ident); ok && id.Name == "fetchEC2ByIDs" {
				owned = true
			}
			for _, arg := range call.Args {
				if ptr, ok := types.Unalias(pkg.TypesInfo.TypeOf(arg)).(*types.Pointer); ok {
					if named, ok := types.Unalias(ptr.Elem()).(*types.Named); ok && named.Obj().Pkg() != nil &&
						strings.HasSuffix(named.Obj().Pkg().Path(), "/service/ec2") {
						describesEC2 = true
					}
				}
			}
			return true
		})
		if describesEC2 && !owned {
			found = append(found, filepath.Base(path)+" "+fn.Name.Name)
		}
	})
	sort.Strings(found)
	for _, f := range found {
		t.Errorf("%s describes EC2 resources by ID outside fetchEC2ByIDs", f)
	}
}

// t571FirstPageCalls are the calls a single page answers, each for the reason
// its API reference gives: the request names one item or asks whether a page
// exists, or the service documents an order that puts the item wanted on the
// first page.
var t571FirstPageCalls = map[string]string{ //nolint:gochecknoglobals // test-only table
	"DescribeStackEvents":            "reverse chronological order",
	"DescribeStacks":                 "names one stack",
	"DescribeScalingActivities":      "most recent first",
	"GetLogEvents":                   "asks whether an earlier page exists",
	"DescribeLaunchTemplateVersions": "names one version",
	"DescribeDBEngineVersions":       "names one engine version",
	"ListExecutions":                 "most recent first",
	"DescribeLogStreams":             "ordered by LastEventTime, descending",
	"ListBuildsForProject":           "sortOrder DESCENDING",
	"GetJobRuns":                     "most recent first",
}

// A one-page read is a claim that the page holds the answer. An API with no
// documented order is walked with PageAll: its first page is any page.
func TestT571_FirstPageReadsHaveAnOrder(t *testing.T) {
	var found []string
	t570EachFunc(t, func(pkg *packages.Package, path string, fn *ast.FuncDecl) {
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			fun := call.Fun
			if ix, isIndex := fun.(*ast.IndexExpr); isIndex {
				fun = ix.X
			}
			if id, ok := fun.(*ast.Ident); !ok || id.Name != "firstPage" {
				return true
			}
			for _, arg := range call.Args {
				lit, ok := arg.(*ast.FuncLit)
				if !ok {
					continue
				}
				ast.Inspect(lit.Body, func(m ast.Node) bool {
					inner, ok := m.(*ast.CallExpr)
					if !ok {
						return true
					}
					sel, ok := inner.Fun.(*ast.SelectorExpr)
					if !ok || pkg.TypesInfo.Selections[sel] == nil {
						return true
					}
					if _, known := t571FirstPageCalls[sel.Sel.Name]; !known {
						found = append(found, filepath.Base(path)+":"+strconv.Itoa(pkg.Fset.Position(inner.Pos()).Line)+" "+sel.Sel.Name)
					}
					return true
				})
			}
			return true
		})
	})
	sort.Strings(found)
	for _, f := range found {
		t.Errorf("%s is read one page at a time with no documented order that puts the answer there", f)
	}
}

// r53ZoneRecords is the one walk over a zone's record sets; the only other
// caller of ListResourceRecordSets is a list fetcher that hands its cursor
// back to the caller.
func TestT571_OneWalkOverZoneRecords(t *testing.T) {
	var found []string
	t570EachFunc(t, func(pkg *packages.Package, path string, fn *ast.FuncDecl) {
		if fn.Name.Name == "r53ZoneRecords" || t571ReturnsFetchResult(fn, pkg.TypesInfo) {
			return
		}
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			if sel, ok := n.(*ast.SelectorExpr); ok && sel.Sel.Name == "ListResourceRecordSets" {
				found = append(found, filepath.Base(path)+":"+strconv.Itoa(pkg.Fset.Position(sel.Pos()).Line)+" "+fn.Name.Name)
			}
			return true
		})
	})
	for _, f := range found {
		t.Errorf("%s walks a zone's record sets outside r53ZoneRecords", f)
	}
}
