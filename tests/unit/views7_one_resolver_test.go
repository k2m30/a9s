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
	"go/ast"
	"path/filepath"
	"testing"

	_ "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/catalog"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// views7LookupSurface is every exported way to turn a type name into a type
// definition. FindAny answers for a parent and a child alike; the two narrow
// ones exist because two callers mean one half and say so — the parents
// accessor and the child registry. A fourth is a caller somewhere resolving
// half the catalog again, which is the defect this task removed from eight
// places.
var views7LookupSurface = map[string]bool{
	"FindAny":      true,
	"TopLevelOnly": true,
	"ChildOnly":    true,
}

// TestCatalogLookupSurfaceIsTheThree pins row 25. Naming the two lookups that
// used to be exported made the gate green the day they were renamed; the shape
// is the surface, so it is the surface that is asserted.
func TestCatalogLookupSurfaceIsTheThree(t *testing.T) {
	_, files := views7GoFilesUnder(t, filepath.Join("core", "catalog"))

	found := map[string]bool{}
	for path, f := range files {
		for _, decl := range f.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Recv != nil || !fn.Name.IsExported() || fn.Type.Results == nil {
				continue
			}
			for _, r := range fn.Type.Results.List {
				star, ok := r.Type.(*ast.StarExpr)
				if !ok {
					continue
				}
				if id, ok := star.X.(*ast.Ident); ok && id.Name == "ResourceTypeDef" {
					found[fn.Name.Name] = true
					if !views7LookupSurface[fn.Name.Name] {
						rel, _ := filepath.Rel(projectRoot(t), path)
						t.Errorf("core/catalog exports %s, a fourth way to resolve a type name (%s) — "+
							"one resolver answers for a parent and a child, and the two narrow lookups "+
							"exist because their callers mean one half and say so. A third narrow one is "+
							"a caller resolving half the catalog again",
							fn.Name.Name, rel)
					}
				}
			}
		}
	}

	for name := range views7LookupSurface {
		if !found[name] {
			t.Errorf("core/catalog no longer exports %s — the gate names the surface, so the surface "+
				"and this list are changed together", name)
		}
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

	// The registered half of the same claim, on a child the catalog itself
	// carries: the general resolver answers for it, and the parents-only
	// lookup is what "resolving half the catalog" looks like. Without this,
	// every lookup below is answered by the test registry's own arm and the
	// catalog arm could be narrowed back with nothing to say so.
	const catalogChild = "tg_health"
	if catalog.FindAny(catalogChild) == nil {
		t.Errorf("catalog.FindAny(%q) found nothing — it is the one resolver, and a child type is a "+
			"type", catalogChild)
	}
	if catalog.TopLevelOnly(catalogChild) != nil {
		t.Errorf("catalog.TopLevelOnly(%q) answered — the narrow lookup means the parents, which is "+
			"why the callers that mean both ask FindAny", catalogChild)
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
