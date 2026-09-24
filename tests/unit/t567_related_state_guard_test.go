package unit_test

// One rule decides a related row's state: relatedAnswer in
// core/aws/related_fetch.go, over the result constructors of core/domain and
// core/resource. A checker reports what it read and never builds a count, a
// lower bound or a proven zero itself, and it reads a target list through
// related_fetch.go, which knows what a disk-restored or degraded row can say.

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"io/fs"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"golang.org/x/tools/go/packages"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/resource"
)

const (
	t567RuleFile   = "core/aws/related_fetch.go"
	t567DomainPkg  = "github.com/k2m30/a9s/v3/core/domain"
	t567ResultPkg  = "github.com/k2m30/a9s/v3/core/resource"
	t567CachePkg   = "github.com/k2m30/a9s/v3/core/aws"
	t567SessionPkg = "github.com/k2m30/a9s/v3/core/session"
	t567EntryType  = "ResourceCacheEntry"
	t567PartialFix = "PartialScan"
)

// t567StateConstructors build a result whose state is a count, a lower bound,
// a proven zero, unknown or an error. A checker that built unknown or an error
// itself could drop the ids it had found.
var t567StateConstructors = map[string]bool{
	"ProvenZero": true, "KnownRelated": true, "HeuristicRelated": true, "UnknownRelated": true, "ErrorRelated": true,
}

// t567CacheReaders are the files outside the rule that read the session's
// cache directly, and why: each is an issue enricher, whose contract is to
// judge the rows the session has loaded and nothing more.
var t567CacheReaders = map[string]string{
	"backup_coverage.go":      "enricher: backup coverage of the loaded rows",
	"cf_issue_enrichment.go":  "enricher: origins among the loaded buckets",
	"ebs_issue_enrichment.go": "enricher: snapshots among the loaded ones",
	"ec2_issue_enrichment.go": "enricher: loaded security groups and route tables",
	"lt_issue_enrichment.go":  "enricher: images among the loaded ones",
	"r53_issue_enrichment.go": "enricher: alias targets among the loaded rows",
	"sg_issue_enrichment.go":  "enricher: network interfaces among the loaded ones",
	"snapshot_cross_ref.go":   "enricher: a snapshot's parent among the loaded rows",
}

// t567StateBuilders returns every place in the Go source src that names a
// state constructor of core/domain or core/resource, or marks a result a
// partial scan or attaches a failure to it.
func t567StateBuilders(t *testing.T, name string, src any) []string {
	t.Helper()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, name, src, 0)
	if err != nil {
		t.Fatal(err)
	}
	pkgOf := map[string]string{}
	for _, imp := range f.Imports {
		path, _ := strconv.Unquote(imp.Path.Value)
		local := path[strings.LastIndex(path, "/")+1:]
		if imp.Name != nil {
			local = imp.Name.Name
		}
		pkgOf[local] = path
	}
	var hits []string
	ast.Inspect(f, func(n ast.Node) bool {
		sel, ok := n.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		if sel.Sel.Name == t567PartialFix || sel.Sel.Name == "WithFailure" {
			hits = append(hits, fmt.Sprintf("%s: .%s", fset.Position(sel.Pos()), sel.Sel.Name))
			return true
		}
		x, ok := sel.X.(*ast.Ident)
		if !ok || !t567StateConstructors[sel.Sel.Name] {
			return true
		}
		if p := pkgOf[x.Name]; p == t567DomainPkg || p == t567ResultPkg {
			hits = append(hits, fmt.Sprintf("%s: %s.%s", fset.Position(sel.Pos()), x.Name, sel.Sel.Name))
		}
		return true
	})
	return hits
}

// TestRelatedStateGuard_OnlyTheRuleBuildsAState walks every production file
// and fails on a result state built outside the rule.
func TestRelatedStateGuard_OnlyTheRuleBuildsAState(t *testing.T) {
	root := filepath.Join("..", "..")
	var hits []string
	scanned := 0
	for _, dir := range []string{"core", "internal", "cmd"} {
		err := filepath.WalkDir(filepath.Join(root, dir), func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return err
			}
			rel, _ := filepath.Rel(root, path)
			rel = filepath.ToSlash(rel)
			if rel == t567RuleFile || strings.HasPrefix(rel, "core/domain/") || strings.HasPrefix(rel, "core/resource/") {
				return nil
			}
			scanned++
			hits = append(hits, t567StateBuilders(t, path, nil)...)
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	if scanned < 100 {
		t.Fatalf("scanned %d production files; the walk has stopped reaching the tree", scanned)
	}
	if len(hits) > 0 {
		sort.Strings(hits)
		t.Errorf("%d result(s) built outside %s — report what was read through relatedAnswer instead:\n  %s",
			len(hits), t567RuleFile, strings.Join(hits, "\n  "))
	}
}

// TestRelatedStateGuard_TheScanSeesAStateBuilt is the guard's own witness: a
// checker that builds its own zero, lower bound, candidates, unknown or error
// is caught under any import name.
func TestRelatedStateGuard_TheScanSeesAStateBuilt(t *testing.T) {
	src := `package aws

import (
	res "github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/domain"
)

func a() res.RelatedCheckResult { return res.ProvenZero("sg", "x") }
func b() res.RelatedCheckResult { return domain.KnownRelated("sg", nil, true) }
func c(r res.RelatedCheckResult) res.RelatedCheckResult { return r.PartialScan() }
var d = res.HeuristicRelated
func f(r res.RelatedCheckResult, err error) res.RelatedCheckResult { return r.WithFailure(err) }
func e() res.RelatedCheckResult { return res.UnknownRelated("sg") }
func g(err error) res.RelatedCheckResult { return domain.ErrorRelated("sg", err) }
`
	if hits := t567StateBuilders(t, "witness.go", src); len(hits) != 7 {
		t.Errorf("the scan found %d state builders in the witness, want 7: %v", len(hits), hits)
	}
}

// TestRelatedStateGuard_NoCheckerReadsTheCacheItself fails on an index or a
// range over the session's ResourceCache in core/aws outside the rule file
// and the enrichers listed above: a checker reads a target list through
// related_fetch.go, which treats a disk-restored entry as a miss.
func TestRelatedStateGuard_NoCheckerReadsTheCacheItself(t *testing.T) {
	cfg := &packages.Config{
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedSyntax | packages.NeedTypes | packages.NeedTypesInfo,
		Dir:  filepath.Join("..", ".."),
	}
	pkgs, err := packages.Load(cfg, t567CachePkg)
	if err != nil || len(pkgs) != 1 || len(pkgs[0].Errors) > 0 {
		t.Fatalf("load %s: %v %v", t567CachePkg, err, pkgs)
	}
	pkg := pkgs[0]
	isEntry := func(typ types.Type) bool {
		named, ok := types.Unalias(typ).(*types.Named)
		return ok && named.Obj().Name() == t567EntryType && named.Obj().Pkg() != nil && named.Obj().Pkg().Path() == t567DomainPkg
	}
	// A cache is any map of cache entries: the ResourceCache type, an alias,
	// or a conversion to the underlying map type.
	isCache := func(e ast.Expr) bool {
		m, ok := pkg.TypesInfo.TypeOf(e).Underlying().(*types.Map)
		return ok && isEntry(m.Elem())
	}

	var hits []string
	seen := map[string]bool{}
	reads := 0
	for _, file := range pkg.Syntax {
		path := pkg.Fset.Position(file.Pos()).Filename
		base := filepath.Base(path)
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		ast.Inspect(file, func(n ast.Node) bool {
			read := false
			switch v := n.(type) {
			case *ast.IndexExpr:
				read = isCache(v.X)
			case *ast.RangeStmt:
				// An iterator over the cache (maps.All, maps.Values) yields
				// its entries.
				read = isCache(v.X) || v.Value != nil && isEntry(pkg.TypesInfo.TypeOf(v.Value)) ||
					v.Key != nil && isEntry(pkg.TypesInfo.TypeOf(v.Key))
			}
			if !read {
				return true
			}
			reads++
			seen[base] = true
			if _, allowed := t567CacheReaders[base]; !allowed && !strings.HasSuffix(filepath.ToSlash(path), t567RuleFile) {
				hits = append(hits, pkg.Fset.Position(n.Pos()).String())
			}
			return true
		})
	}
	if reads == 0 {
		t.Fatal("the scan found no read of a ResourceCache at all — the type match has stopped reaching the code")
	}
	if len(hits) > 0 {
		sort.Strings(hits)
		t.Errorf("%d direct cache read(s) outside %s — read the list through FetchRelatedTarget, relatedRowsByID or cachedRelatedList:\n  %s",
			len(hits), t567RuleFile, strings.Join(hits, "\n  "))
	}
	var stale []string
	for f := range t567CacheReaders {
		if !seen[f] {
			stale = append(stale, f)
		}
	}
	if len(stale) > 0 {
		sort.Strings(stale)
		t.Errorf("allowlisted files that no longer read the cache — delete them from t567CacheReaders: %v", stale)
	}
}

// TestT567FetchIsPartial_ItemFailuresOnHeldRowsAreNotASubset: a failure
// whose row the answer still holds withheld that row's details, not the row;
// a failed page, an item the fetcher dropped and an error of any other shape
// may have cost a row.
func TestT567FetchIsPartial_ItemFailuresOnHeldRowsAreNotASubset(t *testing.T) {
	denied := t567Denied("eks:DescribeCluster")
	held := resource.FetchResult{Resources: []resource.Resource{{ID: "acme-prod"}, {ID: "acme-analytics"}}}
	item := awsclient.AggregateFailures("eks: DescribeCluster", []awsclient.Failure{awsclient.FailedCall("acme-analytics", denied)}, 2)
	dropped := awsclient.AggregateFailures("eks: DescribeCluster", []awsclient.Failure{awsclient.FailedCall("acme-batch", denied)}, 3)
	page := awsclient.AggregateFailures("eks: ListClusters", []awsclient.Failure{awsclient.FailedOnPage(2, denied)}, 2)
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"no error", nil, false},
		{"item failure on a held row", item, false},
		{"two item failures on held rows", awsclient.JoinAggregates(item, item), false},
		{"item failure on a dropped row", dropped, true},
		{"failed page", page, true},
		{"held item beside a failed page", awsclient.JoinAggregates(item, page), true},
		{"plain error", denied, true},
	}
	for _, tc := range cases {
		if got := awsclient.FetchIsPartial(held, tc.err); got != tc.want {
			t.Errorf("%s: FetchIsPartial = %v, want %v", tc.name, got, tc.want)
		}
	}
	truncated := held
	truncated.Pagination = &resource.PaginationMeta{IsTruncated: true}
	if !awsclient.FetchIsPartial(truncated, nil) {
		t.Error("a page that ended with a continuation token: FetchIsPartial = false, want true")
	}
}

// t567FetchWriters are the functions that store a fetch-origin or probe-origin
// row set without going through ObserveFetchResult, and why their pagination
// already carries FetchIsPartial's verdict.
var t567FetchWriters = map[string]string{
	"observeRelatedCheckResultRows": "a NeedsTargetCache page the executor stored with FetchIsPartial",
	"SetResourceCache":              "the PatchResourceCache write of that same page",
}

// TestRelatedStateGuard_EveryFetchWriterDecidesCompletenessOnce fails on a
// RowStore write of fetched or probed rows outside ObserveFetchResult: such a
// writer stores its own idea of whether the page is whole, and a page that
// lost rows beside an error reads complete from the cache. An origin is read
// by its type: a constant by its value, a function's own parameter makes that
// function a writer its callers are checked against, and any other value may
// be the fetch origin.
func TestRelatedStateGuard_EveryFetchWriterDecidesCompletenessOnce(t *testing.T) {
	cfg := &packages.Config{
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedSyntax | packages.NeedTypes | packages.NeedTypesInfo,
		Dir:  filepath.Join("..", ".."),
	}
	pkgs, err := packages.Load(cfg, "./core/...", "./internal/...", "./cmd/...")
	if err != nil {
		t.Fatal(err)
	}
	fetched := map[string]bool{}
	type fnDecl struct {
		pkg *packages.Package
		fn  *ast.FuncDecl
	}
	var decls []fnDecl
	for _, pkg := range pkgs {
		if len(pkg.Errors) > 0 {
			t.Fatalf("load %s: %v", pkg.PkgPath, pkg.Errors)
		}
		if pkg.PkgPath == t567SessionPkg {
			for _, name := range []string{"OriginFetch", "OriginProbe"} {
				fetched[pkg.Types.Scope().Lookup(name).(*types.Const).Val().ExactString()] = true
			}
			continue
		}
		for _, f := range pkg.Syntax {
			if strings.HasSuffix(pkg.Fset.Position(f.Pos()).Filename, "_test.go") {
				continue
			}
			for _, decl := range f.Decls {
				if fn, ok := decl.(*ast.FuncDecl); ok && fn.Body != nil {
					decls = append(decls, fnDecl{pkg, fn})
				}
			}
		}
	}
	if len(fetched) != 2 {
		t.Fatalf("found %d of the session's fetch and probe origins", len(fetched))
	}

	writers := map[string]bool{"ObserveRows": true, "Observe": true}
	var hits []string
	seen := map[string]bool{}
	routed := 0
	for grew := true; grew; {
		grew, hits, routed = false, nil, 0
		for _, d := range decls {
			info, name := d.pkg.TypesInfo, d.fn.Name.Name
			ast.Inspect(d.fn.Body, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				sel, ok := call.Fun.(*ast.SelectorExpr)
				var callee string
				switch {
				case ok:
					callee = sel.Sel.Name
				default:
					if id, isIdent := call.Fun.(*ast.Ident); isIdent {
						callee = id.Name
					}
				}
				if callee == "ObserveFetchResult" {
					routed++
				}
				if !writers[callee] || name == "ObserveFetchResult" {
					return true
				}
				for _, a := range call.Args {
					tv := info.Types[a]
					if !t567IsOrigin(tv.Type) {
						continue
					}
					if tv.Value != nil {
						if !fetched[tv.Value.ExactString()] {
							continue
						}
					} else if t567IsParam(info, d.fn, a) {
						if !writers[name] {
							writers[name], grew = true, true
						}
						continue
					}
					seen[name] = true
					if _, allowed := t567FetchWriters[name]; !allowed {
						hits = append(hits, fmt.Sprintf("%s: %s", d.pkg.Fset.Position(call.Pos()), name))
					}
				}
				return true
			})
		}
	}
	if routed == 0 {
		t.Fatal("no writer calls ObserveFetchResult — the scan has stopped reaching the write path")
	}
	if len(hits) > 0 {
		sort.Strings(hits)
		t.Errorf("%d fetched-row write(s) outside ObserveFetchResult — store the result and its error through it:\n  %s",
			len(hits), strings.Join(hits, "\n  "))
	}
	for name := range t567FetchWriters {
		if !seen[name] {
			t.Errorf("t567FetchWriters names %s, which no longer writes fetched rows — delete the entry", name)
		}
	}
}

// t567IsOrigin reports whether typ is the session's row origin.
func t567IsOrigin(typ types.Type) bool {
	named, ok := types.Unalias(typ).(*types.Named)
	return ok && named.Obj().Name() == "Origin" && named.Obj().Pkg() != nil && named.Obj().Pkg().Path() == t567SessionPkg
}

// t567IsParam reports whether e names one of fn's own parameters.
func t567IsParam(info *types.Info, fn *ast.FuncDecl, e ast.Expr) bool {
	id, ok := e.(*ast.Ident)
	if !ok {
		return false
	}
	obj := info.Uses[id]
	for _, field := range fn.Type.Params.List {
		for _, p := range field.Names {
			if obj != nil && info.Defs[p] == obj {
				return true
			}
		}
	}
	return false
}

// TestT567StoredPagination_ASubsetIsALowerBoundNoCursorResumes: a page whose
// rows are whole is stored as fetched; one that lost rows beside an error is
// stored as a lower bound that no cursor resumes, keeping its own fields.
func TestT567StoredPagination_ASubsetIsALowerBoundNoCursorResumes(t *testing.T) {
	denied := t567Denied("eks:ListNodegroups")
	rows := []resource.Resource{{ID: "acme-prod/general-pool"}}
	whole := resource.FetchResult{Resources: rows, Pagination: &resource.PaginationMeta{PageSize: 1, TotalHint: 1}}
	if got := awsclient.StoredPagination(whole, nil); got != whole.Pagination {
		t.Errorf("a whole page is stored as %+v, want its own pagination", got)
	}
	dropped := awsclient.AggregateFailures("ng", []awsclient.Failure{awsclient.FailedCall("acme-staging", denied)}, 2)
	got := awsclient.StoredPagination(whole, dropped)
	if got == nil || !got.IsTruncated || !got.LowerBoundOnly || got.NextToken != "" || got.PageSize != 1 || whole.Pagination.IsTruncated {
		t.Errorf("a page that lost a cluster's rows is stored as %+v, want a lower bound over a copy of its own pagination", got)
	}
	if got := awsclient.StoredPagination(resource.FetchResult{Resources: rows}, dropped); got == nil || !got.IsTruncated || !got.LowerBoundOnly {
		t.Errorf("a subset with no pagination is stored as %+v, want a lower bound", got)
	}
	cursor := resource.FetchResult{Resources: rows, Pagination: &resource.PaginationMeta{IsTruncated: true, NextToken: "t2"}}
	if got := awsclient.StoredPagination(cursor, dropped); got != cursor.Pagination {
		t.Errorf("a page with a cursor is stored as %+v, want its own pagination", got)
	}
}
