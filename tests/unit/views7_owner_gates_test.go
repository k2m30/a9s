// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// views7_owner_gates_test.go — what the one-owner change leaves behind.
//
// Moving the column list onto the catalog closed the drift between two
// declarations. These gates close the four ways a second truth source grows
// back: a field nobody reads, a column whose value source is a guess, a
// derivation that treats a type registered by a test differently from one
// registered by the catalog, and a command that reads the catalog before it
// is installed.
package unit

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/k2m30/a9s/v3/core/config"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// views7GoFilesUnder parses every non-test Go file under the given repo-root
// relative directories.
func views7GoFilesUnder(t *testing.T, dirs ...string) (*token.FileSet, map[string]*ast.File) {
	t.Helper()
	fset := token.NewFileSet()
	files := map[string]*ast.File{}
	for _, dir := range dirs {
		root := filepath.Join(projectRoot(t), dir)
		err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			f, parseErr := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
			if parseErr != nil {
				return parseErr
			}
			files[path] = f
			return nil
		})
		if err != nil {
			t.Fatalf("walking %s: %v", dir, err)
		}
	}
	if len(files) == 0 {
		t.Fatalf("no Go files under %v", dirs)
	}
	return fset, files
}

// TestSortableIsReadOrGone pins row 6's observable.
//
// domain.Column.Sortable is set on most catalog columns and read by nothing
// outside tests: handleActionSort (core/app/actions_list.go) sorts on whatever
// key the action carries, so a column declared unsortable sorts like any
// other. A field a declaration carries and no code consults is a second truth
// source with nobody to contradict it — the tests that assert it are
// asserting the literal back to itself.
//
// Either the field goes, or something refuses a sort on a column that says it
// does not sort. This gate says nothing about which; it fails only while the
// field exists with no production reader.
func TestSortableIsReadOrGone(t *testing.T) {
	if _, ok := reflect.TypeOf(domain.Column{}).FieldByName("Sortable"); !ok {
		return
	}

	fset, files := views7GoFilesUnder(t, "core", "internal", "cmd")
	var readers []string
	for path, f := range files {
		ast.Inspect(f, func(n ast.Node) bool {
			sel, ok := n.(*ast.SelectorExpr)
			if !ok || sel.Sel.Name != "Sortable" {
				return true
			}
			rel, _ := filepath.Rel(projectRoot(t), path)
			readers = append(readers, fmt.Sprintf("%s:%d", rel, fset.Position(sel.Pos()).Line))
			return true
		})
	}
	if len(readers) == 0 {
		t.Errorf("domain.Column.Sortable is declared on the catalog column and read by no production " +
			"code — a fact carried on every column and consulted by nothing. Delete the field with its " +
			"catalog values and the assertions that read it back, or give it the reader that makes it " +
			"true (a sort on a column that declares itself unsortable is refused)")
	}
}

// TestEveryCatalogColumnNamesItsValueSource pins row 8. A column that declares
// neither a key nor a path has its value looked up by turning its TITLE into a
// field key, which is a guess that happens to work: it holds while the title
// and the fetcher's key agree letter for letter after lowercasing and spacing,
// and breaks silently when either is reworded. The catalog owns what a cell
// reads, so every column says where the value comes from.
func TestEveryCatalogColumnNamesItsValueSource(t *testing.T) {
	var offenders []string
	for _, td := range append(resource.AllResourceTypes(), resource.AllChildTypesForTest()...) {
		for _, col := range td.Columns {
			if col.Key != "" || col.Path != "" {
				continue
			}
			offenders = append(offenders, fmt.Sprintf("%s/%q", td.ShortName, col.Title))
		}
	}
	sort.Strings(offenders)
	for _, o := range offenders {
		t.Errorf("catalog column %s declares neither a key nor a path — its cell is filled by guessing "+
			"a field key from the title, the one column shape whose value source is implicit. Name the "+
			"key the fetcher writes, or the RawStruct path the value is read from", o)
	}
}

// views7OverrideColumns is the column a type registered by a test declares:
// one of each thing a column can carry, so a derivation that drops a field is
// visible rather than merely equal.
var views7OverrideColumns = []domain.Column{
	{Key: "widget_name", Title: "Widget Name", Path: "WidgetName", Width: 24, SortKey: "widget_name_raw"},
	{Key: "widget_size", Title: "Size", Path: "SizeBytes", Width: 12, SortKey: "size_bytes_raw"},
}

// TestCascadeDerivesEveryTypeTheSameWay pins row 9. The cascade has an arm for
// a type no view declares, and it builds its columns from the catalog by hand
// rather than through the derivation the built-in views come from — so it
// drops the fields that derivation carries. No registered type reaches it now
// that every one of them derives a view from its own columns; a type
// registered after the first DefaultConfig does, because that config was built
// once.
//
// Which means the bench and production resolve a column by different rules,
// and a test can pass on a shape production never renders.
func TestCascadeDerivesEveryTypeTheSameWay(t *testing.T) {
	const overrideName = "views7_widgets"
	resource.SetChildTypeForTest(resource.ResourceTypeDef{
		ShortName: overrideName,
		Columns:   views7OverrideColumns,
	})
	t.Cleanup(func() { resource.CleanupChildTypeForTest(overrideName) })

	td := resource.GetChildType(overrideName)
	if td == nil {
		t.Fatalf("the override %q did not register", overrideName)
	}
	views7AssertCascadeCarriesTheColumns(t, overrideName, td)

	// The same assertion through the other arm, on a type the catalog
	// registers: it is the behaviour the override must reach, and a change
	// that makes both arms wrong the same way is not a fix.
	registered := resource.FindResourceType("ec2")
	if registered == nil {
		t.Fatal("no ec2 resource type registered")
	}
	views7AssertCascadeCarriesTheColumns(t, "ec2", registered)
}

// views7AssertCascadeCarriesTheColumns resolves a type's list columns the way
// every renderer does and requires the resolved column to carry what the type
// declared.
func views7AssertCascadeCarriesTheColumns(t *testing.T, name string, td *resource.ResourceTypeDef) {
	t.Helper()
	got := resource.ResolveListColumnCascade(nil, name, td)
	if len(got) != len(td.Columns) {
		t.Fatalf("%s: the cascade resolved %d columns, the type declares %d", name, len(got), len(td.Columns))
	}
	for i, want := range td.Columns {
		resolved := config.ListColumn{
			Title:   want.Title,
			Path:    want.Path,
			Key:     want.Key,
			Width:   want.Width,
			SortKey: want.SortKey,
		}
		if got[i] != resolved {
			t.Errorf("%s column %d resolves to {Title:%q Path:%q Key:%q Width:%d SortKey:%q}, "+
				"the type declares {Title:%q Path:%q Key:%q Width:%d SortKey:%q} — one derivation, "+
				"whether the type came from the catalog or from a test",
				name, i, got[i].Title, got[i].Path, got[i].Key, got[i].Width, got[i].SortKey,
				resolved.Title, resolved.Path, resolved.Key, resolved.Width, resolved.SortKey)
		}
	}
}

// views7CatalogBackedPackages are the packages that answer nothing until the
// AWS catalog is installed: they panic with the catalog's own message.
var views7CatalogBackedPackages = map[string]bool{
	"github.com/k2m30/a9s/v3/core/config":   true,
	"github.com/k2m30/a9s/v3/core/resource": true,
	"github.com/k2m30/a9s/v3/core/catalog":  true,
}

// TestEveryCommandInstallsTheCatalogFirst pins row 10. The built-in views are
// the catalog now, so config.DefaultConfig and EnsureViewsDir panic in a
// binary that has not called aws.Install. Every command that reads them
// installs it first today; nothing said so, and the next command is written
// by copying one of these files.
func TestEveryCommandInstallsTheCatalogFirst(t *testing.T) {
	fset, files := views7GoFilesUnder(t, "cmd")

	type usage struct {
		install token.Pos
		first   token.Pos
		what    string
	}
	byCommand := map[string]*usage{}

	paths := make([]string, 0, len(files))
	for path := range files {
		paths = append(paths, path)
	}
	sort.Strings(paths)

	for _, path := range paths {
		f := files[path]
		command := filepath.Base(filepath.Dir(path))
		if byCommand[command] == nil {
			byCommand[command] = &usage{}
		}
		u := byCommand[command]

		catalogBacked := map[string]bool{}
		awsAlias := ""
		for _, imp := range f.Imports {
			importPath := strings.Trim(imp.Path.Value, `"`)
			alias := filepath.Base(importPath)
			if imp.Name != nil {
				alias = imp.Name.Name
			}
			if views7CatalogBackedPackages[importPath] {
				catalogBacked[alias] = true
			}
			if importPath == "github.com/k2m30/a9s/v3/core/aws" {
				awsAlias = alias
			}
		}

		ast.Inspect(f, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			pkg, ok := sel.X.(*ast.Ident)
			if !ok {
				return true
			}
			if pkg.Name == awsAlias && sel.Sel.Name == "Install" {
				if u.install == token.NoPos || call.Pos() < u.install {
					u.install = call.Pos()
				}
				return true
			}
			if !catalogBacked[pkg.Name] {
				return true
			}
			if u.first == token.NoPos || call.Pos() < u.first {
				u.first = call.Pos()
				u.what = pkg.Name + "." + sel.Sel.Name
			}
			return true
		})
	}

	commands := make([]string, 0, len(byCommand))
	for command := range byCommand {
		commands = append(commands, command)
	}
	sort.Strings(commands)

	for _, command := range commands {
		u := byCommand[command]
		if u.first == token.NoPos {
			continue
		}
		if u.install == token.NoPos {
			t.Errorf("cmd/%s calls %s at %s and never calls aws.Install() — the built-in views and "+
				"the type registry are the catalog, and every accessor panics until it is installed",
				command, u.what, fset.Position(u.first))
			continue
		}
		if u.install > u.first {
			t.Errorf("cmd/%s calls %s at %s before aws.Install() at %s — installing the catalog after "+
				"the first read is the same as not installing it",
				command, u.what, fset.Position(u.first), fset.Position(u.install))
		}
	}
}
