// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// views7_one_resolver_test.go — one lookup for a type name.
//
// A view file, a migration and a report each had to ask twice — the parents,
// then the children — and each place that asked once answered wrong for half
// the catalog. The remaining callers are the same shape: the related panel,
// the navigable fields and the by-id fetch resolve a name among the parents
// only, so a child type that declares any of them is a declaration nothing
// reads. Nothing declares one today, which is the whole hazard: the first one
// added is silently ignored, and the file that ignores it looks correct.
package unit

import (
	"fmt"
	"go/ast"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	_ "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// TestOneResolverForATypeName pins row 23's shape: the parent-only and
// child-only lookups are the catalog's own business, and every caller outside
// it asks the one resolver that answers for both.
func TestOneResolverForATypeName(t *testing.T) {
	fset, files := views7GoFilesUnder(t, "core", "internal", "cmd")

	var offenders []string
	for path, f := range files {
		if strings.Contains(path, filepath.Join("core", "catalog")+string(filepath.Separator)) {
			continue
		}
		ast.Inspect(f, func(n ast.Node) bool {
			sel, ok := n.(*ast.SelectorExpr)
			if !ok || (sel.Sel.Name != "Find" && sel.Sel.Name != "FindChild") {
				return true
			}
			pkg, ok := sel.X.(*ast.Ident)
			if !ok || pkg.Name != "catalog" {
				return true
			}
			rel, _ := filepath.Rel(projectRoot(t), path)
			offenders = append(offenders, fmt.Sprintf("%s:%d: catalog.%s", rel, fset.Position(sel.Pos()).Line, sel.Sel.Name))
			return true
		})
	}
	sort.Strings(offenders)
	for _, o := range offenders {
		t.Errorf("a type name is resolved by half the catalog: %s — one resolver answers for a parent "+
			"and a child, and a caller that picks one registry is a caller that is wrong for the other "+
			"half of the types", o)
	}
}

// TestChildTypeDeclarationsAreReachable pins the behaviour under it. A child
// type registered the way production registers one carries the same fields a
// parent does, and the getters that read them must find it — the related
// panel, the navigable fields and the by-id fetch.
//
// The registry written here is the one production reads: GetChildType has
// consulted it and the catalog's children since before this task, which is
// why a child's ChildFetcher works and its Related does not.
func TestChildTypeDeclarationsAreReachable(t *testing.T) {
	const child = "views7_widget_health"
	resource.SetChildTypeForTest(resource.ResourceTypeDef{
		ShortName: child,
		Columns: []domain.Column{
			{Key: "widget_id", Title: "Widget ID", Width: 24},
			{Key: "health", Title: "Health", Width: 14},
		},
		LifecycleKey: "health",
		Related: []resource.RelatedDef{{
			TargetType: "ec2",
			Checker:    resource.NoopCheckerForTest,
		}},
		Navigable: []resource.NavigableField{{
			FieldPath: "InstanceId", TargetType: "ec2",
		}},
	})
	t.Cleanup(func() { resource.CleanupChildTypeForTest(child) })

	if got := resource.GetChildType(child); got == nil {
		t.Fatalf("the child type %q did not register", child)
	}

	if defs := resource.GetRelated(child); len(defs) != 1 || defs[0].TargetType != "ec2" {
		t.Errorf("the child type's related panel resolves to %v — a child type declares Related the "+
			"same way a parent does, and the lookup that reads it asks the parents only", defs)
	}
	if fields := resource.GetNavigableFields(child); len(fields) != 1 || fields[0].TargetType != "ec2" {
		t.Errorf("the child type's navigable fields resolve to %v — same lookup, same half of the "+
			"catalog", fields)
	}
}
