package unit_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"testing"

	"github.com/k2m30/a9s/v3/core/catalog"
)

func t9bDevConsoleReads(t *testing.T) (fields, parent map[string][]string) {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(t9bRepoRoot(t), "core", "aws", "catalog_*.go"))
	if err != nil || len(files) == 0 {
		t.Fatalf("no catalog files: %v", err)
	}
	fields, parent = map[string][]string{}, map[string][]string{}
	for _, file := range files {
		f, err := parser.ParseFile(token.NewFileSet(), file, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", file, err)
		}
		ast.Inspect(f, func(n ast.Node) bool {
			lit, ok := n.(*ast.CompositeLit)
			if !ok {
				return true
			}
			var short string
			var console ast.Node
			for _, el := range lit.Elts {
				kv, ok := el.(*ast.KeyValueExpr)
				if !ok {
					continue
				}
				key, ok := kv.Key.(*ast.Ident)
				if !ok {
					continue
				}
				switch key.Name {
				case "ShortName":
					if bl, ok := kv.Value.(*ast.BasicLit); ok {
						short, _ = strconv.Unquote(bl.Value) //nolint:errcheck // a ShortName literal is a quoted string
					}
				case "ConsoleURL":
					console = kv.Value
				}
			}
			if short == "" || console == nil {
				return true
			}
			ast.Inspect(console, func(m ast.Node) bool {
				ix, ok := m.(*ast.IndexExpr)
				if !ok {
					return true
				}
				sel, ok := ix.X.(*ast.SelectorExpr)
				if !ok {
					return true
				}
				bl, ok := ix.Index.(*ast.BasicLit)
				if !ok || bl.Kind != token.STRING {
					return true
				}
				k, _ := strconv.Unquote(bl.Value) //nolint:errcheck // a STRING BasicLit is quoted
				switch sel.Sel.Name {
				case "Fields":
					fields[short] = append(fields[short], k)
				case "Parent":
					parent[short] = append(parent[short], k)
				}
				return true
			})
			return true
		})
	}
	return fields, parent
}

// A child row's console link reads only what the row holds: a Fields key its
// fetcher produces (FieldKeys), or a Parent key every drill into the type
// declares in ChildViewDef.ContextKeys, which the child fetcher carries onto
// every row as Resource.Parent.
func TestT9bDev_ChildConsoleLinkReadsOnlyDeclaredKeys(t *testing.T) {
	declared := map[string][]map[string]string{}
	for _, p := range append(catalog.All(), catalog.AllChildren()...) {
		for _, ch := range p.Children {
			declared[ch.ChildType] = append(declared[ch.ChildType], ch.ContextKeys)
		}
	}
	fieldReads, parentReads := t9bDevConsoleReads(t)
	var usesParent []string
	checked := 0
	for _, td := range catalog.AllChildren() {
		if td.ConsoleURL == nil {
			continue
		}
		produced := map[string]bool{}
		for _, k := range td.FieldKeys {
			produced[k] = true
		}
		for _, k := range fieldReads[td.ShortName] {
			checked++
			if !produced[k] {
				t.Errorf("%s: ConsoleURL reads Fields[%q], which its fetcher does not declare in FieldKeys", td.ShortName, k)
			}
		}
		for _, k := range parentReads[td.ShortName] {
			checked++
			usesParent = append(usesParent, td.ShortName+"."+k)
			if len(declared[td.ShortName]) == 0 {
				t.Errorf("%s: ConsoleURL reads Parent[%q], and no drill opens this type", td.ShortName, k)
			}
			for _, ck := range declared[td.ShortName] {
				if _, ok := ck[k]; !ok {
					t.Errorf("%s: ConsoleURL reads Parent[%q], which a drill into it does not declare in ContextKeys %v", td.ShortName, k, ck)
				}
			}
		}
	}
	if checked == 0 {
		t.Fatal("no child ConsoleURL reads a row key; the source scan found nothing")
	}
	sort.Strings(usesParent)
	if !slices.Contains(usesParent, "dbi_events.engine") {
		t.Errorf("dbi_events does not read its parent's engine from Resource.Parent; parent reads: %v", usesParent)
	}
}
