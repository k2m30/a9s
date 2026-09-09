// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// views7_column_owner_test.go — one owner of the per-type column list.
//
// A list column is one fact: a title, the place its value comes from, how wide
// it renders, whether it sorts and what it sorts on. That fact is declared
// twice today — once on the catalog type (domain.Column) and once in
// core/config's per-type defaults (config.ListColumn) — and the two disagree,
// which is how lambda's Handler column reached one of them and not the other.
//
// These tests pin the one owner: the catalog column carries every field a view
// column needs, the built-in view config is derived from the catalog, and no
// per-type column literal is left in core/config for the two to drift apart
// again.
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

// views7ColumnField reads a named string field off a catalog column by
// reflection, so this file compiles — and reports the missing field as a
// failure a person can act on — before domain.Column carries it.
func views7ColumnField(col domain.Column, field string) (string, bool) {
	v := reflect.ValueOf(col).FieldByName(field)
	if !v.IsValid() || v.Kind() != reflect.String {
		return "", false
	}
	return v.String(), true
}

// TestCatalogColumnCarriesEveryViewColumnFact pins the shape the rest of this
// file depends on. A view column reads its value from a RawStruct path and
// sorts on a stored key; while the catalog column cannot say either, the
// config literal has to, and the fact stays split in two.
func TestCatalogColumnCarriesEveryViewColumnFact(t *testing.T) {
	for _, field := range []string{"Path", "SortKey"} {
		if _, ok := views7ColumnField(domain.Column{}, field); !ok {
			t.Errorf("domain.Column has no string field %s — a catalog column cannot express what a "+
				"view column needs, so the per-type list in core/config has to keep saying it", field)
		}
	}
}

// views7CatalogTypes returns every catalog type that renders a list, parents
// and children alike, keyed by the view name its columns are configured under.
func views7CatalogTypes(t *testing.T) map[string]resource.ResourceTypeDef {
	t.Helper()
	out := map[string]resource.ResourceTypeDef{}
	for _, td := range resource.AllResourceTypes() {
		out[td.ShortName] = td
	}
	for _, td := range resource.AllChildTypesForTest() {
		out[td.ShortName] = td
	}
	if len(out) == 0 {
		t.Fatal("no catalog types installed")
	}
	return out
}

// views7Describe renders a config column for a failure message.
func views7Describe(c config.ListColumn) string {
	return fmt.Sprintf("{Title:%q Path:%q Key:%q Width:%d SortKey:%q}", c.Title, c.Path, c.Key, c.Width, c.SortKey)
}

// TestDefaultConfigColumnsAreTheCatalogs pins the derivation: for every type
// that has a built-in view, the columns the config hands out are the catalog's
// columns — same set, same order, same width, same value source, same sort
// key. A difference is the two owners disagreeing, whichever one is right.
func TestDefaultConfigColumnsAreTheCatalogs(t *testing.T) {
	cfg := config.DefaultConfig()
	types := views7CatalogTypes(t)

	names := make([]string, 0, len(cfg.Views))
	for name := range cfg.Views {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		vd := cfg.Views[name]
		td, known := types[name]
		if !known {
			if len(vd.List) > 0 {
				t.Errorf("core/config declares %d list columns for view %q, which no catalog type "+
					"declares — a per-type column list with no type to own it", len(vd.List), name)
			}
			continue
		}
		if len(vd.List) != len(td.Columns) {
			t.Errorf("%s: the config declares %d list columns, the catalog %d — one column list, one owner",
				name, len(vd.List), len(td.Columns))
		}
		n := min(len(vd.List), len(td.Columns))
		for i := range n {
			got, want := vd.List[i], td.Columns[i]
			path, _ := views7ColumnField(want, "Path")
			sortKey, _ := views7ColumnField(want, "SortKey")
			from := config.ListColumn{Title: want.Title, Path: path, Key: want.Key, Width: want.Width, SortKey: sortKey}
			if got != from {
				t.Errorf("%s column %d: the config says %s, the catalog says %s",
					name, i, views7Describe(got), views7Describe(from))
			}
		}
	}

	for name, td := range types {
		if len(td.Columns) == 0 {
			continue
		}
		if _, ok := cfg.Views[name]; !ok {
			t.Errorf("the catalog declares %d columns for %q and the built-in config has no view for it — "+
				"a list nobody can configure", len(td.Columns), name)
		}
	}
}

// views7ConfigFiles returns every non-test Go file of core/config, parsed.
func views7ConfigFiles(t *testing.T) (*token.FileSet, map[string]*ast.File) {
	t.Helper()
	dir := filepath.Join(projectRoot(t), "core", "config")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read core/config: %v", err)
	}
	fset := token.NewFileSet()
	files := map[string]*ast.File{}
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		f, parseErr := parser.ParseFile(fset, filepath.Join(dir, name), nil, parser.SkipObjectResolution)
		if parseErr != nil {
			t.Fatalf("parse %s: %v", name, parseErr)
		}
		files[name] = f
	}
	if len(files) == 0 {
		t.Fatal("no Go files found in core/config")
	}
	return fset, files
}

// views7IsListColumnLiteral reports whether lit declares config.ListColumn
// values — either the column itself or the slice of them a view is given.
func views7IsListColumnLiteral(lit *ast.CompositeLit) bool {
	switch t := lit.Type.(type) {
	case *ast.Ident:
		return t.Name == "ListColumn"
	case *ast.ArrayType:
		id, ok := t.Elt.(*ast.Ident)
		return ok && id.Name == "ListColumn"
	}
	return false
}

// TestNoPerTypeColumnLiteralInConfig is the half of the change that keeps it
// done. Deriving the built-in views from the catalog while the per-type lists
// stay in core/config leaves both owners in the tree and the next column is
// added to whichever one the writer happened to open.
//
// The migration table in ensure_views.go is not a declaration of a column: it
// records, whole, what an OLDER build generated, so an installed file can be
// told apart from an edited one. It is history, and it stays.
func TestNoPerTypeColumnLiteralInConfig(t *testing.T) {
	fset, files := views7ConfigFiles(t)

	history := map[string][2]token.Pos{}
	for name, f := range files {
		for _, decl := range f.Decls {
			gd, ok := decl.(*ast.GenDecl)
			if !ok || gd.Tok != token.VAR {
				continue
			}
			for _, spec := range gd.Specs {
				vs, ok := spec.(*ast.ValueSpec)
				if !ok {
					continue
				}
				for _, id := range vs.Names {
					if id.Name == "viewColumnChanges" {
						history[name] = [2]token.Pos{gd.Pos(), gd.End()}
					}
				}
			}
		}
	}

	var offenders []string
	for name, f := range files {
		ast.Inspect(f, func(n ast.Node) bool {
			lit, ok := n.(*ast.CompositeLit)
			if !ok {
				return true
			}
			if !views7IsListColumnLiteral(lit) || len(lit.Elts) == 0 {
				return true
			}
			if r, recorded := history[name]; recorded && lit.Pos() >= r[0] && lit.End() <= r[1] {
				return true
			}
			offenders = append(offenders, fmt.Sprintf("%s: ListColumn literal", fset.Position(lit.Pos())))
			return true
		})
	}
	sort.Strings(offenders)
	for _, o := range offenders {
		t.Errorf("a list column is declared in core/config: %s — the column belongs to its catalog type, "+
			"and a second declaration here is the drift this change removes", o)
	}
}

// TestGeneratedViewsVersionIsBumpedForTheColumnMove pins the operator's half.
// Moving the column list changes what a9s generates; a view file already on
// disk is only reconciled with it when the stamp it was written at is older
// than this build's.
func TestGeneratedViewsVersionIsBumpedForTheColumnMove(t *testing.T) {
	const stampBeforeTheMove = 5
	if config.GeneratedViewsVersion <= stampBeforeTheMove {
		t.Errorf("GeneratedViewsVersion is still %d — a view file written by an earlier build keeps the "+
			"columns that build generated, so the moved column list never reaches anyone who has run a9s once",
			config.GeneratedViewsVersion)
	}
}

// TestRepoViewFilesAreWhatThisBuildGenerates pins the checked-in view files
// against the built-in defaults they are generated from (go run ./cmd/viewsgen).
// A change to the column list that stops there leaves the repo's own files
// showing the previous build's columns.
func TestRepoViewFilesAreWhatThisBuildGenerates(t *testing.T) {
	dir := filepath.Join(projectRoot(t), ".a9s", "views")
	for name := range views7CatalogTypes(t) {
		def := config.DefaultViewDef(name)
		if len(def.List) == 0 && len(def.Detail) == 0 {
			continue
		}
		path := filepath.Join(dir, name+".yaml")
		onDisk, err := os.ReadFile(path) //nolint:gosec // a test reading its own repo's generated files
		if err != nil {
			t.Errorf("%s: %v — every type with a built-in view has a generated file", name, err)
			continue
		}
		if want := config.GenerateViewYAML(def); string(onDisk) != string(want) {
			t.Errorf("%s is not what this build generates — regenerate with go run ./cmd/viewsgen/\n--- on disk\n%s\n--- generated\n%s",
				filepath.Join(".a9s", "views", name+".yaml"), onDisk, want)
		}
	}
}

// TestInstalledViewFileMigratesToTheMovedColumns pins that the move is
// RECORDED, not only made. An untouched file written by the build before it
// takes this build's column set — but only while every column in it is one
// this build still declares or one the migration table names, whole, as what
// an older build wrote. A column whose key or width changed without a line in
// that table makes the file look edited, and the operator keeps the old
// column forever.
func TestInstalledViewFileMigratesToTheMovedColumns(t *testing.T) {
	previous := views7PreChangeViewFiles(t)
	dir := t.TempDir()
	for name, body := range previous {
		if err := os.WriteFile(filepath.Join(dir, name+".yaml"), []byte(body), 0o600); err != nil {
			t.Fatalf("seed %s: %v", name, err)
		}
	}
	if err := config.EnsureViewsDir(dir); err != nil {
		t.Fatalf("EnsureViewsDir: %v", err)
	}

	names := make([]string, 0, len(previous))
	for name := range previous {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		raw, err := os.ReadFile(filepath.Join(dir, name+".yaml")) //nolint:gosec // a temp dir this test wrote
		if err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}
		vd, err := config.ParseSingle(raw)
		if err != nil {
			t.Errorf("%s: parse migrated file: %v", name, err)
			continue
		}
		if vd.Generated != config.GeneratedViewsVersion {
			t.Errorf("%s: migrated file is stamped %d, this build is %d", name, vd.Generated, config.GeneratedViewsVersion)
		}
		want := config.DefaultViewDef(name).List
		if len(vd.List) != len(want) {
			t.Errorf("%s: an untouched file from the previous build kept %d columns, this build declares %d — "+
				"record the change in ensure_views.go's migration table so the file is recognised as generated",
				name, len(vd.List), len(want))
			continue
		}
		for i := range want {
			if vd.List[i] != want[i] {
				t.Errorf("%s column %d after migration: %s, want %s — the previous build's column is not "+
					"recorded, so the file reads as one the operator edited and is left alone",
					name, i, views7Describe(vd.List[i]), views7Describe(want[i]))
			}
		}
	}
}

// views7PreChangeViewFiles reads the view files this build generated before
// the column list moved, captured from the tree at task/views7's base. They
// are what an operator who has run a9s once has on disk.
func views7PreChangeViewFiles(t *testing.T) map[string]string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(projectRoot(t), "tests", "unit", "testdata", "views7_pre_change_views.txt"))
	if err != nil {
		t.Fatalf("read the pre-change view files: %v", err)
	}
	out := map[string]string{}
	for _, block := range strings.Split(string(raw), "===== ") {
		block = strings.TrimPrefix(block, "\n")
		if strings.TrimSpace(block) == "" {
			continue
		}
		head, body, ok := strings.Cut(block, "\n")
		if !ok {
			t.Fatalf("malformed capture near %.40q", block)
		}
		out[strings.TrimSpace(head)] = body
	}
	if len(out) == 0 {
		t.Fatal("the pre-change capture is empty")
	}
	return out
}
