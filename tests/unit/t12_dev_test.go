package unit_test

import (
	"context"
	"go/ast"
	"go/types"
	"slices"
	"strconv"
	"testing"

	"golang.org/x/tools/go/packages"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	demofixtures "github.com/k2m30/a9s/v3/core/demo/fixtures"
	"github.com/k2m30/a9s/v3/core/resource"
)

// A target group finds its ECS services in what each service's own
// description says about its load balancers. The demo service whose
// description ECS left out carries none of that, so it may be one of them:
// every target group's ECS Services count is a lower bound while that row is
// in the list, and exact once it is not.
func TestT12Dev_StructMatchedPivotOverADegradedDemoTargetIsALowerBound(t *testing.T) {
	b := newRefBench(t)
	svcs := b.byType["ecs-svc"]
	degraded := slices.IndexFunc(svcs, func(r resource.Resource) bool {
		return r.Fields[awsclient.DegradedFindingField] != "" && r.Name == demofixtures.WarnECSSvcDetailsUnavailableID
	})
	if degraded < 0 {
		t.Fatalf("demo ecs-svc list carries no degraded row named %s", demofixtures.WarnECSSvcDetailsUnavailableID)
	}
	described := t567CacheWith(b.cache, "ecs-svc", resource.ResourceCacheEntry{Resources: slices.Delete(slices.Clone(svcs), degraded, degraded+1)})
	check := refChecker(t, "tg", "ecs-svc")
	if len(b.byType["tg"]) == 0 {
		t.Fatal("demo carries no target group")
	}
	for _, tg := range b.byType["tg"] {
		if got := check(context.Background(), refClients(), tg, b.cache); !got.Truncated() {
			t.Errorf("tg %s → ECS Services %s with %s undescribed, want a lower bound", tg.ID, t567Badge(got), svcs[degraded].ID)
		}
		if got := check(context.Background(), refClients(), tg, described); got.Truncated() {
			t.Errorf("tg %s → ECS Services %s over described services only, want an exact count", tg.ID, t567Badge(got))
		}
	}
}

// t12RowFields are the catalog fields whose functions Install binds to the
// type: every row they return carries it and has passed the Wave 1 door.
var t12RowFields = map[string]bool{ //nolint:gochecknoglobals // static field set
	"Fetcher": true, "AvailabilityFetcher": true, "FilteredFetcher": true,
	"ChildFetcher": true, "FetchByIDs": true, "StubCreator": true, "DetailEnrich": true,
}

var t12BoundGetters = map[string]bool{ //nolint:gochecknoglobals // static accessor set
	"GetPaginatedFetcher": true, "GetAvailabilityFetcher": true, "GetPaginatedChildFetcher": true,
	"GetFilteredPaginatedFetcher": true, "GetFetchByIDs": true,
}

// t12WrappedProducers maps each catalog literal's short name to the names of
// the row-producing functions its row fields wrap, so a call graph can follow
// resource.GetX("short") to the functions it reaches.
func t12WrappedProducers(pkg *packages.Package) map[string][]string {
	out := map[string][]string{}
	for _, f := range pkg.Syntax {
		ast.Inspect(f, func(n ast.Node) bool {
			lit, ok := n.(*ast.CompositeLit)
			if !ok {
				return true
			}
			short := ""
			var names []string
			for _, e := range lit.Elts {
				kv, ok := e.(*ast.KeyValueExpr)
				if !ok {
					continue
				}
				key, ok := kv.Key.(*ast.Ident)
				if !ok {
					continue
				}
				if s, isLit := kv.Value.(*ast.BasicLit); isLit && key.Name == "ShortName" {
					short, _ = strconv.Unquote(s.Value)
				}
				if !t12RowFields[key.Name] {
					continue
				}
				ast.Inspect(kv.Value, func(m ast.Node) bool {
					if id, isID := m.(*ast.Ident); isID {
						if fn, isFn := pkg.TypesInfo.Uses[id].(*types.Func); isFn && t12ReturnsRows(fn) {
							names = append(names, fn.Name())
						}
					}
					return true
				})
			}
			if short != "" && len(names) > 0 {
				out[short] = append(out[short], names...)
			}
			return true
		})
	}
	return out
}

func t12BoundCallees(info *types.Info, wrapped map[string][]string, call *ast.CallExpr) []string {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || !t12BoundGetters[sel.Sel.Name] || len(call.Args) != 1 {
		return nil
	}
	if fn, isFn := info.Uses[sel.Sel].(*types.Func); !isFn || fn.Pkg() == nil || fn.Pkg().Path() != "github.com/k2m30/a9s/v3/core/resource" {
		return nil
	}
	lit, ok := call.Args[0].(*ast.BasicLit)
	if !ok {
		return nil
	}
	short, _ := strconv.Unquote(lit.Value)
	return wrapped[short]
}

func t12ReturnsRows(fn *types.Func) bool {
	res := fn.Signature().Results()
	for i := range res.Len() {
		tp := types.Unalias(res.At(i).Type())
		if s, ok := tp.(*types.Slice); ok {
			tp = types.Unalias(s.Elem())
		}
		switch types.TypeString(tp, nil) {
		case "github.com/k2m30/a9s/v3/core/domain.FetchResult", "github.com/k2m30/a9s/v3/core/domain.Resource":
			return true
		}
	}
	return false
}
