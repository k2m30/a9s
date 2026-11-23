package unit_test

import (
	"context"
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"golang.org/x/tools/go/packages"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/demo"
	"github.com/k2m30/a9s/v3/core/resource"
)

const t6DomainPkg = "github.com/k2m30/a9s/v3/core/domain"

// t6DrawingSites are the functions whose IsIssue call decides how a row or a
// finding is drawn, never whether a row is counted or kept by a filter.
var t6DrawingSites = map[string]string{
	"github.com/k2m30/a9s/v3/core/app.resolveListRowSeverity": "tints the row",
	"github.com/k2m30/a9s/v3/core/app.buildAttentionEntries":  "lays out the detail Attention block",
	"github.com/k2m30/a9s/v3/core/app.listBodyBuild.run":      "fills a row's Status cell",
}

type t6Source struct {
	pkgs []*packages.Package
}

func t6LoadSource(t *testing.T) t6Source {
	t.Helper()
	cfg := &packages.Config{
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedSyntax | packages.NeedTypes | packages.NeedTypesInfo,
		Dir:  filepath.Join("..", ".."),
	}
	pkgs, err := packages.Load(cfg, "./core/...", "./internal/...", "./cmd/...")
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range pkgs {
		if len(p.Errors) > 0 {
			t.Fatalf("load %s: %v", p.PkgPath, p.Errors)
		}
	}
	return t6Source{pkgs: pkgs}
}

func (s t6Source) each(fn func(pkg *packages.Package, file string, decl *ast.FuncDecl)) {
	for _, pkg := range s.pkgs {
		for _, f := range pkg.Syntax {
			file := pkg.Fset.Position(f.Pos()).Filename
			if strings.HasSuffix(file, "_test.go") {
				continue
			}
			for _, d := range f.Decls {
				if decl, ok := d.(*ast.FuncDecl); ok && decl.Body != nil {
					fn(pkg, file, decl)
				}
			}
		}
	}
}

func t6FuncName(pkg *packages.Package, decl *ast.FuncDecl) string {
	name := decl.Name.Name
	if decl.Recv != nil && len(decl.Recv.List) == 1 {
		recv := decl.Recv.List[0].Type
		if star, ok := recv.(*ast.StarExpr); ok {
			recv = star.X
		}
		if id, ok := recv.(*ast.Ident); ok {
			name = id.Name + "." + name
		}
	}
	return pkg.PkgPath + "." + name
}

type t6IssueCall struct {
	fn, file, recv string
	pos            token.Position
}

func t6IssueCalls(s t6Source) []t6IssueCall {
	var out []t6IssueCall
	s.each(func(pkg *packages.Package, file string, decl *ast.FuncDecl) {
		if pkg.PkgPath == t6DomainPkg {
			return
		}
		ast.Inspect(decl.Body, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok || sel.Sel.Name != "IsIssue" {
				return true
			}
			selection := pkg.TypesInfo.Selections[sel]
			if selection == nil {
				return true
			}
			named, ok := types.Unalias(selection.Recv()).(*types.Named)
			if !ok || named.Obj().Pkg() == nil || named.Obj().Pkg().Path() != t6DomainPkg {
				return true
			}
			out = append(out, t6IssueCall{fn: t6FuncName(pkg, decl), file: file, recv: named.Obj().Name(), pos: pkg.Fset.Position(call.Pos())})
			return true
		})
	})
	return out
}

// Whether a row is an issue is decided by one function: the menu badge, the
// disk count, the list title count, the ctrl+z filter and both probes ask it.
// A colour's or a severity's IsIssue outside that function and its file is a
// second answer to the same question, except where it only decides how a row
// or a finding is drawn.
func TestT6Guard_OneFunctionDecidesARowIsAnIssue(t *testing.T) {
	calls := t6IssueCalls(t6LoadSource(t))
	seenDrawing := map[string]bool{}
	owners := map[string]string{}
	var severity []t6IssueCall
	for _, c := range calls {
		if _, drawing := t6DrawingSites[c.fn]; drawing {
			seenDrawing[c.fn] = true
			continue
		}
		switch c.recv {
		case "Color":
			owners[c.fn] = c.file
		case "Severity":
			severity = append(severity, c)
		}
	}
	for fn := range t6DrawingSites {
		if !seenDrawing[fn] {
			t.Errorf("t6DrawingSites names %s, which no longer calls IsIssue — delete the entry", fn)
		}
	}

	var list []string
	for _, c := range calls {
		if _, drawing := t6DrawingSites[c.fn]; !drawing {
			list = append(list, fmt.Sprintf("%s: %s.IsIssue in %s", c.pos, c.recv, c.fn))
		}
	}
	sort.Strings(list)
	if len(owners) > 1 {
		t.Errorf("%d functions decide from a colour whether a row is an issue; one owner must, and everything else asks it:\n  %s",
			len(owners), strings.Join(list, "\n  "))
	}
	ownerFile := ""
	if len(owners) == 1 {
		for _, file := range owners {
			ownerFile = file
		}
	}
	for _, c := range severity {
		if c.file != ownerFile {
			t.Errorf("%s: %s decides from a finding's severity whether a row is an issue, outside the owner", c.pos, c.fn)
		}
	}
}

// The lower bound of an issue count is decided from the rows marked not
// inspected and the catalog's severities alone, so nothing carries a cut flag
// of its own.
func TestT6Guard_NoCutFlagBesideTheMarkedRows(t *testing.T) {
	if _, ok := reflect.TypeFor[awsclient.IssueEnricherResult]().FieldByName("Truncated"); ok {
		t.Error("IssueEnricherResult still has a Truncated field")
	}
	banned := map[string]bool{
		"SetTruncated":          true,
		"MarkInformationalOnly": true,
		"listHasBadgeFinding":   true,
		"listHasIssueFinding":   true,
	}
	var hits []string
	s := t6LoadSource(t)
	for _, pkg := range s.pkgs {
		for _, f := range pkg.Syntax {
			file := pkg.Fset.Position(f.Pos()).Filename
			if strings.HasSuffix(file, "_test.go") {
				continue
			}
			ast.Inspect(f, func(n ast.Node) bool {
				if id, ok := n.(*ast.Ident); ok && (banned[id.Name] || strings.HasPrefix(id.Name, "EnrichmentCut")) {
					hits = append(hits, fmt.Sprintf("%s: %s", pkg.Fset.Position(id.Pos()), id.Name))
				}
				return true
			})
		}
	}
	sort.Strings(hits)
	if len(hits) > 0 {
		t.Errorf("%d reference(s) to a cut flag or a second issue predicate:\n  %s", len(hits), strings.Join(hits, "\n  "))
	}
}

// Every Fields key an enricher writes is one its type declares in
// IssueEnricherFieldKeys, and every declared key is written on every row the
// enricher answered — empty when it has no value — so an answer without the
// value clears the one an earlier answer wrote. A row the enricher did not
// reach keeps the value it had.
func TestT6Guard_EnricherFieldsAreDeclaredAndWrittenOnEveryJudgedRow(t *testing.T) {
	byType, cache := buildVisibilityTypeCache(t)
	checked := 0
	for _, w := range awsclient.AllWave2() {
		rows := byType[w.ShortName]
		td := resource.FindResourceType(w.ShortName)
		if len(rows) == 0 || td == nil {
			continue
		}
		checked++
		t.Run(w.ShortName, func(t *testing.T) {
			var judged []resource.Resource
			var unreached map[string]struct{}
			wrapped := w.Enricher
			inner := w.Enricher.Fn
			wrapped.Fn = func(ctx context.Context, c *awsclient.ServiceClients, rs []resource.Resource, rc resource.ResourceCache) (awsclient.IssueEnricherResult, error) {
				judged = append([]resource.Resource(nil), rs...)
				res, err := inner(ctx, c, rs, rc)
				unreached = res.Unreached
				return res, err
			}
			awsclient.SetWave2EnricherForTest(t, w.ShortName, wrapped)

			a := t6NewApp(t, w.ShortName, demo.NewServiceClients(), rows, cache, false)
			ev := a.wave2(t)

			declared := map[string]bool{}
			for _, k := range td.IssueEnricherFieldKeys {
				declared[k] = true
			}
			undeclared := map[string]string{}
			for id, fields := range ev.FieldUpdates {
				for k := range fields {
					if !declared[k] {
						undeclared[k] = id
					}
				}
			}
			for k, id := range undeclared {
				t.Errorf("writes Fields[%q] (e.g. on %s), which IssueEnricherFieldKeys %v does not declare", k, id, td.IssueEnricherFieldKeys)
			}

			stored, _ := a.core.ProbeResources(w.ShortName)
			byID := make(map[string]resource.Resource, len(stored))
			for _, r := range stored {
				byID[r.ID] = r
			}
			for _, k := range td.IssueEnricherFieldKeys {
				var missing []string
				n := 0
				for _, r := range judged {
					if _, marked := ev.TruncatedIDs[r.ID]; marked {
						continue
					}
					if _, left := unreached[r.ID]; left {
						continue
					}
					n++
					if _, ok := byID[r.ID].Fields[k]; !ok {
						missing = append(missing, r.ID)
					}
				}
				if len(missing) > 0 {
					sort.Strings(missing)
					t.Errorf("declared Fields[%q] is absent on %d of %d judged rows, e.g. %s", k, len(missing), n, missing[0])
				}
			}
		})
	}
	if checked == 0 {
		t.Fatal("no enricher ran on demo rows")
	}
}
