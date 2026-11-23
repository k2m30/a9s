package unit_test

import (
	"fmt"
	"go/ast"
	"go/types"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"golang.org/x/tools/go/packages"
)

// t6DevCutRecorders are the helpers that record what an early stop left: a
// mark, an unreached row, or an error wrapping errPageCap, which MarkSkipped
// turns into one.
var t6DevCutRecorders = map[string]bool{
	"MarkSkipped": true, "markUninspected": true, "MarkUnusable": true,
	"markUnreached": true, "errPageCap": true, "errRedshiftParamsCutShort": true,
}

// t6DevUnmarkedCuts are the walks whose cut leaves no row unanswered, keyed by
// file, function and the walk's first result, each with the fact that makes
// it so.
var t6DevUnmarkedCuts = map[string]string{
	"apigw_issue_enrichment.go:apigwHTTPRow:stages":                      "stages_count reads N+; the stages decide only \"~\" findings",
	"codeartifact_issue_enrichment.go:EnrichCodeArtifactRepository:pkgs": "package_count reads N+ and decides no finding",
	"sns_issue_enrichment.go:snsTopicRow:subs":                           "subs_count reads N+; the subscriptions decide only \"~\" findings",
	"elb_issue_enrichment.go:allELBListeners:listeners":                  "elb declares only \"~\" Wave-2 findings",
	"eb_rule_issue_enrichment.go:EnrichEventBridgeRuleTargets:targets":   "target_count reads N+; the targets decide only \"~\" findings",
	"iam_user_issue_enrichment.go:EnrichIAMUserMFA:devices":              "a walk that reached its cap read a device, which is the whole MFA verdict",
}

func t6DevMentions(n ast.Node, names map[string]bool) bool {
	found := false
	ast.Inspect(n, func(m ast.Node) bool {
		if id, ok := m.(*ast.Ident); ok && names[id.Name] {
			found = true
		}
		return !found
	})
	return found
}

func t6DevRecordsUnconditionally(stmts []ast.Stmt) bool {
	for _, s := range stmts {
		switch s := s.(type) {
		case *ast.IfStmt:
			if s.Else != nil && t6DevRecordsUnconditionally(s.Body.List) && t6DevRecordsUnconditionally([]ast.Stmt{s.Else}) {
				return true
			}
		case *ast.BlockStmt:
			if t6DevRecordsUnconditionally(s.List) {
				return true
			}
		case *ast.ForStmt:
			if t6DevRecordsUnconditionally(s.Body.List) || t6DevMentions(s.Body, t6DevCutRecorders) {
				return true
			}
		case *ast.RangeStmt:
			if t6DevMentions(s.Body, t6DevCutRecorders) {
				return true
			}
		case *ast.SwitchStmt, *ast.TypeSwitchStmt, *ast.SelectStmt:
		default:
			if t6DevMentions(s, t6DevCutRecorders) {
				return true
			}
		}
	}
	return false
}

func t6DevUses(n ast.Node, obj types.Object, info *types.Info) bool {
	found := false
	ast.Inspect(n, func(m ast.Node) bool {
		if id, ok := m.(*ast.Ident); ok && info.Uses[id] == obj {
			found = true
		}
		return !found
	})
	return found
}

// An enricher stops early only through the helpers that record what it left:
// a page walk whose cap or refusal is recorded by a mark, an unreached row or
// errPageCap, and a loop over the rows that never returns or breaks out of
// it. An early stop anywhere else leaves rows nobody answered that the marks
// do not name, so the issue count reads exact and the scan reads ok.
func TestT6Dev_EnricherCutsGoThroughTheHelpers(t *testing.T) {
	cfg := &packages.Config{
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedSyntax | packages.NeedTypes | packages.NeedTypesInfo,
		Dir:  filepath.Join("..", ".."),
	}
	pkgs, err := packages.Load(cfg, "./core/aws")
	if err != nil || len(pkgs) != 1 || len(pkgs[0].Errors) > 0 {
		t.Fatalf("load core/aws: %v %v", err, pkgs)
	}
	pkg := pkgs[0]
	var hits []string
	walks := 0
	seenUnmarked := map[string]bool{}
	for _, f := range pkg.Syntax {
		file := filepath.Base(pkg.Fset.Position(f.Pos()).Filename)
		if !strings.HasSuffix(file, "_issue_enrichment.go") {
			continue
		}
		for _, d := range f.Decls {
			decl, ok := d.(*ast.FuncDecl)
			if !ok || decl.Body == nil {
				continue
			}
			where := file + ":" + decl.Name.Name
			ast.Inspect(decl.Body, func(n ast.Node) bool {
				switch n := n.(type) {
				case *ast.AssignStmt:
					call, isCall := n.Rhs[0].(*ast.CallExpr)
					if !isCall || len(n.Lhs) != 3 {
						return true
					}
					fn := call.Fun
					if ix, isIndex := fn.(*ast.IndexExpr); isIndex {
						fn = ix.X
					}
					if id, isIdent := fn.(*ast.Ident); !isIdent || id.Name != "PageAll" {
						return true
					}
					walks++
					if first, named := n.Lhs[0].(*ast.Ident); named {
						key := where + ":" + first.Name
						if _, unmarked := t6DevUnmarkedCuts[key]; unmarked {
							seenUnmarked[key] = true
							return true
						}
					}
					completeID, ok := n.Lhs[1].(*ast.Ident)
					if !ok || completeID.Name == "_" {
						hits = append(hits, fmt.Sprintf("%s: %s drops PageAll's complete", pkg.Fset.Position(n.Pos()), where))
						return true
					}
					obj := pkg.TypesInfo.Defs[completeID]
					recorded := false
					ast.Inspect(decl.Body, func(m ast.Node) bool {
						switch m := m.(type) {
						case *ast.IfStmt:
							if t6DevUses(m.Cond, obj, pkg.TypesInfo) && t6DevRecordsUnconditionally(m.Body.List) {
								recorded = true
							}
						case *ast.CaseClause:
							for _, e := range m.List {
								if t6DevUses(e, obj, pkg.TypesInfo) && t6DevRecordsUnconditionally(m.Body) {
									recorded = true
								}
							}
						}
						return !recorded
					})
					if !recorded {
						hits = append(hits, fmt.Sprintf("%s: %s cuts a PageAll walk without recording it", pkg.Fset.Position(n.Pos()), where))
					}
				case *ast.RangeStmt:
					tv, ok := pkg.TypesInfo.Types[n.X]
					if !ok {
						return true
					}
					sl, ok := tv.Type.(*types.Slice)
					if !ok || !types.Identical(sl.Elem(), t6DevResourceType(pkg)) {
						return true
					}
					ast.Inspect(n.Body, func(m ast.Node) bool {
						switch m := m.(type) {
						case *ast.FuncLit:
							return false
						case *ast.ReturnStmt:
							hits = append(hits, fmt.Sprintf("%s: %s returns out of a loop over its rows", pkg.Fset.Position(m.Pos()), where))
						case *ast.BranchStmt:
							if m.Tok.String() == "break" && m.Label == nil {
								hits = append(hits, fmt.Sprintf("%s: %s breaks out of a loop over its rows", pkg.Fset.Position(m.Pos()), where))
							}
						case *ast.ForStmt, *ast.RangeStmt, *ast.SwitchStmt, *ast.TypeSwitchStmt, *ast.SelectStmt:
							return m == n.Body
						}
						return true
					})
				}
				return true
			})
		}
	}
	if walks == 0 {
		t.Fatal("no PageAll walk found in the issue enrichers")
	}
	for key := range t6DevUnmarkedCuts {
		if !seenUnmarked[key] {
			hits = append(hits, "t6DevUnmarkedCuts names "+key+", which no walk matches — delete the entry")
		}
	}
	sort.Strings(hits)
	if len(hits) > 0 {
		t.Errorf("%d early stop(s) outside the cut helpers:\n  %s", len(hits), strings.Join(hits, "\n  "))
	}
}

const (
	t6DevOwnerFile = "core/runtime/issue_class.go"
	t6DevDomainPkg = "github.com/k2m30/a9s/v3/core/domain"
)

// t6DevSeverityDrawing are the functions outside the owner that read a
// severity to draw a finding, never to count or keep a row.
var t6DevSeverityDrawing = map[string]string{
	"github.com/k2m30/a9s/v3/core/app.buildAttentionEntries": "the detail Attention block's tier",
	"github.com/k2m30/a9s/v3/cmd/catalogen.severityColor":    "the generated catalog's severity colour",
	"github.com/k2m30/a9s/v3/cmd/catalogen.severityLabel":    "the generated catalog's severity label",
}

func t6DevLoadApp(t *testing.T) []*packages.Package {
	t.Helper()
	cfg := &packages.Config{
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedSyntax | packages.NeedTypes | packages.NeedTypesInfo,
		Dir:  filepath.Join("..", ".."),
	}
	pkgs, err := packages.Load(cfg, "./core/app/...", "./core/runtime/...", "./core/web/...", "./internal/...", "./cmd/...")
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range pkgs {
		if len(p.Errors) > 0 {
			t.Fatalf("load %s: %v", p.PkgPath, p.Errors)
		}
	}
	return pkgs
}

func t6DevEachFunc(pkgs []*packages.Package, fn func(pkg *packages.Package, file, name string, body *ast.BlockStmt)) {
	for _, pkg := range pkgs {
		for _, f := range pkg.Syntax {
			file := pkg.Fset.Position(f.Pos()).Filename
			if strings.HasSuffix(file, "_test.go") {
				continue
			}
			for _, d := range f.Decls {
				if decl, ok := d.(*ast.FuncDecl); ok && decl.Body != nil {
					fn(pkg, file, t6FuncName(pkg, decl), decl.Body)
				}
			}
		}
	}
}

// Whether a row counts or is kept by the issues-only filter is decided from a
// finding's severity in the owner alone: a comparison against SevWarn or
// SevBroken anywhere else in the app, the runtime, the web host, the terminal
// host or a command is a second answer, except where it only draws.
func TestT6Dev_SeverityIsReadOnlyByTheOwner(t *testing.T) {
	pkgs := t6DevLoadApp(t)
	seenDrawing := map[string]bool{}
	var hits []string
	t6DevEachFunc(pkgs, func(pkg *packages.Package, file, name string, body *ast.BlockStmt) {
		if strings.HasSuffix(file, t6DevOwnerFile) {
			return
		}
		ast.Inspect(body, func(n ast.Node) bool {
			sel, ok := n.(*ast.SelectorExpr)
			if !ok || (sel.Sel.Name != "SevWarn" && sel.Sel.Name != "SevBroken") {
				return true
			}
			obj := pkg.TypesInfo.Uses[sel.Sel]
			if obj == nil || obj.Pkg() == nil || obj.Pkg().Path() != t6DevDomainPkg {
				return true
			}
			if _, drawing := t6DevSeverityDrawing[name]; drawing {
				seenDrawing[name] = true
				return true
			}
			hits = append(hits, fmt.Sprintf("%s: %s reads domain.%s", pkg.Fset.Position(sel.Pos()), name, sel.Sel.Name))
			return true
		})
	})
	for name := range t6DevSeverityDrawing {
		if !seenDrawing[name] {
			hits = append(hits, "t6DevSeverityDrawing names "+name+", which no longer reads a severity — delete the entry")
		}
	}
	sort.Strings(hits)
	if len(hits) > 0 {
		t.Errorf("severity read outside the issue owner:\n  %s", strings.Join(hits, "\n  "))
	}
}

// ExcludeFromIssueBadge is read by the owner alone (runtime.ShowsIssueBadge);
// every surface that draws a number asks it.
func TestT6Dev_ExclusionIsReadOnlyByTheOwner(t *testing.T) {
	pkgs := t6DevLoadApp(t)
	var hits []string
	t6DevEachFunc(pkgs, func(pkg *packages.Package, file, name string, body *ast.BlockStmt) {
		if strings.HasSuffix(file, t6DevOwnerFile) {
			return
		}
		ast.Inspect(body, func(n ast.Node) bool {
			if sel, ok := n.(*ast.SelectorExpr); ok && sel.Sel.Name == "ExcludeFromIssueBadge" {
				hits = append(hits, fmt.Sprintf("%s: %s", pkg.Fset.Position(sel.Pos()), name))
			}
			return true
		})
	})
	sort.Strings(hits)
	if len(hits) > 0 {
		t.Errorf("ExcludeFromIssueBadge read outside the issue owner:\n  %s", strings.Join(hits, "\n  "))
	}
}

func t6DevResourceType(pkg *packages.Package) types.Type {
	for _, imp := range pkg.Types.Imports() {
		if imp.Path() == "github.com/k2m30/a9s/v3/core/resource" {
			return imp.Scope().Lookup("Resource").Type()
		}
	}
	return nil
}
