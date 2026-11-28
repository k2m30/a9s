package unit_test

import (
	"context"
	"fmt"
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"
	"maps"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"golang.org/x/tools/go/packages"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/catalog"
	"github.com/k2m30/a9s/v3/core/demo"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

type t1aDevStateField struct {
	owner reflect.Type
	name  string
	enum  bool
}

func (s t1aDevStateField) key() string {
	return s.owner.PkgPath() + "." + s.owner.Name() + "." + s.name
}

func t1aDevStateNamed(name string) bool {
	return strings.Contains(name, "Status") || strings.Contains(name, "State")
}

func t1aDevHasValues(t reflect.Type) bool {
	_, ok := t.MethodByName("Values")
	return ok
}

func t1aDevFindState(v reflect.Value, raw string, depth int) (t1aDevStateField, []int, bool) {
	for v.Kind() == reflect.Pointer || v.Kind() == reflect.Interface {
		if v.IsNil() {
			return t1aDevStateField{}, nil, false
		}
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		return t1aDevStateField{}, nil, false
	}
	for i := 0; i < v.NumField(); i++ {
		sf := v.Type().Field(i)
		if !sf.IsExported() {
			continue
		}
		f := v.Field(i)
		ft := sf.Type
		if ft.Kind() == reflect.Pointer && ft.Elem().Kind() == reflect.String {
			if !f.IsNil() && f.Elem().String() == raw && t1aDevStateNamed(sf.Name) {
				return t1aDevStateField{owner: v.Type(), name: sf.Name, enum: t1aDevHasValues(ft.Elem())}, []int{i}, true
			}
			continue
		}
		if ft.Kind() == reflect.String {
			if f.String() == raw && (t1aDevStateNamed(sf.Name) || (sf.Name == "Name" || sf.Name == "Code") && t1aDevStateNamed(v.Type().Name())) {
				return t1aDevStateField{owner: v.Type(), name: sf.Name, enum: t1aDevHasValues(ft)}, []int{i}, true
			}
			continue
		}
		if depth < 2 {
			if s, path, ok := t1aDevFindState(f, raw, depth+1); ok {
				return s, append([]int{i}, path...), true
			}
		}
	}
	return t1aDevStateField{}, nil, false
}

// t1aDevSetState returns a copy of raw with the field at path set to to,
// copying every struct on the way so the demo row is left as it was.
func t1aDevSetState(raw any, path []int, to string) any {
	v := reflect.ValueOf(raw)
	if v.Kind() == reflect.Pointer {
		cp := reflect.New(v.Elem().Type())
		cp.Elem().Set(v.Elem())
		t1aDevSetAt(cp.Elem(), path, to)
		return cp.Interface()
	}
	cp := reflect.New(v.Type()).Elem()
	cp.Set(v)
	t1aDevSetAt(cp, path, to)
	return cp.Interface()
}

func t1aDevSetAt(v reflect.Value, path []int, to string) {
	f := v.Field(path[0])
	if len(path) == 1 {
		if f.Kind() == reflect.Pointer {
			n := reflect.New(f.Type().Elem())
			n.Elem().SetString(to)
			f.Set(n)
			return
		}
		f.SetString(to)
		return
	}
	if f.Kind() == reflect.Pointer {
		n := reflect.New(f.Type().Elem())
		n.Elem().Set(f.Elem())
		t1aDevSetAt(n.Elem(), path[1:], to)
		f.Set(n)
		return
	}
	t1aDevSetAt(f, path[1:], to)
}

// G1: a non-live row that keeps its struct, with only the lifecycle value
// changed in Fields and in the struct, gets no Wave 2 call. The struct is what
// every enricher asserts on, so the zero calls are the runner's doing.
func TestT1aDevGuard_NonLiveRowWithItsStructMakesNoWave2Call(t *testing.T) {
	byType, _ := buildVisibilityTypeCache(t)
	denyAll := func(service, op string, _ any) error { return t576Denied(service, op) }
	falsifiable := 0
	var unbuilt []string
	for _, td := range t1aTables(t) {
		enricher, ok := awsclient.Wave2EnricherFor(td.ShortName)
		if !ok || enricher.ReadsNonLive {
			continue
		}
		rows := byType[td.ShortName]
		idx := slices.IndexFunc(rows, func(r resource.Resource) bool {
			class, _ := catalog.Lifecycle(td, r.Fields[td.StatusKey()])
			return class == catalog.LifecycleNormal && r.RawStruct != nil && r.Fields[td.StatusKey()] != "" && r.Fields[awsclient.DegradedFindingField] == ""
		})
		if idx < 0 {
			unbuilt = append(unbuilt, td.ShortName+" (no normal row with a struct)")
			continue
		}
		live := rows[idx]
		_, path, found := t1aDevFindState(reflect.ValueOf(live.RawStruct), live.Fields[td.StatusKey()], 0)
		if !found {
			unbuilt = append(unbuilt, td.ShortName+" (lifecycle value is not a struct field)")
			continue
		}
		if _, calls := t1aWave2(t, td.ShortName, []resource.Resource{live}, nil); len(calls) == 0 {
			continue
		}
		for _, def := range t1aStateCodes(td) {
			if !t1aNonLive(def.Class) {
				continue
			}
			for _, v := range def.States {
				row := live
				row.Fields = maps.Clone(live.Fields)
				row.Fields[td.StatusKey()] = v
				row.RawStruct = t1aDevSetState(live.RawStruct, path, v)
				row.Findings = []domain.Finding{{Code: def.Code, Phrase: def.Phrase, Severity: def.Severity, Source: def.Source}}
				falsifiable++
				if _, calls := t1aWave2(t, td.ShortName, []resource.Resource{row}, denyAll); len(calls) > 0 {
					keys := make([]string, 0, len(calls))
					for _, c := range calls {
						keys = append(keys, t576CallKey(c))
					}
					t.Errorf("%s row in %q (%v) with its struct: Wave 2 made %d calls: %v", td.ShortName, v, def.Class, len(calls), keys)
				}
			}
		}
	}
	if falsifiable < 20 {
		t.Fatalf("only %d non-live rows were built with their struct; the guard cannot fail. Not built: %v", falsifiable, unbuilt)
	}
	t.Logf("%d non-live rows with their struct; not built: %v", falsifiable, unbuilt)
}

// t1aDevDeniedDescribes lets the list call of every fetcher through and
// refuses every other read, so every list-then-describe fetcher builds its
// degraded rows.
func t1aDevDeniedDescribes(service, op string, _ any) error {
	if strings.HasPrefix(op, "List") {
		return nil
	}
	return t576Denied(service, op)
}

// G2: the lifecycle field holds what AWS reported or nothing. A finding phrase
// stored there, by any route, is caught on the stored value: across every demo
// row and the rows every fetcher builds when its describes are refused.
func TestT1aDevGuard_NoFindingPhraseStoredInTheLifecycleField(t *testing.T) {
	byType, _ := buildVisibilityTypeCache(t)
	clients := demo.NewServiceClients()
	t576WrapClients(clients, &t576Injector{fail: t1aDevDeniedDescribes})
	checked, degraded := 0, 0
	for _, td := range catalog.All() {
		var rows []resource.Resource
		rows = append(rows, byType[td.ShortName]...)
		if td.Fetcher != nil {
			out, _ := td.Fetcher(context.Background(), clients, "") //nolint:errcheck // refused reads are the point
			rows = append(rows, out.Resources...)
		}
		phrases := map[string]bool{}
		for _, def := range td.Findings {
			phrases[def.Phrase] = true
		}
		for _, r := range rows {
			if r.Fields[awsclient.DegradedFindingField] != "" {
				degraded++
			}
			v := r.Fields[td.StatusKey()]
			if v == "" {
				continue
			}
			checked++
			isPhrase := phrases[v] || slices.ContainsFunc(r.Findings, func(f domain.Finding) bool { return f.Phrase == v })
			if !isPhrase {
				continue
			}
			if slices.Contains(td.StateEnum, v) {
				continue
			}
			if _, _, ok := t1aDevFindState(reflect.ValueOf(r.RawStruct), v, 0); ok {
				continue
			}
			t.Errorf("%s/%s Fields[%q] = %q: a finding phrase, not a value AWS reported", td.ShortName, r.ID, td.StatusKey(), v)
		}
	}
	if checked < 200 || degraded < 5 {
		t.Fatalf("checked %d lifecycle values over %d degraded rows; the guard reaches too little", checked, degraded)
	}
	t.Logf("checked %d lifecycle values, %d degraded rows", checked, degraded)
}

// G4: a type whose AWS answer carries a lifecycle value declares a state
// table. The demo rows say which types: the lifecycle key holds the value of
// a status or state field of the struct the type stores.
func TestT1aDevGuard_EveryTypeWithAnAWSLifecycleDeclaresATable(t *testing.T) {
	byType, _ := buildVisibilityTypeCache(t)
	inScope := 0
	for _, td := range catalog.All() {
		reason := ""
		for _, r := range byType[td.ShortName] {
			v := r.Fields[td.StatusKey()]
			if v == "" {
				continue
			}
			if s, _, ok := t1aDevFindState(reflect.ValueOf(r.RawStruct), v, 0); ok {
				reason = "Fields[" + td.StatusKey() + "] holds " + s.key()
				break
			}
		}
		if reason == "" {
			continue
		}
		inScope++
		if len(td.StateEnum) == 0 {
			t.Errorf("%s: %s, and the type declares no state table", td.ShortName, reason)
		}
	}
	if inScope < 30 {
		t.Fatalf("only %d types found with an AWS lifecycle; the catalog walk lost them", inScope)
	}
	t.Logf("%d types carry an AWS lifecycle", inScope)
}

// t1aDevLifecycle is what the AST guard knows of one tabled type.
type t1aDevLifecycle struct {
	short  string
	values map[string]bool
}

// G3: only the state table decides what a lifecycle value means. A string
// literal from a type's lifecycle domain compared against that type's
// lifecycle value — read from the struct field AWS reports it in, from the
// declared lifecycle key in Fields, or through a variable or a parameter that
// one of those flows into — is a second answer outside the table.
func TestT1aDevGuard_NoLifecycleValueComparedToALiteral(t *testing.T) {
	byType, _ := buildVisibilityTypeCache(t)
	byField := map[string][]t1aDevLifecycle{}
	byKey := map[string][]t1aDevLifecycle{}
	for _, td := range t1aTables(t) {
		lc := t1aDevLifecycle{short: td.ShortName, values: map[string]bool{}}
		for _, v := range td.StateEnum {
			lc.values[v] = true
		}
		byKey[td.StatusKey()] = append(byKey[td.StatusKey()], lc)
		seen := map[string]bool{}
		for _, r := range byType[td.ShortName] {
			if s, _, ok := t1aDevFindState(reflect.ValueOf(r.RawStruct), r.Fields[td.StatusKey()], 0); ok && !seen[s.key()] {
				seen[s.key()] = true
				byField[s.key()] = append(byField[s.key()], lc)
			}
		}
	}
	if len(byField) < 25 {
		t.Fatalf("found the struct field of only %d lifecycle values", len(byField))
	}

	cfg := &packages.Config{Mode: packages.NeedName | packages.NeedFiles | packages.NeedSyntax | packages.NeedTypes | packages.NeedTypesInfo, Dir: filepath.Join("..", "..")}
	pkgs, err := loadPackagesOnce(cfg, "./core/aws")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	// The ecs_tasks child kind classifies its tasks with its own switch; a
	// child kind has no state table.
	childKinds := map[string]bool{"ecsTaskWave1Findings": true, "ecsTaskStructuralFindings": true}
	var sites []string
	for _, p := range pkgs {
		for _, e := range p.Errors {
			t.Fatalf("%s: %v", p.PkgPath, e)
		}
		info := p.TypesInfo
		vars := map[types.Object][]t1aDevLifecycle{}
		var read func(e ast.Expr) []t1aDevLifecycle
		read = func(e ast.Expr) []t1aDevLifecycle {
			switch x := ast.Unparen(e).(type) {
			case *ast.SelectorExpr:
				tv, ok := info.Types[x.X]
				if !ok {
					return nil
				}
				owner := tv.Type
				if ptr, isPtr := owner.(*types.Pointer); isPtr {
					owner = ptr.Elem()
				}
				named, ok := owner.(*types.Named)
				if !ok || named.Obj().Pkg() == nil {
					return nil
				}
				return byField[named.Obj().Pkg().Path()+"."+named.Obj().Name()+"."+x.Sel.Name]
			case *ast.CallExpr:
				if len(x.Args) == 1 {
					return read(x.Args[0])
				}
			case *ast.IndexExpr:
				if tv, ok := info.Types[x.Index]; ok && tv.Value != nil && tv.Value.Kind() == constant.String {
					if m, ok := info.Types[x.X].Type.Underlying().(*types.Map); ok && types.Identical(m.Key(), types.Typ[types.String]) {
						return byKey[constant.StringVal(tv.Value)]
					}
				}
			case *ast.Ident:
				return vars[info.ObjectOf(x)]
			}
			return nil
		}
		mark := func(obj types.Object, lcs []t1aDevLifecycle) bool {
			if obj == nil || len(lcs) == 0 || len(vars[obj]) > 0 {
				return false
			}
			vars[obj] = lcs
			return true
		}
		for changed := true; changed; {
			changed = false
			for _, f := range p.Syntax {
				ast.Inspect(f, func(n ast.Node) bool {
					switch x := n.(type) {
					case *ast.AssignStmt:
						if len(x.Lhs) == len(x.Rhs) {
							for i, l := range x.Lhs {
								if id, ok := l.(*ast.Ident); ok && mark(info.ObjectOf(id), read(x.Rhs[i])) {
									changed = true
								}
							}
						}
					case *ast.CallExpr:
						fn, ok := info.Uses[t1aDevCallee(x.Fun)].(*types.Func)
						if !ok || fn.Pkg() != p.Types {
							return true
						}
						sig := fn.Type().(*types.Signature)
						for i, a := range x.Args {
							if i < sig.Params().Len() && mark(sig.Params().At(i), read(a)) {
								changed = true
							}
						}
					}
					return true
				})
			}
		}
		literal := func(e ast.Expr) (string, bool) {
			tv, ok := info.Types[e]
			if !ok || tv.Value == nil || tv.Value.Kind() != constant.String {
				return "", false
			}
			if _, isLit := ast.Unparen(e).(*ast.BasicLit); !isLit {
				return "", false
			}
			return constant.StringVal(tv.Value), true
		}
		report := func(pos token.Pos, lcs []t1aDevLifecycle, lit string) {
			for _, lc := range lcs {
				if lc.values[lit] {
					at := p.Fset.Position(pos)
					sites = append(sites, fmt.Sprintf("%s:%d %s %q", filepath.Base(at.Filename), at.Line, lc.short, lit))
					return
				}
			}
		}
		for _, f := range p.Syntax {
			for _, decl := range f.Decls {
				fd, ok := decl.(*ast.FuncDecl)
				if !ok || fd.Body == nil || childKinds[fd.Name.Name] {
					continue
				}
				ast.Inspect(fd.Body, func(n ast.Node) bool {
					switch x := n.(type) {
					case *ast.BinaryExpr:
						if x.Op != token.EQL && x.Op != token.NEQ {
							return true
						}
						if lit, ok := literal(x.Y); ok {
							report(x.Pos(), read(x.X), lit)
						}
						if lit, ok := literal(x.X); ok {
							report(x.Pos(), read(x.Y), lit)
						}
					case *ast.SwitchStmt:
						if x.Tag == nil {
							return true
						}
						lcs := read(x.Tag)
						for _, s := range x.Body.List {
							for _, e := range s.(*ast.CaseClause).List {
								if lit, ok := literal(e); ok {
									report(e.Pos(), lcs, lit)
								}
							}
						}
					}
					return true
				})
			}
		}
	}
	slices.Sort(sites)
	if len(sites) > 0 {
		t.Errorf("%d comparisons of a lifecycle value against a literal outside the state table: %v", len(sites), sites)
	}
}

// A lifecycle value a type's table does not declare reads the same way on
// every type, "<value> (unrecognised)", because one builder makes every
// UnknownState FindingDef: no FindingDef literal in core/aws carries such a
// code.
func TestT1aDevGuard_UnknownStatePhraseHasOneBuilder(t *testing.T) {
	unknown := map[string]string{}
	for _, td := range t1aTables(t) {
		if td.UnknownState == "" {
			t.Errorf("%s declares a state table and no UnknownState", td.ShortName)
			continue
		}
		unknown[string(td.UnknownState)] = td.ShortName
		for _, def := range td.Findings {
			if def.Code == td.UnknownState && !strings.HasSuffix(def.Phrase, " (unrecognised)") {
				t.Errorf("%s: unknown-state phrase %q does not end in \" (unrecognised)\"", td.ShortName, def.Phrase)
			}
		}
	}
	cfg := &packages.Config{Mode: packages.NeedName | packages.NeedFiles | packages.NeedSyntax | packages.NeedTypes | packages.NeedTypesInfo, Dir: filepath.Join("..", "..")}
	pkgs, err := loadPackagesOnce(cfg, "./core/aws")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	var sites []string
	for _, p := range pkgs {
		for _, f := range p.Syntax {
			ast.Inspect(f, func(n ast.Node) bool {
				kv, ok := n.(*ast.KeyValueExpr)
				if !ok {
					return true
				}
				if k, isIdent := kv.Key.(*ast.Ident); !isIdent || k.Name != "Code" {
					return true
				}
				tv, ok := p.TypesInfo.Types[kv.Value]
				if !ok || tv.Value == nil || tv.Value.Kind() != constant.String {
					return true
				}
				if short, isUnknown := unknown[constant.StringVal(tv.Value)]; isUnknown {
					at := p.Fset.Position(kv.Pos())
					sites = append(sites, fmt.Sprintf("%s:%d (%s)", filepath.Base(at.Filename), at.Line, short))
				}
				return true
			})
		}
	}
	if len(sites) > 0 {
		t.Errorf("%d unknown-state FindingDefs not made by unrecognisedStateDef: %v", len(sites), sites)
	}
}

func t1aDevCallee(fun ast.Expr) *ast.Ident {
	switch f := fun.(type) {
	case *ast.Ident:
		return f
	case *ast.SelectorExpr:
		return f.Sel
	}
	return &ast.Ident{}
}

// The text filter finds a row by the lifecycle value AWS reported, which the
// blank Status cell of a healthy row of a tabled type does not show.
func TestT1aDev_FilterMatchesTheRawLifecycleValue(t *testing.T) {
	for _, tc := range []struct{ short, raw string }{
		{"ec2", "running"},
		{"dbi", "available"},
		{"eks", "active"},
		{"vpc", "available"},
	} {
		t.Run(tc.short, func(t *testing.T) {
			td := resource.FindResourceType(tc.short)
			page, _ := td.Fetcher(context.Background(), demo.NewServiceClients(), "")
			want := 0
			for _, r := range page.Resources {
				if strings.Contains(strings.ToLower(r.Fields[td.StatusKey()]), tc.raw) {
					want++
				}
			}
			if want == 0 {
				t.Fatalf("no demo %s row is %s", tc.short, tc.raw)
			}
			if n, _ := filterHits(t, tc.short, tc.raw); n < want {
				t.Errorf("filter %q on %s matched %d rows, want at least the %d rows AWS reports as %s", tc.raw, tc.short, n, want, tc.raw)
			}
		})
	}
}
