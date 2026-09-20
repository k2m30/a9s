package unit_test

// catalog_conformance_test.go — what a catalog entry declares for a type
// (its Fields keys, its column and detail paths, its navigable fields) is held
// against what the fetcher actually writes and what the default views render.
//
// Each declaration has a consumer that trusts it without checking: the view
// loader rejects a column keyed by a Fields key the type does not declare, a
// column path the SDK struct does not have renders blank on every row, and a
// navigable field whose path no detail row shows is a jump the operator can
// never take. None of these fail loudly at run time, so the comparison lives
// here, over the demo account, where every type is fetched the way the app
// fetches it.

import (
	"context"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"reflect"
	goruntime "runtime"
	"sort"
	"strconv"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/catalog"
	"github.com/k2m30/a9s/v3/core/config"
	"github.com/k2m30/a9s/v3/core/demo"
	"github.com/k2m30/a9s/v3/core/resource"
)

// conformanceRepoRoot is the repository root, found from this file's path so
// the test reads the committed sources regardless of the working directory.
func conformanceRepoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := goruntime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller(0) failed")
	}
	return filepath.Join(filepath.Dir(file), "..", "..")
}

// allCatalogTypes is every parent and every child type: children declare
// FieldKeys, Navigable and Related the same way parents do.
func allCatalogTypes(t *testing.T) []catalog.ResourceTypeDef {
	t.Helper()
	all := append(append([]catalog.ResourceTypeDef{}, catalog.All()...), catalog.AllChildren()...)
	if len(all) < 80 {
		t.Fatalf("catalog holds %d types — aws.Install() has not run", len(all))
	}
	return all
}

// demoWrittenKeys returns, per type, the Fields keys written over the demo
// account by the fetcher (parents drained to exhaustion, children through the
// parent context navigation builds) and, separately, the keys the type's
// Wave-2 enricher writes through FieldUpdates.
func demoWrittenKeys(t *testing.T) (fetched, enriched map[string]map[string]bool) {
	t.Helper()
	clients := demo.NewServiceClients()
	byType, cache := buildVisibilityTypeCache(t)
	rows := demoRowsIncludingChildren(t, clients, byType)

	fetched = make(map[string]map[string]bool)
	enriched = make(map[string]map[string]bool)
	for shortName, rs := range rows {
		set := make(map[string]bool)
		for _, r := range rs {
			for k := range r.Fields {
				set[k] = true
			}
		}
		fetched[shortName] = set

		enricher, ok := awsclient.Wave2EnricherFor(shortName)
		if !ok || enricher.Fn == nil || len(rs) == 0 {
			continue
		}
		result, err := enricher.Fn(context.Background(), clients, rs, cache)
		if err != nil {
			t.Fatalf("%s: Wave-2 enricher on the demo account: %v", shortName, err)
		}
		eset := make(map[string]bool)
		for _, updates := range result.FieldUpdates {
			for k := range updates {
				eset[k] = true
			}
		}
		enriched[shortName] = eset
	}
	return fetched, enriched
}

func conformanceKeySet(lists ...[]string) map[string]bool {
	out := make(map[string]bool)
	for _, l := range lists {
		for _, k := range l {
			out[k] = true
		}
	}
	return out
}

func conformanceMinus(a, b map[string]bool) []string {
	var out []string
	for k := range a {
		if !b[k] {
			out = append(out, k)
		}
	}
	sort.Strings(out)
	return out
}

func conformanceUnion(a, b map[string]bool) map[string]bool {
	out := make(map[string]bool, len(a)+len(b))
	for k := range a {
		out[k] = true
	}
	for k := range b {
		out[k] = true
	}
	return out
}

// TestCatalogFieldKeysEqualWhatTheDemoFetchWrites holds each type's declared
// FieldKeys plus IssueEnricherFieldKeys equal to the keys its fetcher and its
// Wave-2 enricher write over the demo account.
//
// A written key missing from the declaration is a key the view loader refuses:
// an operator who configures a column by it is told the column has no producer
// while every row carries the value. A declared key nothing writes is a column
// the loader accepts and that renders blank.
func TestCatalogFieldKeysEqualWhatTheDemoFetchWrites(t *testing.T) {
	fetched, enriched := demoWrittenKeys(t)

	for _, td := range allCatalogTypes(t) {
		if len(td.FieldKeys) == 0 && len(td.IssueEnricherFieldKeys) == 0 && fetched[td.ShortName] == nil {
			continue
		}
		written := conformanceUnion(fetched[td.ShortName], enriched[td.ShortName])
		declared := conformanceKeySet(td.FieldKeys, td.IssueEnricherFieldKeys)

		if undeclared := conformanceMinus(written, declared); len(undeclared) > 0 {
			t.Errorf("%s: written over the demo account but not declared in FieldKeys/IssueEnricherFieldKeys: %s",
				td.ShortName, strings.Join(undeclared, ", "))
		}
		if unproduced := conformanceMinus(declared, written); len(unproduced) > 0 {
			t.Errorf("%s: declared in FieldKeys/IssueEnricherFieldKeys but never written over the demo account: %s",
				td.ShortName, strings.Join(unproduced, ", "))
		}
	}
}

// TestACMCertificateARNColumnIsAccepted is the operator's view of the rule
// above on its witness: a list column keyed by certificate_arn, which the acm
// fetcher writes on every row, is one the view loader counts as filled.
func TestACMCertificateARNColumnIsAccepted(t *testing.T) {
	td := catalog.FindAny("acm")
	if td == nil {
		t.Fatal("acm is not registered")
	}
	fetched, _ := demoWrittenKeys(t)
	if !fetched["acm"]["certificate_arn"] {
		t.Fatal("the acm fetcher no longer writes certificate_arn over the demo account — the witness is gone")
	}

	col := config.ListColumn{Title: "Certificate ARN", Key: "certificate_arn"}
	if !config.ColumnFilled(*td, col) {
		t.Error("a column keyed by certificate_arn is rejected as unproduced on acm, " +
			"while the acm fetcher writes it on every row")
	}
	bogus := config.ListColumn{Title: "Nothing", Key: "no_such_acm_key"}
	if config.ColumnFilled(*td, bogus) {
		t.Error("a column keyed by a key nothing writes is accepted on acm — the loader check no longer bites")
	}
}

// viewsReferencePaths reads core/config/views_reference.yaml: per type, every
// SDK struct path an operator may name in a view file.
func viewsReferencePaths(t *testing.T) map[string][]string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(conformanceRepoRoot(t), "core", "config", "views_reference.yaml"))
	if err != nil {
		t.Fatalf("reading views_reference.yaml: %v", err)
	}
	var ref map[string][]string
	if err := yaml.Unmarshal(data, &ref); err != nil {
		t.Fatalf("parsing views_reference.yaml: %v", err)
	}
	return ref
}

// referenceHasPath reports whether path names a leaf or an ancestor of a leaf
// in refs. The reference lists leaves only and marks slices with "[]"; a view
// path names a whole struct or slice without the marker ("VpcConfig",
// "Routes"), and ExtractValue resolves it case-insensitively.
func referenceHasPath(refs []string, path string) bool {
	p := strings.ToLower(path)
	for _, r := range refs {
		r = strings.ToLower(strings.ReplaceAll(r, "[]", ""))
		if r == p || strings.HasPrefix(r, p+".") {
			return true
		}
	}
	return false
}

// detailPathDeclared reports whether a detail path is on the SDK struct or is
// one the type declares it computes from data outside the struct (a policy
// document, an attribute map, a second API call). Those are the only two
// sources a detail row has; the computed list is matched whole, never as a
// prefix, so declaring "Attributes" does not vouch for "AttributesX".
func detailPathDeclared(refs, computed []string, path string) bool {
	if referenceHasPath(refs, path) {
		return true
	}
	for _, c := range computed {
		if strings.EqualFold(c, path) {
			return true
		}
	}
	return false
}

// conformanceTypeDef finds a view's type among parents and children.
func conformanceTypeDef(name string) *catalog.ResourceTypeDef {
	if td := catalog.FindAny(name); td != nil {
		return td
	}
	return catalog.ChildOnly(name)
}

// TestDefaultViewPathsExistOnTheSDKStruct holds every column Path of every
// default view to a path views_reference.yaml lists for that type, and every
// detail Path to the same or to the type's ComputedDetailPaths. A path neither
// source has resolves to nothing on every row, so the cell is blank whatever
// the resource carries.
func TestDefaultViewPathsExistOnTheSDKStruct(t *testing.T) {
	ref := viewsReferencePaths(t)
	views := config.DefaultConfig().Views

	names := make([]string, 0, len(views))
	for name := range views {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		view := views[name]
		refs, ok := ref[name]
		var computed []string
		if td := conformanceTypeDef(name); td != nil {
			computed = td.ComputedDetailPaths
		}
		var missing []string
		for _, col := range view.List {
			if col.Path != "" && !referenceHasPath(refs, col.Path) {
				missing = append(missing, fmt.Sprintf("column %q path %s", col.Title, col.Path))
			}
		}
		for _, d := range view.Detail {
			if d.Path != "" && !detailPathDeclared(refs, computed, d.Path) {
				missing = append(missing, "detail path "+d.Path)
			}
		}
		if len(missing) == 0 {
			continue
		}
		if !ok {
			t.Errorf("%s: no section in views_reference.yaml, so none of its paths can be checked: %s",
				name, strings.Join(missing, "; "))
			continue
		}
		t.Errorf("%s: neither in views_reference.yaml for this type nor in its ComputedDetailPaths: %s",
			name, strings.Join(missing, "; "))
	}
}

// TestReferenceMatchRejectsAPathTheStructLacks keeps the matchers above
// honest: a gate that accepts every path cannot fail, so each is shown
// refusing one.
func TestReferenceMatchRejectsAPathTheStructLacks(t *testing.T) {
	refs := viewsReferencePaths(t)["ec2"]
	if len(refs) == 0 {
		t.Fatal("views_reference.yaml has no ec2 section")
	}
	for _, p := range []string{"InstanceId", "instanceid", "BlockDeviceMappings", "BlockDeviceMappings.Ebs.VolumeId"} {
		if !referenceHasPath(refs, p) {
			t.Errorf("ec2 path %q is on ec2types.Instance but the matcher rejects it", p)
		}
	}
	for _, p := range []string{"Instance", "BlockDevice", "InstanceId.Foo", "Configuration.BytesScannedCutoffPerQuery"} {
		if referenceHasPath(refs, p) {
			t.Errorf("ec2 path %q is not on ec2types.Instance but the matcher accepts it", p)
		}
	}

	computed := []string{"UserData"}
	for _, p := range []string{"UserData", "userdata", "InstanceId"} {
		if !detailPathDeclared(refs, computed, p) {
			t.Errorf("ec2 detail path %q is on the struct or declared computed, but the matcher rejects it", p)
		}
	}
	for _, p := range []string{"UserDataX", "UserData.Value", "Concurrency"} {
		if detailPathDeclared(refs, computed, p) {
			t.Errorf("ec2 detail path %q is neither on the struct nor declared computed, but the matcher accepts it", p)
		}
	}
	if detailPathDeclared(refs, nil, "UserData") {
		t.Error("ec2 UserData is accepted with no computed declaration — the gate cannot tell a declared path from a stray one")
	}
}

// TestNavigableFieldsAreShownInTheDefaultDetail holds every NavigableField to
// a row the type's default detail view renders: its FieldPath equals a detail
// entry or extends one ("VpcConfig.SubnetIds" under "VpcConfig"). A navigable
// field no detail row shows is a jump the operator can never take: the value it
// would jump from is on no screen.
func TestNavigableFieldsAreShownInTheDefaultDetail(t *testing.T) {
	for _, td := range allCatalogTypes(t) {
		detail := config.DefaultViewDef(td.ShortName).Detail
		var missing []string
		for _, nf := range td.Navigable {
			if !detailShowsPath(detail, nf.FieldPath) {
				missing = append(missing, fmt.Sprintf("%s (-> %s)", nf.FieldPath, nf.TargetType))
			}
		}
		if len(missing) > 0 {
			t.Errorf("%s: navigable but on no default detail row: %s", td.ShortName, strings.Join(missing, ", "))
		}
	}
}

func detailShowsPath(detail []config.DetailField, path string) bool {
	p := strings.ToLower(path)
	for _, d := range detail {
		s := strings.ToLower(d.String())
		if s == p || strings.HasPrefix(p, s+".") {
			return true
		}
	}
	return false
}

// TestDetailShowsPathRefusesAnUnshownField keeps the navigable matcher from
// passing vacuously: a sibling path and a parent of a shown path are not shown.
func TestDetailShowsPathRefusesAnUnshownField(t *testing.T) {
	detail := []config.DetailField{{Path: "VpcConfig"}, {Path: "Role"}, {Key: "role_name"}}
	for _, p := range []string{"VpcConfig", "vpcconfig.SubnetIds", "Role", "role_name"} {
		if !detailShowsPath(detail, p) {
			t.Errorf("%q is shown by %v but the matcher rejects it", p, config.DetailStringsForTest(detail))
		}
	}
	for _, p := range []string{"KMSKeyArn", "VpcConfigX", "RoleArn", "Vpc"} {
		if detailShowsPath(detail, p) {
			t.Errorf("%q is not shown by %v but the matcher accepts it", p, config.DetailStringsForTest(detail))
		}
	}
}

// ─── Fields reads in checkers and enrichers ───────────────────────────────

// fieldsRead is one literal Fields["key"] read, attributed to the type whose
// row the expression holds.
type fieldsRead struct {
	key  string
	recv string
	pos  string
	via  string
}

// awsPackageSource is core/aws parsed with local identifier resolution, so a
// read of res.Fields is tied to the declaration of res and a shadowing loop
// variable over another type's cache is not mistaken for it.
type awsPackageSource struct {
	fset  *token.FileSet
	files map[string]*ast.File
	funcs map[string]*ast.FuncDecl
}

func loadAWSPackageSource(t *testing.T) *awsPackageSource {
	t.Helper()
	dir := filepath.Join(conformanceRepoRoot(t), "core", "aws")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("reading core/aws: %v", err)
	}
	src := &awsPackageSource{fset: token.NewFileSet(), files: map[string]*ast.File{}, funcs: map[string]*ast.FuncDecl{}}
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		path := filepath.Join(dir, name)
		f, err := parser.ParseFile(src.fset, path, nil, 0)
		if err != nil {
			t.Fatalf("parsing %s: %v", path, err)
		}
		abs, err := filepath.EvalSymlinks(path)
		if err != nil {
			abs = path
		}
		src.files[abs] = f
		for _, d := range f.Decls {
			if fd, ok := d.(*ast.FuncDecl); ok && fd.Recv == nil {
				src.funcs[fd.Name.Name] = fd
			}
		}
	}
	return src
}

// entryFunc finds the declaration of a registered function value: a named
// function by name, a closure by the file and line its entry points at.
func (s *awsPackageSource) entryFunc(fn any) (*ast.FuncType, *ast.BlockStmt, string, bool) {
	rf := goruntime.FuncForPC(reflect.ValueOf(fn).Pointer())
	if rf == nil {
		return nil, nil, "", false
	}
	name := rf.Name()
	short := name[strings.LastIndex(name, ".")+1:]
	if !strings.HasPrefix(short, "func") {
		if fd, ok := s.funcs[short]; ok && fd.Body != nil {
			return fd.Type, fd.Body, name, true
		}
	}
	file, line := rf.FileLine(rf.Entry())
	abs, err := filepath.EvalSymlinks(file)
	if err != nil {
		abs = file
	}
	f, ok := s.files[abs]
	if !ok {
		return nil, nil, name, false
	}
	var lit *ast.FuncLit
	ast.Inspect(f, func(n ast.Node) bool {
		if l, ok := n.(*ast.FuncLit); ok && lit == nil && s.fset.Position(l.Pos()).Line == line {
			lit = l
		}
		return lit == nil
	})
	if lit == nil {
		return nil, nil, name, false
	}
	return lit.Type, lit.Body, name, true
}

// resourceParamKind reports whether a parameter type is one resource row or a
// slice of them.
func resourceParamKind(expr ast.Expr) (single, slice bool) {
	isResource := func(e ast.Expr) bool {
		if st, ok := e.(*ast.StarExpr); ok {
			e = st.X
		}
		sel, ok := e.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != "Resource" {
			return false
		}
		pkg, ok := sel.X.(*ast.Ident)
		return ok && (pkg.Name == "resource" || pkg.Name == "domain")
	}
	if at, ok := expr.(*ast.ArrayType); ok && at.Len == nil {
		return false, isResource(at.Elt)
	}
	return isResource(expr), false
}

type trackedVars struct {
	single map[*ast.Object]bool
	slice  map[*ast.Object]bool
}

func (tv trackedVars) isRow(e ast.Expr) bool {
	switch x := e.(type) {
	case *ast.ParenExpr:
		return tv.isRow(x.X)
	case *ast.StarExpr:
		return tv.isRow(x.X)
	case *ast.UnaryExpr:
		return x.Op == token.AND && tv.isRow(x.X)
	case *ast.Ident:
		return x.Obj != nil && tv.single[x.Obj]
	case *ast.IndexExpr:
		return tv.isSlice(x.X)
	}
	return false
}

func (tv trackedVars) isSlice(e ast.Expr) bool {
	id, ok := e.(*ast.Ident)
	return ok && id.Obj != nil && tv.slice[id.Obj]
}

// collectFieldsReads walks one function body and every same-package function
// a row is handed to, recording each literal Fields["key"] read on a row.
func (s *awsPackageSource) collectFieldsReads(
	typ *ast.FuncType, body *ast.BlockStmt, via string,
	seed func(i int, obj *ast.Object, tv trackedVars),
	visited map[string]bool, out *[]fieldsRead,
) {
	tv := trackedVars{single: map[*ast.Object]bool{}, slice: map[*ast.Object]bool{}}
	i := 0
	for _, field := range typ.Params.List {
		names := field.Names
		if len(names) == 0 {
			i++
			continue
		}
		for _, n := range names {
			if n.Obj != nil {
				seed(i, n.Obj, tv)
			}
			i++
		}
	}
	if len(tv.single) == 0 && len(tv.slice) == 0 {
		return
	}

	ast.Inspect(body, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.RangeStmt:
			if tv.isSlice(x.X) {
				if v, ok := x.Value.(*ast.Ident); ok && v.Obj != nil {
					tv.single[v.Obj] = true
				}
			}
		case *ast.AssignStmt:
			if len(x.Lhs) == len(x.Rhs) {
				for j, rhs := range x.Rhs {
					if id, ok := x.Lhs[j].(*ast.Ident); ok && id.Obj != nil && tv.isRow(rhs) {
						tv.single[id.Obj] = true
					}
				}
			}
		case *ast.IndexExpr:
			sel, ok := x.X.(*ast.SelectorExpr)
			if !ok || sel.Sel.Name != "Fields" || !tv.isRow(sel.X) {
				return true
			}
			lit, ok := x.Index.(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				return true
			}
			key, err := strconv.Unquote(lit.Value)
			if err != nil {
				return true
			}
			p := s.fset.Position(x.Pos())
			*out = append(*out, fieldsRead{key: key, recv: types.ExprString(sel.X), pos: fmt.Sprintf("%s:%d", filepath.Base(p.Filename), p.Line), via: via})
		case *ast.CallExpr:
			fn, ok := x.Fun.(*ast.Ident)
			if !ok {
				return true
			}
			callee, ok := s.funcs[fn.Name]
			if !ok || callee.Body == nil {
				return true
			}
			rowArgs := map[int]bool{}
			sliceArgs := map[int]bool{}
			for j, a := range x.Args {
				if tv.isRow(a) {
					rowArgs[j] = true
				} else if tv.isSlice(a) {
					sliceArgs[j] = true
				}
			}
			if len(rowArgs) == 0 && len(sliceArgs) == 0 {
				return true
			}
			key := fmt.Sprintf("%s/%v/%v", fn.Name, rowArgs, sliceArgs)
			if visited[key] {
				return true
			}
			visited[key] = true
			s.collectFieldsReads(callee.Type, callee.Body, via+" -> "+fn.Name,
				func(i int, obj *ast.Object, tv trackedVars) {
					if rowArgs[i] {
						tv.single[obj] = true
					}
					if sliceArgs[i] {
						tv.slice[obj] = true
					}
				}, visited, out)
		}
		return true
	})
}

// readsOf returns every literal Fields read a registered function makes on the
// rows it is handed, following those rows into same-package helpers.
func (s *awsPackageSource) readsOf(t *testing.T, fn any) []fieldsRead {
	t.Helper()
	typ, body, name, ok := s.entryFunc(fn)
	if !ok {
		t.Errorf("cannot locate the source of %s in core/aws — the scan would pass it unread", name)
		return nil
	}
	var out []fieldsRead
	s.collectFieldsReads(typ, body, name[strings.LastIndex(name, "/")+1:],
		func(i int, obj *ast.Object, tv trackedVars) {
			field, ok := obj.Decl.(*ast.Field)
			if !ok {
				return
			}
			single, slice := resourceParamKind(field.Type)
			if single {
				tv.single[obj] = true
			}
			if slice {
				tv.slice[obj] = true
			}
		}, map[string]bool{}, &out)
	return out
}

// TestCheckerAndEnricherFieldsReadsNameWrittenKeys holds every literal
// Fields["key"] a related checker, a Wave-2 enricher or a detail enricher reads
// off its own type's rows to a key that type's fetcher (or its Wave-2
// enricher) writes. A read of a key nothing writes is always "", so the arm of
// the match that depends on it silently never fires.
func TestCheckerAndEnricherFieldsReadsNameWrittenKeys(t *testing.T) {
	fetched, enriched := demoWrittenKeys(t)
	src := loadAWSPackageSource(t)

	total := 0
	for _, td := range allCatalogTypes(t) {
		accepted := conformanceUnion(
			conformanceUnion(fetched[td.ShortName], enriched[td.ShortName]),
			conformanceKeySet(td.FieldKeys, td.IssueEnricherFieldKeys),
		)

		var fns []any
		for _, rd := range td.Related {
			if rd.Checker != nil {
				fns = append(fns, rd.Checker)
			}
		}
		if e, ok := awsclient.Wave2EnricherFor(td.ShortName); ok && e.Fn != nil {
			fns = append(fns, e.Fn)
		}
		if de := resource.GetDetailEnricher(td.ShortName); de != nil {
			fns = append(fns, de)
		}

		seen := map[string]bool{}
		for _, fn := range fns {
			for _, r := range src.readsOf(t, fn) {
				total++
				if accepted[r.key] || seen[r.key+r.pos] {
					continue
				}
				seen[r.key+r.pos] = true
				t.Errorf("%s: Fields[%q] read at %s (%s) — no %s row carries that key",
					td.ShortName, r.key, r.pos, r.via, td.ShortName)
			}
		}
	}
	if total < 100 {
		t.Fatalf("the scan found only %d Fields reads across every checker and enricher — it is not reaching them", total)
	}
}

// TestFieldsReadScanFollowsTheRowNotTheName keeps the scan attributing reads
// correctly: dbi's secrets pivot reads Fields["arn"] off a secrets row from the
// cache, which is not a dbi read, while checkEC2TargetGroups reaches
// res.Fields["vpc_id"] through the ec2Identity helper, which is an ec2 read of
// a key the ec2 fetcher writes.
func TestFieldsReadScanFollowsTheRowNotTheName(t *testing.T) {
	src := loadAWSPackageSource(t)

	readKeys := func(shortName string, match string) map[string]bool {
		td := catalog.FindAny(shortName)
		if td == nil {
			t.Fatalf("%s is not registered", shortName)
		}
		keys := map[string]bool{}
		for _, rd := range td.Related {
			if rd.Checker == nil {
				continue
			}
			name := goruntime.FuncForPC(reflect.ValueOf(rd.Checker).Pointer()).Name()
			if !strings.HasSuffix(name, match) {
				continue
			}
			for _, r := range src.readsOf(t, rd.Checker) {
				keys[r.key] = true
			}
		}
		return keys
	}

	if !readKeys("ec2", ".checkEC2TargetGroups")["vpc_id"] {
		t.Error("the scan does not see checkEC2TargetGroups reading res.Fields[\"vpc_id\"] off the ec2 row")
	}
	for _, td := range []string{"dbi", "dbc", "redshift"} {
		td := catalog.FindAny(td)
		if td == nil {
			continue
		}
		for _, rd := range td.Related {
			if rd.TargetType != "secrets" || rd.Checker == nil {
				continue
			}
			for _, r := range src.readsOf(t, rd.Checker) {
				if r.recv == "secretRes" {
					t.Errorf("%s: the scan attributes a secrets row's Fields[%q] at %s to %s", td.ShortName, r.key, r.pos, td.ShortName)
				}
			}
		}
	}
}
